package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"googleplex/internal/player"
)

// GP_SERVE=90 go test -run TestServeStreamPage ./internal/ytdlp
// Serves the yt-dlp stream page for a public test video on 127.0.0.1 so a
// browser harness can measure playback. Network; needs yt-dlp.
func TestServeStreamPage(t *testing.T) {
	secs, _ := strconv.Atoi(os.Getenv("GP_SERVE"))
	if secs == 0 {
		t.Skip("set GP_SERVE=seconds")
	}
	id := os.Getenv("GP_VIDEO")
	if id == "" {
		id = "aqz-KE-bpKQ"
	}
	s, err := StreamURL(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		d := player.StreamPageData{Src: "/proxy?part=video", Fallback: s.URL, Autoplay: true}
		if s.Audio != "" {
			d.AudioSrc, d.AudioFallback = "/proxy?part=audio", s.Audio
			if os.Getenv("GP_NO_MSE") == "" { // GP_NO_MSE measures the two-element fallback
				d.VideoCodec, d.AudioCodec = s.VideoCodec, s.AudioCodec
				d.VideoIndex, d.AudioIndex = "/index?part=video", "/index?part=audio"
			}
		}
		player.ServeStreamPage(w, d)
	})
	part := func(r *http.Request) string {
		if r.URL.Query().Get("part") == "audio" {
			return s.Audio
		}
		return s.URL
	}
	mux.HandleFunc("/proxy", func(w http.ResponseWriter, r *http.Request) { Proxy(w, r, s, part(r)) })
	mux.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
		idx, err := IndexOf(r, s, part(r))
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		json.NewEncoder(w).Encode(idx)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// the app embeds the stream page in an iframe; do the same
		fmt.Fprint(w, `<!doctype html><body style="margin:0"><iframe id="f" src="/stream" allow="autoplay" style="width:960px;height:540px;border:0"></iframe></body>`)
	})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	fmt.Printf("SERVING http://%s/ separateAudio=%v\n", ln.Addr(), s.Audio != "")
	go http.Serve(ln, mux)
	time.Sleep(time.Duration(secs) * time.Second)
}
