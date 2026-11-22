// Package player serves a tiny YouTube player page on 127.0.0.1.
//
// YouTube embeds need an http(s) page as their parent. Inside Wails the page
// origin is wails://wails (macOS, Linux) or http://wails.localhost (Windows),
// which YouTube can reject with error 153. Serving the player from a loopback
// server gives it a normal http origin. The same server also carries the
// pop-out window's calls back to the main app (see pip.go).
package player

import (
	"crypto/subtle"
	_ "embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"regexp"
	"strconv"
)

var videoID = regexp.MustCompile(`^[\w-]{11}$`)

// ValidID reports whether s looks like a YouTube video id.
func ValidID(s string) bool { return videoID.MatchString(s) }

// The page reports ready/playing/error, and the playback time every 2 seconds
// so a video can move between the app and the pop-out window without restarting.
var page = template.Must(template.New("p").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="referrer" content="strict-origin-when-cross-origin">
<style>html,body{margin:0;height:100%;overflow:hidden;background:#000}#p{display:block;width:100%;height:100%}</style></head>
<body><div id="p"></div>
<script src="https://www.youtube.com/iframe_api"></script>
<script>
function report(msg){ parent.postMessage({source:"gp-player", msg:msg}, "*") }
function onYouTubeIframeAPIReady(){
  var player = new YT.Player("p", {
    videoId: "{{.ID}}",
    playerVars: { playsinline: 1, rel: 0, autoplay: {{.Autoplay}}, start: {{.Start}}, origin: location.origin },
    events: {
      onReady: function(){
        report("ready")
        // Commands from the app (keyboard shortcuts, timestamps in descriptions).
        window.addEventListener("message", function(e){
          var c = e.data
          if (e.source !== parent || !c || c.gp !== "cmd") return
          if (c.cmd === "toggle") player.getPlayerState() === 1 ? player.pauseVideo() : player.playVideo()
          if (c.cmd === "seekBy") player.seekTo(Math.max(0, player.getCurrentTime() + c.seconds), true)
          if (c.cmd === "seekTo") { player.seekTo(Math.max(0, c.seconds), true); player.playVideo() }
        })
        setInterval(function(){
          if (player.getCurrentTime) parent.postMessage({source:"gp-player", time: player.getCurrentTime()}, "*")
        }, 2000)
      },
      onStateChange: function(e){ if (e.data === 1) report("playing"); if (e.data === 0) report("ended") },
      onError: function(e){
        // 101/150: the owner blocks embedding; 5 and 152/153: the embed itself failed.
        // The app then plays the video through yt-dlp instead.
        if ([101, 150, 5, 152, 153].indexOf(e.data) >= 0) parent.postMessage({source:"gp-player", msg:"embed_blocked", videoId:"{{.ID}}", code:e.data}, "*")
        else report("error " + e.data)
      }
    }
  })
}
</script></body></html>`))

// streamPage plays yt-dlp links in a plain video element; see stream.html.
//
//go:embed stream.html
var streamHTML string

var streamPage = template.Must(template.New("s").Parse(streamHTML))

// StreamPageData fills the stream page.
type StreamPageData struct {
	Src, Fallback           string // the video (with audio unless AudioSrc is set)
	AudioSrc, AudioFallback string // separate audio, or ""
	// For the Media Source player: codecs and the segment index URLs.
	VideoCodec, AudioCodec string
	VideoIndex, AudioIndex string
	Start                  int
	Autoplay               bool
}

// ServeStreamPage writes the video-element player page.
func ServeStreamPage(w http.ResponseWriter, d StreamPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	streamPage.Execute(w, d)
}

// Start listens on a random loopback port and returns the base URL. extra
// adds more routes, such as the pop-out window's control endpoints.
func Start(extra map[string]http.HandlerFunc) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/player", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		id := q.Get("v")
		if !ValidID(id) {
			http.Error(w, "bad video id", http.StatusBadRequest)
			return
		}
		autoplay := 0
		if q.Get("autoplay") == "1" {
			autoplay = 1
		}
		start, _ := strconv.Atoi(q.Get("start"))
		if start < 0 {
			start = 0
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page.Execute(w, struct {
			ID              string
			Autoplay, Start int
		}{id, autoplay, start})
	})
	for path, h := range extra {
		mux.HandleFunc(path, h)
	}
	go http.Serve(ln, mux)
	return fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port), nil
}

// TokenOK compares a request's token with the expected one in constant time.
// An empty expected token never matches, so nothing is accepted before a
// pop-out has been started.
func TokenOK(got, want string) bool {
	return want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
