// Package ytdlp downloads YouTube videos by running the official standalone
// yt-dlp build (no Python needed), which the app fetches on first use.
package ytdlp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const releases = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// Progress is one update about a download, sent to the UI as it happens.
type Progress struct {
	VideoID string  `json:"videoId"`
	Title   string  `json:"title"`
	Percent float64 `json:"percent"`
	ETA     string  `json:"eta"`
	Status  string  `json:"status"` // starting, downloading, merging, done, error
	Path    string  `json:"path"`
	Error   string  `json:"error"`
}

// asset is the release file name for this OS and CPU.
func asset() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64", "darwin/amd64":
		return "yt-dlp_macos", nil
	case "windows/amd64", "windows/arm64":
		return "yt-dlp.exe", nil
	case "linux/amd64":
		return "yt-dlp_linux", nil
	case "linux/arm64":
		return "yt-dlp_linux_aarch64", nil
	}
	return "", fmt.Errorf("yt-dlp has no standalone build for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// BinaryPath is where the app keeps its own copy of yt-dlp.
func BinaryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name = "yt-dlp.exe"
	}
	return filepath.Join(dir, "googleplex", "bin", name), nil
}

// Installed reports whether the app's copy of yt-dlp exists.
func Installed() bool {
	p, err := BinaryPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// EnsureBinary downloads the latest standalone yt-dlp if the app has none yet,
// checking it against the release's SHA2-256SUMS file.
func EnsureBinary() (string, error) {
	path, err := BinaryPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	name, err := asset()
	if err != nil {
		return "", err
	}
	want, err := expectedSum(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	resp, err := httpClient.Get(releases + name)
	if err != nil {
		return "", fmt.Errorf("download yt-dlp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download yt-dlp: %s", resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "yt-dlp-*.part")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		tmp.Close()
		return "", fmt.Errorf("download yt-dlp: %w", err)
	}
	tmp.Close()
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return "", errors.New("download yt-dlp: checksum mismatch, refusing to use it")
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		exec.Command("xattr", "-d", "com.apple.quarantine", path).Run() // usually absent; ignore errors
	}
	return path, nil
}

// expectedSum reads the published checksum for one release file.
func expectedSum(name string) (string, error) {
	resp, err := httpClient.Get(releases + "SHA2-256SUMS")
	if err != nil {
		return "", fmt.Errorf("download yt-dlp checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download yt-dlp checksums: %s", resp.Status)
	}
	sums, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return findSum(string(sums), name)
}

func findSum(sums, name string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum published for %s", name)
}

// Version returns the installed yt-dlp version, or "" when it is not installed.
func Version() string {
	if !Installed() {
		return ""
	}
	path, _ := BinaryPath()
	cmd := exec.Command(path, "--version")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Update installs yt-dlp if needed, then asks it to update itself.
func Update() (string, error) {
	path, err := EnsureBinary()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(path, "-U")
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("yt-dlp update: %s", lastLines(string(out), 3))
	}
	return Version(), nil
}

// FindFFmpeg returns ffmpeg's path, or "" if it is not installed. Apps started
// from Finder don't get Homebrew's PATH, so its folders are checked too.
func FindFFmpeg() string {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Args builds the yt-dlp command line for one video.
func Args(id, outDir, ffmpeg string) []string {
	args := []string{
		"--newline", "--progress", "--no-playlist", "--no-mtime", "--encoding", "utf-8",
		"-P", outDir,
		"-o", "%(title).150B [%(id)s].%(ext)s",
		"--progress-template", "download:GP %(progress._percent_str)s %(progress._eta_str)s",
		"--print", "after_move:GPDONE %(filepath)s",
	}
	if ffmpeg != "" {
		args = append(args, "-f", "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/b", "--merge-output-format", "mp4", "--ffmpeg-location", ffmpeg)
	} else {
		args = append(args, "-f", "b[ext=mp4]/b") // one file with audio, usually up to 720p
	}
	return append(args, "https://www.youtube.com/watch?v="+id)
}

// Download runs yt-dlp for one video and reports progress until it finishes.
// Cancelling ctx stops it.
func Download(ctx context.Context, id, outDir string, onProgress func(Progress)) error {
	if !videoID.MatchString(id) {
		return errors.New("bad video id")
	}
	path, err := EnsureBinary()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, path, Args(id, outDir, FindFFmpeg())...)
	hideWindow(cmd)
	killTree(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr tailBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	done := ""
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		p, ok := ParseLine(sc.Text())
		if !ok {
			continue
		}
		p.VideoID = id
		if p.Status == "done" {
			done = p.Path
			continue // reported once the process has exited cleanly
		}
		onProgress(p)
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return errors.New("cancelled")
		}
		if msg := lastLines(stderr.String(), 2); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	onProgress(Progress{VideoID: id, Status: "done", Percent: 100, Path: done})
	return nil
}

// Search runs a YouTube search through yt-dlp (no API quota) and returns its
// JSON. The query is one argument, never passed through a shell.
func Search(ctx context.Context, query string, n int) ([]byte, error) {
	path, err := EnsureBinary()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--flat-playlist", "-J", "--no-warnings", "--encoding", "utf-8",
		fmt.Sprintf("ytsearch%d:%s", n, query))
	hideWindow(cmd)
	killTree(cmd)
	var stderr tailBuffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := lastLines(stderr.String(), 2); msg != "" {
			return nil, errors.New("yt-dlp search: " + msg)
		}
		return nil, fmt.Errorf("yt-dlp search: %w", err)
	}
	return out, nil
}

// ParseLine reads one stdout line from yt-dlp run with Args.
func ParseLine(line string) (Progress, bool) {
	line = strings.TrimSpace(line)
	if path, ok := strings.CutPrefix(line, "GPDONE "); ok {
		return Progress{Status: "done", Percent: 100, Path: strings.TrimSpace(path)}, true
	}
	rest, ok := strings.CutPrefix(line, "GP ")
	if !ok {
		return Progress{}, false
	}
	f := strings.Fields(rest)
	if len(f) == 0 {
		return Progress{}, false
	}
	pct, err := strconv.ParseFloat(strings.TrimSuffix(f[0], "%"), 64)
	if err != nil {
		return Progress{}, false
	}
	p := Progress{Status: "downloading", Percent: pct}
	if len(f) > 1 && f[1] != "NA" && f[1] != "Unknown" {
		p.ETA = f[1]
	}
	if pct >= 100 {
		p.Status = "merging" // this part is done; yt-dlp may now fetch audio or merge
		p.ETA = ""
	}
	return p, true
}

// tailBuffer keeps the last 8 KB written to it, for error messages.
type tailBuffer struct{ b []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > 8192 {
		t.b = t.b[len(t.b)-8192:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.b) }

func lastLines(s string, n int) string {
	var keep []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "WARNING") {
			keep = append(keep, l)
		}
	}
	if len(keep) > n {
		keep = keep[len(keep)-n:]
	}
	return strings.Join(keep, " ")
}

// Reveal shows a downloaded file in Finder, Explorer or the Linux file manager.
func Reveal(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return revealWindows(path)
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}
