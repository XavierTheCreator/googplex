package auth

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// Client is the OAuth client the user creates in their own Google Cloud project.
type Client struct {
	ID        string
	Secret    string
	ProjectID string
}

// ParseClientJSON reads the file Google offers when you create a Desktop OAuth client.
func ParseClientJSON(text string) (Client, error) {
	var f struct {
		Installed *struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			ProjectID    string `json:"project_id"`
		} `json:"installed"`
		Web *json.RawMessage `json:"web"`
	}
	if err := json.Unmarshal([]byte(text), &f); err != nil {
		return Client{}, errors.New("that file is not valid JSON")
	}
	if f.Installed == nil {
		if f.Web != nil {
			return Client{}, errors.New("this is a Web application client; create a Desktop app client instead")
		}
		return Client{}, errors.New("no \"installed\" section found; download the JSON of a Desktop app client")
	}
	c := Client{ID: f.Installed.ClientID, Secret: f.Installed.ClientSecret, ProjectID: f.Installed.ProjectID}
	if c.ID == "" {
		return Client{}, errors.New("the file has no client_id")
	}
	return c, nil
}

// ClientFromDotEnv reads GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / GOOGLE_PROJECT_ID
// from a .env file. Only dev builds call this.
func ClientFromDotEnv(path string) (Client, bool) {
	vals := map[string]string{}
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if ok {
				vals[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
			}
		}
	}
	get := func(k string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return vals[k]
	}
	c := Client{ID: get("GOOGLE_CLIENT_ID"), Secret: get("GOOGLE_CLIENT_SECRET"), ProjectID: get("GOOGLE_PROJECT_ID")}
	return c, c.ID != ""
}
