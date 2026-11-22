package main

// The pop-out player runs as a second copy of this executable started with
// --pip, because a Wails v2 process has exactly one window. It is a small
// frameless, always-on-top window that can go anywhere on screen. It talks
// back to the main app over the main app's loopback player server.

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"googleplex/internal/player"
)

const (
	pipW = 480
	pipH = 270
)

// Pip is bound in both processes so its bindings are generated; in the main
// app it stays inactive and Config reports that.
type Pip struct {
	ctx        context.Context
	active     bool
	videoID    string
	title      string
	start      int
	port       int
	token      string
	x, y       int
	hasPos     bool
	playerBase string
	theme      string
	stream     string // the main app's yt-dlp stream page, for videos that block embedding
}

type PipConfig struct {
	Active  bool   `json:"active"`
	VideoID string `json:"videoId"`
	Title   string `json:"title"`
	Start   int    `json:"start"`
	Stream  string `json:"stream"`
	Theme   string `json:"theme"` // system, light or dark, as the app has it
}

// PipState is what the pop-out reports to the main app.
type PipState struct {
	VideoID string  `json:"videoId"`
	Time    float64 `json:"time"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
}

func parsePipArgs(args []string) (*Pip, error) {
	fs := flag.NewFlagSet("pip", flag.ContinueOnError)
	fs.Bool("pip", true, "")
	p := &Pip{active: true, x: -1, y: -1}
	fs.StringVar(&p.videoID, "video", "", "")
	fs.StringVar(&p.title, "title", "", "")
	fs.IntVar(&p.start, "start", 0, "")
	fs.IntVar(&p.port, "port", 0, "")
	fs.StringVar(&p.token, "token", "", "")
	fs.IntVar(&p.x, "x", -1, "")
	fs.IntVar(&p.y, "y", -1, "")
	fs.StringVar(&p.stream, "stream", "", "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if !player.ValidID(p.videoID) || p.port == 0 || p.token == "" {
		return nil, fmt.Errorf("pip: missing --video, --port or --token")
	}
	p.hasPos = p.x >= 0 && p.y >= 0
	return p, nil
}

func runPip(args []string) error {
	p, err := parsePipArgs(args)
	if err != nil {
		return err
	}
	p.theme = storedTheme()
	return wails.Run(&options.App{
		Title:            p.title,
		Width:            pipW,
		Height:           pipH,
		MinWidth:         320,
		MinHeight:        180,
		Frameless:        true,
		AlwaysOnTop:      true,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 1},
		OnStartup:        p.startup,
		OnDomReady:       p.domReady,
		OnBeforeClose:    func(ctx context.Context) bool { p.report("/pip/state", 0, false); return false },
		Bind:             []interface{}{p},
		Mac:              &mac.Options{TitleBar: mac.TitleBarHidden(), Appearance: macAppearance(p.theme)},
	})
}

func (p *Pip) startup(ctx context.Context) {
	p.ctx = ctx
	p.playerBase, _ = player.Start(nil)
	hideDockIcon()
	go p.followTheme()
}

// followTheme asks the main app for its theme every few seconds, so the
// pop-out's controls change with it ("theme:changed" in this window).
func (p *Pip) followTheme() {
	for range time.Tick(3 * time.Second) {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/pip/theme", p.port), nil)
		if err != nil {
			return
		}
		req.Header.Set("X-Pip-Token", p.token)
		resp, err := pipHTTP.Do(req)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 32))
		resp.Body.Close()
		if t := validTheme(strings.TrimSpace(string(b))); resp.StatusCode == 200 && t != p.theme {
			p.theme = t
			setAppAppearance(t)
			runtime.EventsEmit(p.ctx, "theme:changed", t)
		}
	}
}

// domReady puts the window where it was last time, or bottom-right.
func (p *Pip) domReady(ctx context.Context) {
	if p.hasPos {
		runtime.WindowSetPosition(ctx, p.x, p.y)
		return
	}
	if screens, err := runtime.ScreenGetAll(ctx); err == nil {
		for _, s := range screens {
			if s.IsPrimary {
				runtime.WindowSetPosition(ctx, s.Size.Width-pipW-24, s.Size.Height-pipH-96)
				return
			}
		}
	}
}

func (p *Pip) Config() PipConfig {
	return PipConfig{Active: p.active, VideoID: p.videoID, Title: p.title, Start: p.start, Stream: p.stream, Theme: p.theme}
}

func (p *Pip) PlayerURL(id string) string { return p.playerBase + "/player?v=" + id }

// ReportTime sends the playback time and window position to the main app,
// so Bring back and the next pop-out pick up where this one is.
func (p *Pip) ReportTime(t float64) { p.report("/pip/state", t, true) }

// Dock hands the video back to the main window at time t, then quits.
func (p *Pip) Dock(t float64) {
	p.report("/pip/dock", t, true)
	runtime.Quit(p.ctx)
}

func (p *Pip) Close() { runtime.Quit(p.ctx) }

// Ended tells the main app the video finished, so autoplay can bring the next one.
func (p *Pip) Ended() { p.report("/pip/ended", 0, false) }

var pipHTTP = &http.Client{Timeout: 2 * time.Second}

func (p *Pip) report(path string, t float64, withTime bool) {
	if !p.active || p.ctx == nil {
		return
	}
	x, y := runtime.WindowGetPosition(p.ctx)
	st := map[string]any{"videoId": p.videoID, "x": x, "y": y}
	if withTime {
		st["time"] = t
	}
	body, _ := json.Marshal(st)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d%s", p.port, path), bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("X-Pip-Token", p.token)
	req.Header.Set("Content-Type", "application/json")
	if resp, err := pipHTTP.Do(req); err == nil {
		resp.Body.Close()
	}
}
