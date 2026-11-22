// Package google holds the plain HTTP calls the app makes to Google APIs.
package google

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Profile is the signed-in user's basic identity.
type Profile struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func GetProfile(client *http.Client) (Profile, error) {
	var p Profile
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return p, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return p, fmt.Errorf("userinfo: %s: %s", resp.Status, b)
	}
	return p, json.NewDecoder(resp.Body).Decode(&p)
}

// Status of one Google API for the signed-in user.
const (
	StatusReady        = "ready"         // API enabled and scopes granted
	StatusAPIDisabled  = "api_disabled"  // enable it in the Cloud Console
	StatusNeedsConsent = "needs_consent" // API on, but this feature's scopes not granted yet
	StatusNeedsSignIn  = "needs_sign_in" // token missing or revoked
	StatusUnknownError = "error"
)

type probe struct {
	api string // ConsoleLinks key for the API's library page
	url string
}

// probes are the cheapest read calls for each feature (1 quota unit or free).
// A feature that needs several APIs is ready when all of them are. For Docs
// and Sheets a made-up id answers 404 once the API is on and the scope granted.
var probes = map[string][]probe{
	"gmail":    {{"gmail", "https://gmail.googleapis.com/gmail/v1/users/me/profile"}},
	"calendar": {{"calendar", "https://www.googleapis.com/calendar/v3/users/me/calendarList?maxResults=1"}},
	"youtube":  {{"youtube", "https://www.googleapis.com/youtube/v3/channels?part=id&mine=true"}},
	"drive":    {{"drive", "https://www.googleapis.com/drive/v3/files?pageSize=1&fields=files(id)"}},
	"tasks":    {{"tasks", "https://tasks.googleapis.com/tasks/v1/users/@me/lists?maxResults=1"}},
	"docs": {
		{"drive", "https://www.googleapis.com/drive/v3/files?pageSize=1&fields=files(id)"},
		{"docs", "https://docs.googleapis.com/v1/documents/googleplex-probe-not-a-real-id"},
		{"sheets", "https://sheets.googleapis.com/v4/spreadsheets/googleplex-probe-not-a-real-id"},
	},
}

// CheckAPI probes a feature's APIs and turns Google's answers into a setup
// status. api names the API that needs attention (a ConsoleLinks key).
func CheckAPI(client *http.Client, service string) (status, detail, api string) {
	list, ok := probes[service]
	if !ok {
		return StatusUnknownError, "unknown service " + service, ""
	}
	for _, p := range list {
		if st, d := checkOne(client, p.url); st != StatusReady {
			return st, d, p.api
		}
	}
	return StatusReady, "", ""
}

func checkOne(client *http.Client, url string) (status, detail string) {
	resp, err := client.Get(url)
	if err != nil {
		return StatusUnknownError, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	switch {
	case resp.StatusCode == 200 || resp.StatusCode == 404 && !strings.Contains(text, "SERVICE_DISABLED"):
		return StatusReady, ""
	case resp.StatusCode == 401:
		return StatusNeedsSignIn, "Google no longer accepts the saved sign-in."
	case strings.Contains(text, "SERVICE_DISABLED") || strings.Contains(text, "accessNotConfigured"):
		return StatusAPIDisabled, "This API is turned off for your Cloud project."
	case strings.Contains(text, "ACCESS_TOKEN_SCOPE_INSUFFICIENT") || strings.Contains(text, "insufficientPermissions"):
		return StatusNeedsConsent, "Googleplex needs your permission for this feature."
	}
	return StatusUnknownError, fmt.Sprintf("%s: %s", resp.Status, firstLine(text))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}
