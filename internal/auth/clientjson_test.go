package auth

import "testing"

func TestParseClientJSON(t *testing.T) {
	c, err := ParseClientJSON(`{"installed":{"client_id":"id.apps.googleusercontent.com","project_id":"p-1","client_secret":"s"}}`)
	if err != nil || c.ID == "" || c.ProjectID != "p-1" || c.Secret != "s" {
		t.Fatalf("got %+v, %v", c, err)
	}
	if _, err := ParseClientJSON(`{"web":{"client_id":"x"}}`); err == nil {
		t.Fatal("web client should be rejected")
	}
	if _, err := ParseClientJSON(`nope`); err == nil {
		t.Fatal("bad JSON should be rejected")
	}
}
