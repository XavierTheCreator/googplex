package player

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// GP_EMBED="id1,id2" GP_SERVE=60 go test -run TestServeEmbedCheck ./internal/player
// Serves a page that embeds the app's player page for each id and collects
// what each reports (ready, playing, embed_blocked, error N), for a WebKit
// harness to read from window.__msgs. Network.
func TestServeEmbedCheck(t *testing.T) {
	ids := os.Getenv("GP_EMBED")
	secs, _ := strconv.Atoi(os.Getenv("GP_SERVE"))
	if ids == "" || secs == 0 {
		t.Skip("set GP_EMBED and GP_SERVE")
	}
	base, err := Start(nil)
	if err != nil {
		t.Fatal(err)
	}
	var frames strings.Builder
	for _, id := range strings.Split(ids, ",") {
		fmt.Fprintf(&frames, `<iframe src="%s/player?v=%s&autoplay=1" allow="autoplay" width="320" height="180"></iframe>`, base, id)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!doctype html><body><script>window.__msgs=[];addEventListener("message",function(e){
			if(e.data&&e.data.source==="gp-player"&&e.data.msg)__msgs.push((e.data.videoId||"")+" "+e.data.msg+(e.data.code?" "+e.data.code:""))})</script>%s</body>`, frames.String())
	})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	fmt.Printf("SERVING http://%s/\n", ln.Addr())
	go http.Serve(ln, mux)
	time.Sleep(time.Duration(secs) * time.Second)
}
