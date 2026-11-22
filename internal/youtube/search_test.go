package youtube

import (
	"path/filepath"
	"testing"
	"time"

	"googleplex/internal/db"
)

func TestMapSearch(t *testing.T) {
	body := []byte(`{
	  "nextPageToken": "CBgQAA",
	  "items": [
	    {"id": {"kind": "youtube#video", "videoId": "abcdefghijk"},
	     "snippet": {"publishedAt": "2026-11-20T10:00:00Z", "channelId": "UC1", "channelTitle": "Tom &amp; Jerry",
	       "title": "It&#39;s &quot;fine&quot;", "description": "a &lt;b&gt;",
	       "thumbnails": {"medium": {"url": "https://i.ytimg.com/vi/abcdefghijk/mqdefault.jpg"},
	                      "high": {"url": "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg"}},
	       "liveBroadcastContent": "none"}},
	    {"id": {"kind": "youtube#video", "videoId": "lmnopqrstuv"},
	     "snippet": {"title": "Live now", "thumbnails": {"medium": {"url": "m.jpg"}}, "liveBroadcastContent": "live"}},
	    {"id": {"kind": "youtube#channel", "channelId": "UC2"}, "snippet": {"title": "a channel"}}
	  ]}`)
	videos, next, err := mapSearch(body)
	if err != nil {
		t.Fatal(err)
	}
	if next != "CBgQAA" || len(videos) != 2 {
		t.Fatalf("next=%q, %d videos", next, len(videos))
	}
	v := videos[0]
	if v.Title != `It's "fine"` || v.ChannelTitle != "Tom & Jerry" || v.Description != "a <b>" {
		t.Errorf("entities not decoded: %+v", v)
	}
	if v.Thumbnail != "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg" || v.Live != "" || v.PublishedAt == "" {
		t.Errorf("first video wrong: %+v", v)
	}
	if videos[1].Live != "live" || videos[1].Thumbnail != "m.jpg" {
		t.Errorf("second video wrong: %+v", videos[1])
	}
}

func TestParseYtdlpSearch(t *testing.T) {
	body := []byte(`{"_type": "playlist", "entries": [
	  {"id": "abcdefghijk", "title": "One", "channel": "Chan", "channel_id": "UC1", "duration": 253.0,
	   "view_count": 1200, "description": null, "live_status": null,
	   "thumbnails": [{"url": "https://i.ytimg.com/a.jpg"}, {"url": "https://i.ytimg.com/b.jpg"}]},
	  {"id": "lmnopqrstuv", "title": "Two", "duration": null, "view_count": null, "live_status": "is_live", "thumbnails": []},
	  {"title": "no id"}
	]}`)
	videos, err := ParseYtdlpSearch(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 2 {
		t.Fatalf("%d videos", len(videos))
	}
	a, b := videos[0], videos[1]
	if a.Duration != 253 || a.Views != 1200 || a.Thumbnail != "https://i.ytimg.com/b.jpg" || a.ChannelTitle != "Chan" {
		t.Errorf("first wrong: %+v", a)
	}
	if b.Live != "live" || b.Duration != 0 || b.Thumbnail != "https://i.ytimg.com/vi/lmnopqrstuv/hqdefault.jpg" {
		t.Errorf("second wrong: %+v", b)
	}
	if _, err := ParseYtdlpSearch([]byte("not json")); err == nil {
		t.Error("bad json should fail")
	}
}

func TestIsQuotaError(t *testing.T) {
	quota := []byte(`{"error": {"code": 403, "message": "quota", "errors": [{"reason": "quotaExceeded"}]}}`)
	other := []byte(`{"error": {"code": 403, "message": "nope", "errors": [{"reason": "forbidden"}]}}`)
	if !isQuotaError(403, quota) || isQuotaError(403, other) || isQuotaError(400, quota) {
		t.Error("isQuotaError wrong")
	}
	if apiMessage(other) != "nope" {
		t.Errorf("apiMessage = %q", apiMessage(other))
	}
}

func TestPacificDayAndCount(t *testing.T) {
	// 07:30 UTC is still the previous day in California.
	if got := pacificDay(time.Date(2026, 11, 21, 7, 30, 0, 0, time.UTC)); got != "2026-11-20" {
		t.Errorf("pacificDay = %s", got)
	}
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if SearchesToday(conn) != 0 || CountSearch(conn) != 1 || CountSearch(conn) != 2 || SearchesToday(conn) != 2 {
		t.Error("search count wrong")
	}
}
