package ytdlp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// GP_LIVE=1 go test -run TestLiveStream ./internal/ytdlp  (network; installs yt-dlp)
func TestLiveStream(t *testing.T) {
	if os.Getenv("GP_LIVE") == "" {
		t.Skip("set GP_LIVE=1")
	}
	s, err := StreamURL(context.Background(), "aqz-KE-bpKQ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("resolved: ok, headers from yt-dlp:", len(s.Headers), "expires in", time.Until(s.Expires).Round(60e9))
	fmt.Println("separate audio:", s.Audio != "")
	try := func(name string, headers map[string]string) {
		req, _ := http.NewRequest("GET", s.URL, nil)
		req.Header.Set("Range", "bytes=0-65535")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(name, "error", err)
			return
		}
		n, _ := io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		fmt.Println(name, resp.StatusCode, resp.Header.Get("Content-Type"), n, "bytes")
	}
	try("direct, browser-like (no yt-dlp headers):", map[string]string{"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)"})
	try("with yt-dlp headers (proxy path):", s.Headers)
}
