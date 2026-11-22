package ytdlp

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStreamArgs(t *testing.T) {
	a := streamArgs("abcdefghijk")
	i := slices.Index(a, "-f")
	if i < 0 || a[i+1] != streamFormat || !strings.HasPrefix(streamFormat, "bv*[vcodec^=avc1]") {
		t.Errorf("format selector: %v", a)
	}
	for _, want := range []string{"-J", "--no-playlist", "--no-warnings"} {
		if !slices.Contains(a, want) {
			t.Errorf("missing %s: %v", want, a)
		}
	}
	if a[len(a)-1] != "https://www.youtube.com/watch?v=abcdefghijk" {
		t.Errorf("url last: %v", a)
	}
}

func TestExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	if got := expiry("https://rr1.googlevideo.com/videoplayback?expire=1800007200&id=x", now); !got.Equal(time.Unix(1800007200, 0).Add(-30 * time.Minute)) {
		t.Errorf("2h link: %v", got)
	}
	if got := expiry("https://rr1.googlevideo.com/videoplayback?expire=1800021600", now); !got.Equal(now.Add(4 * time.Hour)) {
		t.Errorf("6h link should cap at 4h: %v", got)
	}
	if got := expiry("https://example.com/v.mp4", now); !got.Equal(now.Add(4 * time.Hour)) {
		t.Errorf("no expire: %v", got)
	}
}

func TestParseStream(t *testing.T) {
	s, err := parseStream([]byte(`{"title": "T", "duration": 201.5, "url": "https://v.example/f?expire=1", "http_headers": {"User-Agent": "UA"}}`), time.Unix(0, 0))
	if err != nil || s.URL == "" || s.Headers["User-Agent"] != "UA" || s.Duration != 201.5 {
		t.Errorf("stream = %+v, %v", s, err)
	}
	split, err := parseStream([]byte(`{"title": "T", "requested_formats": [
		{"url": "https://v.example/video?expire=1", "vcodec": "avc1.4d401f", "acodec": "none", "http_headers": {"User-Agent": "UA"}},
		{"url": "https://v.example/audio", "vcodec": "none", "acodec": "mp4a.40.2"}]}`), time.Unix(0, 0))
	if err != nil || split.URL != "https://v.example/video?expire=1" || split.Audio != "https://v.example/audio" || split.Headers["User-Agent"] != "UA" ||
		split.VideoCodec != "avc1.4d401f" || split.AudioCodec != "mp4a.40.2" {
		t.Errorf("split stream = %+v, %v", split, err)
	}
	if _, err := parseStream([]byte(`{"title": "nothing"}`), time.Now()); err == nil {
		t.Error("no url should fail")
	}
}

func TestProxyForwardsRange(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "UA" || r.Header.Get("Range") != "bytes=10-19" {
			http.Error(w, "missing headers", 400)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 10-19/100")
		w.Header().Set("Content-Length", "10")
		w.Header().Set("Set-Cookie", "leak=1")
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, "0123456789")
	}))
	defer upstream.Close()

	req := httptest.NewRequest("GET", "/stream/proxy?v=abcdefghijk", nil)
	req.Header.Set("Range", "bytes=10-19")
	rec := httptest.NewRecorder()
	Proxy(rec, req, Stream{URL: upstream.URL, Headers: map[string]string{"User-Agent": "UA"}}, upstream.URL)

	if rec.Code != 206 || rec.Body.String() != "0123456789" {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if h.Get("Content-Range") != "bytes 10-19/100" || h.Get("Content-Type") != "video/mp4" || h.Get("Content-Length") != "10" {
		t.Errorf("headers = %v", h)
	}
	if strings.Contains(h.Get("Set-Cookie"), "leak") {
		t.Error("upstream cookies must not pass through")
	}
}

// A fake googlevideo that only serves bytes in ranges of at most 7 bytes.
func chunkyUpstream(t *testing.T, body string, requests *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.Header.Get("Range"))
		start, end, ok := parseRange(r.Header.Get("Range"))
		if !ok || end < 0 || end-start+1 > 7 {
			http.Error(w, "range too big", 403)
			return
		}
		if end >= int64(len(body)) {
			end = int64(len(body)) - 1
		}
		w.Header().Set("Content-Type", "audio/mp4")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
		w.WriteHeader(206)
		io.WriteString(w, body[start:end+1])
	}))
}

func TestProxyChunksOpenRange(t *testing.T) {
	old := chunkSize
	chunkSize = 7
	defer func() { chunkSize = old }()
	body := "abcdefghijklmnopqrstuvwxyz0123456789"
	var reqs []string
	up := chunkyUpstream(t, body, &reqs)
	defer up.Close()

	req := httptest.NewRequest("GET", "/stream/proxy", nil)
	req.Header.Set("Range", "bytes=3-")
	rec := httptest.NewRecorder()
	Proxy(rec, req, Stream{}, up.URL)
	if rec.Code != 206 || rec.Body.String() != body[3:] {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Range") != fmt.Sprintf("bytes 3-35/%d", len(body)) || rec.Header().Get("Content-Length") != "33" {
		t.Errorf("headers = %v", rec.Header())
	}
	if len(reqs) != 5 || reqs[0] != "bytes=3-9" || reqs[4] != "bytes=31-35" {
		t.Errorf("upstream ranges = %v", reqs)
	}

	// No Range from the player: the whole file as a 200, still chunked upstream.
	reqs = nil
	rec = httptest.NewRecorder()
	Proxy(rec, httptest.NewRequest("GET", "/stream/proxy", nil), Stream{}, up.URL)
	if rec.Code != 200 || rec.Body.String() != body || len(reqs) != 6 {
		t.Errorf("full read: %d %q %v", rec.Code, rec.Body.String(), reqs)
	}
}

func TestParseRange(t *testing.T) {
	cases := map[string][3]int64{"bytes=0-": {0, -1, 1}, "bytes=10-19": {10, 19, 1}, "bytes=5-2": {0, -1, 0}, "items=1-2": {0, -1, 0}, "bytes=0-1,5-6": {0, -1, 0}, "": {0, -1, 0}}
	for in, want := range cases {
		s, e, ok := parseRange(in)
		if s != want[0] || e != want[1] || ok != (want[2] == 1) {
			t.Errorf("parseRange(%q) = %d %d %v", in, s, e, ok)
		}
	}
}
