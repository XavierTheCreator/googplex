package ytdlp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Stream is a direct link to a playable video, for videos whose owners don't
// allow embedding (YouTube errors 101 and 150). YouTube now mostly serves
// video and audio as separate files, so Audio is set when they are apart.
type Stream struct {
	URL     string            `json:"url"`   // video (with audio when Audio is empty)
	Audio   string            `json:"audio"` // separate audio file, or ""
	Headers map[string]string `json:"-"`     // what yt-dlp says to send when fetching them
	// Codecs for a Media Source player, e.g. "avc1.640028" and "mp4a.40.2".
	VideoCodec string    `json:"videoCodec"`
	AudioCodec string    `json:"audioCodec"`
	Title      string    `json:"title"`
	Duration   float64   `json:"duration"`
	Expires    time.Time `json:"-"`
}

// streamFormat prefers H.264 video up to 1080p plus m4a audio as two plain
// files (every webview plays these), then any single file with both.
const streamFormat = "bv*[vcodec^=avc1][height<=1080][protocol=https]+ba[ext=m4a][protocol=https]/" +
	"best[ext=mp4][vcodec!=none][acodec!=none]/best[vcodec!=none][acodec!=none]"

func streamArgs(id string) []string {
	return []string{"-J", "-f", streamFormat, "--no-playlist", "--no-warnings", "--encoding", "utf-8",
		"https://www.youtube.com/watch?v=" + id}
}

var (
	streamMu    sync.Mutex
	streamCache = map[string]Stream{}
)

// StreamURL resolves (and caches) a direct playable link for a video. The
// first call installs yt-dlp if needed and takes a few seconds.
func StreamURL(ctx context.Context, id string) (Stream, error) {
	if !videoID.MatchString(id) {
		return Stream{}, errors.New("bad video id")
	}
	streamMu.Lock()
	s, ok := streamCache[id]
	streamMu.Unlock()
	if ok && time.Now().Before(s.Expires) {
		return s, nil
	}

	path, err := EnsureBinary()
	if err != nil {
		return Stream{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, streamArgs(id)...)
	hideWindow(cmd)
	killTree(cmd)
	var stderr tailBuffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := lastLines(stderr.String(), 2); msg != "" {
			return Stream{}, errors.New(msg)
		}
		return Stream{}, fmt.Errorf("yt-dlp: %w", err)
	}
	if s, err = parseStream(out, time.Now()); err != nil {
		return Stream{}, err
	}
	streamMu.Lock()
	streamCache[id] = s
	streamMu.Unlock()
	return s, nil
}

// Cached returns a still-valid cached stream for a video.
func Cached(id string) (Stream, bool) {
	streamMu.Lock()
	defer streamMu.Unlock()
	s, ok := streamCache[id]
	return s, ok && time.Now().Before(s.Expires)
}

func parseStream(out []byte, now time.Time) (Stream, error) {
	type format struct {
		URL         string            `json:"url"`
		VCodec      string            `json:"vcodec"`
		ACodec      string            `json:"acodec"`
		HTTPHeaders map[string]string `json:"http_headers"`
	}
	var j struct {
		format
		Title     string   `json:"title"`
		Duration  float64  `json:"duration"`
		Requested []format `json:"requested_formats"` // set when video and audio are separate
	}
	if err := json.Unmarshal(out, &j); err != nil {
		return Stream{}, fmt.Errorf("yt-dlp: %w", err)
	}
	s := Stream{URL: j.URL, Headers: j.HTTPHeaders, Title: j.Title, Duration: j.Duration, VideoCodec: j.VCodec, AudioCodec: j.ACodec}
	if len(j.Requested) == 2 {
		v, a := j.Requested[0], j.Requested[1]
		if v.VCodec == "none" {
			v, a = a, v
		}
		s.URL, s.Audio, s.Headers = v.URL, a.URL, v.HTTPHeaders
		s.VideoCodec, s.AudioCodec = v.VCodec, a.ACodec
	}
	if s.URL == "" {
		return Stream{}, errors.New("YouTube offers no playable version of this video")
	}
	s.Expires = expiry(s.URL, now)
	return s, nil
}

// expiry reads googlevideo's expire=UNIX parameter, minus a margin; links
// without one are kept for 4 hours.
func expiry(link string, now time.Time) time.Time {
	limit := now.Add(4 * time.Hour)
	u, err := url.Parse(link)
	if err != nil {
		return limit
	}
	n, err := strconv.ParseInt(u.Query().Get("expire"), 10, 64)
	if err != nil || n == 0 {
		return limit
	}
	exp := time.Unix(n, 0).Add(-30 * time.Minute)
	if exp.Before(limit) {
		return exp
	}
	return limit
}

var proxyClient = &http.Client{} // no overall timeout: it streams for as long as the video plays

// chunkSize matches yt-dlp's own downloads: googlevideo throttles single
// long requests, so a long read is fetched as a run of 10 MB ranges.
var chunkSize int64 = 10 << 20

// Proxy streams one file (link is s.URL or s.Audio) to the player. The
// player's Range is answered exactly, so seeking works, but upstream it is
// fetched in chunks so playback never waits on a throttled request.
func Proxy(w http.ResponseWriter, r *http.Request, s Stream, link string) {
	start, end, ranged := parseRange(r.Header.Get("Range"))
	first, err := fetchRange(r, s, link, start, chunkEnd(start, end))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	total := totalSize(first.Header.Get("Content-Range"))
	if first.StatusCode != http.StatusPartialContent || total < 0 {
		defer first.Body.Close() // upstream ignored ranges: pass it through as is
		copyHeaders(w, first)
		w.WriteHeader(first.StatusCode)
		io.Copy(w, first.Body)
		return
	}
	if end < 0 || end >= total {
		end = total - 1
	}
	w.Header().Set("Content-Type", first.Header.Get("Content-Type"))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	if ranged {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	resp := first
	pos := start
	for {
		n, err := io.Copy(w, resp.Body)
		resp.Body.Close()
		pos += n
		if err != nil || pos > end {
			return // done, or the player went away (it seeks by asking again)
		}
		if resp, err = fetchRange(r, s, link, pos, chunkEnd(pos, end)); err != nil || resp.StatusCode != http.StatusPartialContent {
			if resp != nil {
				resp.Body.Close()
			}
			return
		}
	}
}

func chunkEnd(start, end int64) int64 {
	e := start + chunkSize - 1
	if end >= 0 && end < e {
		return end
	}
	return e
}

func fetchRange(r *http.Request, s Stream, link string, start, end int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), "GET", link, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	return proxyClient.Do(req)
}

func copyHeaders(w http.ResponseWriter, resp *http.Response) {
	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
}

// parseRange reads "bytes=START-" or "bytes=START-END"; end is -1 when open.
func parseRange(h string) (start, end int64, ok bool) {
	spec, found := strings.CutPrefix(h, "bytes=")
	if !found || strings.Contains(spec, ",") {
		return 0, -1, false
	}
	a, b, _ := strings.Cut(spec, "-")
	start, err := strconv.ParseInt(a, 10, 64)
	if err != nil || start < 0 {
		return 0, -1, false
	}
	end = -1
	if b != "" {
		if end, err = strconv.ParseInt(b, 10, 64); err != nil || end < start {
			return 0, -1, false
		}
	}
	return start, end, true
}

// totalSize reads the size from "bytes 0-9/100", or -1.
func totalSize(cr string) int64 {
	_, size, ok := strings.Cut(cr, "/")
	if !ok {
		return -1
	}
	n, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return -1
	}
	return n
}
