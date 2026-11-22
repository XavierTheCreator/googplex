package youtube

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
	_ "time/tzdata" // Pacific time on Windows, which ships no zone database

	"googleplex/internal/db"
)

// SearchResult is one page of search results.
type SearchResult struct {
	Videos        []Video `json:"videos"`
	NextPageToken string  `json:"nextPageToken"`
	Source        string  `json:"source"` // "api" or "ytdlp"
	SearchesToday int     `json:"searchesToday"`
}

// ErrQuota means Google's daily allowance for the API is used up.
var ErrQuota = errors.New("youtube: daily quota used up")

// Search runs search.list, which has its own small daily allowance, then
// fills in lengths and views with videos.list (1 unit per 50 ids).
func Search(client *http.Client, query, pageToken string) (SearchResult, error) {
	q := url.Values{
		"part":       {"snippet"},
		"type":       {"video"},
		"maxResults": {"24"},
		"q":          {query},
		"safeSearch": {"none"},
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	resp, err := client.Get("https://www.googleapis.com/youtube/v3/search?" + q.Encode())
	if err != nil {
		return SearchResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return SearchResult{}, err
	}
	if resp.StatusCode != 200 {
		if isQuotaError(resp.StatusCode, body) {
			return SearchResult{}, ErrQuota
		}
		return SearchResult{}, fmt.Errorf("youtube search: %s: %s", resp.Status, apiMessage(body))
	}
	videos, next, err := mapSearch(body)
	if err != nil {
		return SearchResult{}, err
	}

	ids := make([]string, len(videos))
	for i, v := range videos {
		ids[i] = v.ID
	}
	if d, err := fetchDetails(client, ids); len(ids) > 0 && err == nil { // lengths are optional; results still show without them
		for i, v := range videos {
			if x, ok := d[v.ID]; ok {
				videos[i].Duration, videos[i].Views = x.duration, x.views
				if x.live != "" {
					videos[i].Live = x.live
				}
			}
		}
	}
	return SearchResult{Videos: videos, NextPageToken: next, Source: "api"}, nil
}

// mapSearch reads a search.list response.
func mapSearch(body []byte) ([]Video, string, error) {
	var r struct {
		NextPageToken string `json:"nextPageToken"`
		Items         []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				PublishedAt  string `json:"publishedAt"`
				ChannelID    string `json:"channelId"`
				ChannelTitle string `json:"channelTitle"`
				Title        string `json:"title"`
				Description  string `json:"description"`
				Thumbnails   struct {
					Medium struct{ URL string } `json:"medium"`
					High   struct{ URL string } `json:"high"`
				} `json:"thumbnails"`
				LiveBroadcastContent string `json:"liveBroadcastContent"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, "", err
	}
	videos := make([]Video, 0, len(r.Items))
	for _, it := range r.Items {
		s := it.Snippet
		if it.ID.VideoID == "" {
			continue
		}
		v := Video{
			ID: it.ID.VideoID, ChannelID: s.ChannelID, PublishedAt: s.PublishedAt,
			ChannelTitle: html.UnescapeString(s.ChannelTitle), // the search API returns HTML-escaped text
			Title:        html.UnescapeString(s.Title),
			Description:  html.UnescapeString(s.Description),
			Thumbnail:    s.Thumbnails.High.URL,
		}
		if v.Thumbnail == "" {
			v.Thumbnail = s.Thumbnails.Medium.URL
		}
		if s.LiveBroadcastContent == "live" || s.LiveBroadcastContent == "upcoming" {
			v.Live = s.LiveBroadcastContent
		}
		videos = append(videos, v)
	}
	return videos, r.NextPageToken, nil
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Errors  []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	} `json:"error"`
}

func isQuotaError(status int, body []byte) bool {
	if status != 403 {
		return false
	}
	var e apiError
	json.Unmarshal(body, &e)
	for _, x := range e.Error.Errors {
		if x.Reason == "quotaExceeded" || x.Reason == "dailyLimitExceeded" {
			return true
		}
	}
	return false
}

func apiMessage(body []byte) string {
	var e apiError
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return firstN(string(body), 200)
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// ParseYtdlpSearch reads `yt-dlp --flat-playlist -J ytsearchN:QUERY` output.
func ParseYtdlpSearch(body []byte) ([]Video, error) {
	var r struct {
		Entries []struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description *string  `json:"description"`
			Channel     string   `json:"channel"`
			ChannelID   string   `json:"channel_id"`
			Duration    *float64 `json:"duration"`
			ViewCount   *int64   `json:"view_count"`
			LiveStatus  *string  `json:"live_status"`
			Thumbnails  []struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("yt-dlp search: %w", err)
	}
	videos := make([]Video, 0, len(r.Entries))
	for _, e := range r.Entries {
		if e.ID == "" {
			continue
		}
		v := Video{ID: e.ID, Title: e.Title, ChannelTitle: e.Channel, ChannelID: e.ChannelID,
			Thumbnail: "https://i.ytimg.com/vi/" + e.ID + "/hqdefault.jpg"}
		if n := len(e.Thumbnails); n > 0 && e.Thumbnails[n-1].URL != "" {
			v.Thumbnail = e.Thumbnails[n-1].URL
		}
		if e.Description != nil {
			v.Description = *e.Description
		}
		if e.Duration != nil {
			v.Duration = int(*e.Duration)
		}
		if e.ViewCount != nil {
			v.Views = *e.ViewCount
		}
		if e.LiveStatus != nil {
			switch *e.LiveStatus {
			case "is_live":
				v.Live = "live"
			case "is_upcoming":
				v.Live = "upcoming"
			}
		}
		videos = append(videos, v)
	}
	return videos, nil
}

// pacificDay is the date Google's API quota resets on (midnight Pacific time).
func pacificDay(t time.Time) string {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.FixedZone("UTC-8", -8*3600)
	}
	return t.In(loc).Format("2006-01-02")
}

func countKey() string { return "yt_search_count:" + pacificDay(time.Now()) }

// SearchesToday is how many API searches ran today (Pacific time).
func SearchesToday(conn *sql.DB) int {
	n, _ := strconv.Atoi(db.GetSetting(conn, countKey()))
	return n
}

// CountSearch records one API search and returns today's total.
func CountSearch(conn *sql.DB) int {
	n := SearchesToday(conn) + 1
	db.SetSetting(conn, countKey(), strconv.Itoa(n))
	return n
}
