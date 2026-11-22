package player

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTokenOK(t *testing.T) {
	cases := []struct {
		got, want string
		ok        bool
	}{
		{"abc123", "abc123", true},
		{"abc124", "abc123", false},
		{"abc12", "abc123", false},
		{"", "abc123", false},
		{"", "", false}, // no pop-out running: nothing is accepted
		{"x", "", false},
	}
	for _, c := range cases {
		if TokenOK(c.got, c.want) != c.ok {
			t.Errorf("TokenOK(%q, %q) = %v", c.got, c.want, !c.ok)
		}
	}
}

func TestPlayerPage(t *testing.T) {
	base, err := Start(map[string]http.HandlerFunc{"/extra": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("extra")) }})
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) (int, string) {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	code, body := get("/player?v=aqz-KE-bpKQ&autoplay=1&start=95")
	if code != 200 || !strings.Contains(body, `videoId: "aqz-KE-bpKQ"`) || !strings.Contains(body, "autoplay:  1 , start:  95 ,") {
		t.Errorf("player page wrong (%d):\n%s", code, body)
	}
	if code, _ := get("/player?v=bad"); code != 400 {
		t.Errorf("bad id status = %d", code)
	}
	if _, body := get("/extra"); body != "extra" {
		t.Errorf("extra route = %q", body)
	}
}

func TestStreamPage(t *testing.T) {
	rec := httptest.NewRecorder()
	ServeStreamPage(rec, StreamPageData{Src: "/stream/proxy?v=x&t=y", Fallback: "https://rr1.googlevideo.com/videoplayback?a=1&b=2", Start: 42, Autoplay: true})
	body := rec.Body.String()
	for _, want := range []string{`data-src="/stream/proxy?v=x&amp;t=y"`, `data-fallback="https://rr1.googlevideo.com/videoplayback?a=1&amp;b=2"`, `data-start="42"`, `data-autoplay="true"`, `data-audio-src=""`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestStreamPageMediaSource(t *testing.T) {
	rec := httptest.NewRecorder()
	ServeStreamPage(rec, StreamPageData{Src: "/p?part=video", AudioSrc: "/p?part=audio", VideoCodec: "avc1.640028", AudioCodec: "mp4a.40.2",
		VideoIndex: "/i?part=video", AudioIndex: "/i?part=audio"})
	body := rec.Body.String()
	for _, want := range []string{`data-vcodec="avc1.640028"`, `data-acodec="mp4a.40.2"`, `data-video-index="/i?part=video"`, `data-audio-index="/i?part=audio"`, "function msePlayer()", "function twoElements()"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
}
