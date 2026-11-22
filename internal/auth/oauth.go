package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// BaseScopes is all that first launch asks for. Each feature adds its own later.
var BaseScopes = []string{"openid", "email", "profile"}

// ServiceScopes are requested the first time a feature is opened.
var ServiceScopes = map[string][]string{
	"gmail": {
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.compose",
	},
	"calendar": {
		"https://www.googleapis.com/auth/calendar.events",
		"https://www.googleapis.com/auth/calendar.readonly",
	},
	"youtube": {
		"https://www.googleapis.com/auth/youtube.force-ssl",
	},
	// Full Drive access: drive.file would only show files this app created or
	// opened, so the user couldn't browse their Drive.
	"drive": {
		"https://www.googleapis.com/auth/drive",
	},
	"tasks": {
		"https://www.googleapis.com/auth/tasks",
	},
	// Docs and Sheets viewers also read Drive metadata (edit rights, recents).
	"docs": {
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/drive",
	},
}

// Config builds the oauth2 config for a Desktop client. The redirect URL is
// filled in per sign-in because the loopback port is random.
func Config(c Client, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ID,
		ClientSecret: c.Secret, // Google treats desktop secrets as non-confidential; PKCE does the real work
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}
}

const successPage = `<!doctype html><html><body style="font-family:system-ui;background:#0b0d10;color:#e6e8eb;display:grid;place-items:center;height:100vh;margin:0">
<div style="text-align:center"><h2 style="font-weight:600">You're signed in</h2><p style="color:#9aa1ab">You can close this tab and go back to Googleplex.</p></div></body></html>`

// Login runs the loopback + PKCE flow: listen on 127.0.0.1 at a random port,
// open the consent page in the system browser, wait for Google's redirect.
// openURL is passed in so the Wails runtime can open the system browser.
func Login(ctx context.Context, cfg *oauth2.Config, openURL func(string)) (*oauth2.Token, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0") // 127.0.0.1 only: no firewall prompt on Windows
	if err != nil {
		return nil, err
	}
	defer ln.Close()

	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	verifier := oauth2.GenerateVerifier()
	state := randomString()

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		switch {
		case q.Get("state") != state:
			http.Error(w, "State mismatch. Start sign-in again from the app.", http.StatusBadRequest)
			done <- result{err: errors.New("state mismatch")}
		case q.Get("error") != "":
			fmt.Fprint(w, "Sign-in was cancelled. You can close this tab.")
			done <- result{err: fmt.Errorf("google returned: %s", q.Get("error"))}
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, successPage)
			done <- result{code: q.Get("code")}
		}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	openURL(cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "consent"), // guarantees a refresh token
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	))

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case <-ctx.Done():
		return nil, errors.New("sign-in timed out after 5 minutes")
	case res := <-done:
		if res.err != nil {
			return nil, res.err
		}
		return cfg.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
	}
}

// Revoke tells Google to forget a token. Errors are ignored by callers on sign-out.
func Revoke(token string) error {
	resp, err := http.PostForm("https://oauth2.googleapis.com/revoke", map[string][]string{"token": {token}})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func randomString() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
