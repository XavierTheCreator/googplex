package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// chunkSize must be a multiple of 256 KB for Drive's resumable uploads.
var chunkSize int64 = 8 << 20

// Upload sends a local file into a folder with a resumable upload, 8 MB at a
// time. If a chunk fails on the network it asks Drive how much arrived and
// carries on from there, a few times before giving up.
func Upload(ctx context.Context, client *http.Client, path, parentID string, onProgress func(float64)) (File, error) {
	f, err := os.Open(path)
	if err != nil {
		return File{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return File{}, err
	}
	if info.IsDir() {
		return File{}, errors.New("folders can't be uploaded yet; upload the files inside it")
	}
	total := info.Size()
	if parentID == "" {
		parentID = "root"
	}
	ctype := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if ctype == "" {
		ctype = "application/octet-stream"
	}

	session, err := startUpload(ctx, client, filepath.Base(path), parentID, ctype, total)
	if err != nil {
		return File{}, err
	}

	var offset int64
	retries := 0
	for {
		end := min(offset+chunkSize, total) - 1
		done, next, file, err := sendChunk(ctx, client, session, f, offset, end, total)
		if err != nil {
			if ctx.Err() != nil {
				return File{}, errors.New("cancelled")
			}
			if retries++; retries > 4 {
				return File{}, err
			}
			time.Sleep(time.Duration(retries) * time.Second)
			if next, err = uploadStatus(ctx, client, session, total); err != nil {
				continue
			}
		} else {
			retries = 0
		}
		if done {
			onProgress(100)
			return file, nil
		}
		offset = next
		if total > 0 {
			onProgress(float64(offset) * 100 / float64(total))
		}
	}
}

func startUpload(ctx context.Context, client *http.Client, name, parentID, ctype string, total int64) (string, error) {
	meta, _ := json.Marshal(map[string]any{"name": name, "parents": []string{parentID}})
	u := upload + "/files?uploadType=resumable&supportsAllDrives=true&fields=" + url.QueryEscape(fileFields)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(meta))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Upload-Content-Type", ctype)
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(total, 10))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", apiError(resp)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", errors.New("drive: no upload session")
	}
	return loc, nil
}

// sendChunk uploads bytes [start, end]. Drive answers 308 with the range it
// has so far, or 200/201 with the file when everything has arrived.
func sendChunk(ctx context.Context, client *http.Client, session string, f *os.File, start, end, total int64) (done bool, next int64, file File, err error) {
	var body io.Reader = http.NoBody
	length := int64(0)
	if total > 0 {
		length = end - start + 1
		body = io.NewSectionReader(f, start, length)
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", session, body)
	if err != nil {
		return false, start, File{}, err
	}
	req.ContentLength = length
	if total > 0 {
		req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	} else {
		req.Header.Set("Content-Range", "bytes */0")
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, start, File{}, err
	}
	defer resp.Body.Close()
	return chunkResult(resp, start)
}

// uploadStatus asks how many bytes Drive has, after a failed chunk.
func uploadStatus(ctx context.Context, client *http.Client, session string, total int64) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "PUT", session, http.NoBody)
	if err != nil {
		return 0, err
	}
	req.ContentLength = 0
	req.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", total))
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, next, _, err := chunkResult(resp, 0)
	return next, err
}

func chunkResult(resp *http.Response, start int64) (done bool, next int64, file File, err error) {
	switch resp.StatusCode {
	case 200, 201:
		err = json.NewDecoder(resp.Body).Decode(&file)
		return true, 0, file, err
	case 308:
		return false, receivedUpTo(resp.Header.Get("Range")), File{}, nil
	}
	return false, start, File{}, apiError(resp)
}

// receivedUpTo reads "bytes=0-1048575" into the next offset (1048576); no
// header means nothing has arrived yet.
func receivedUpTo(h string) int64 {
	_, last, ok := strings.Cut(strings.TrimPrefix(h, "bytes="), "-")
	if !ok {
		return 0
	}
	n, err := strconv.ParseInt(last, 10, 64)
	if err != nil {
		return 0
	}
	return n + 1
}
