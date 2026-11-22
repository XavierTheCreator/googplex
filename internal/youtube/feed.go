// Package youtube builds the subscriptions video feed from each channel's
// public RSS feed, which costs no API quota, and caches it in SQLite.
package youtube

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Video is one upload in the feed.
type Video struct {
	ID           string `json:"id"`
	ChannelID    string `json:"channelId"`
	ChannelTitle string `json:"channelTitle"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Thumbnail    string `json:"thumbnail"`
	PublishedAt  string `json:"publishedAt"`
	Views        int64  `json:"views"`
	Duration     int    `json:"durationSeconds"` // 0 until looked up, and for live streams
	Live         string `json:"live"`            // "", "live" or "upcoming"
}

type rssFeed struct {
	Entries []struct {
		VideoID   string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
		ChannelID string `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
		Title     string `xml:"title"`
		Published string `xml:"published"`
		Author    struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Group struct {
			Description string `xml:"http://search.yahoo.com/mrss/ description"`
			Thumbnail   struct {
				URL string `xml:"url,attr"`
			} `xml:"http://search.yahoo.com/mrss/ thumbnail"`
			Community struct {
				Statistics struct {
					Views string `xml:"views,attr"`
				} `xml:"http://search.yahoo.com/mrss/ statistics"`
			} `xml:"http://search.yahoo.com/mrss/ community"`
		} `xml:"http://search.yahoo.com/mrss/ group"`
	} `xml:"entry"`
}

var rssClient = &http.Client{Timeout: 15 * time.Second}

// ChannelVideos reads one channel's RSS feed (its latest ~15 uploads).
func ChannelVideos(channelID string) ([]Video, error) {
	resp, err := rssClient.Get("https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("rss %s: %s", channelID, resp.Status)
	}
	var f rssFeed
	if err := xml.NewDecoder(resp.Body).Decode(&f); err != nil {
		return nil, err
	}
	videos := make([]Video, 0, len(f.Entries))
	for _, e := range f.Entries {
		views, _ := strconv.ParseInt(e.Group.Community.Statistics.Views, 10, 64)
		videos = append(videos, Video{
			ID: e.VideoID, ChannelID: e.ChannelID, ChannelTitle: e.Author.Name, Title: e.Title,
			Description: e.Group.Description, Thumbnail: e.Group.Thumbnail.URL, PublishedAt: e.Published, Views: views,
		})
	}
	return videos, nil
}

// FetchFeed reads every channel's RSS feed, 8 at a time, and returns the
// newest videos first. Channels whose feed fails are skipped and counted.
func FetchFeed(channelIDs []string, max int) (videos []Video, failed int) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, id := range channelIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			vs, err := ChannelVideos(id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				return
			}
			videos = append(videos, vs...)
		}(id)
	}
	wg.Wait()
	sort.Slice(videos, func(i, j int) bool { return videos[i].PublishedAt > videos[j].PublishedAt })
	if len(videos) > max {
		videos = videos[:max]
	}
	return videos, failed
}

// SaveFeed replaces the cached feed.
func SaveFeed(conn *sql.DB, videos []Video) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM yt_videos"); err != nil {
		return err
	}
	for _, v := range videos {
		_, err := tx.Exec(`INSERT OR REPLACE INTO yt_videos (id, channel_id, channel_title, title, description, thumbnail, published_at, views, duration_seconds, live)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, v.ID, v.ChannelID, v.ChannelTitle, v.Title, v.Description, v.Thumbnail, v.PublishedAt, v.Views, v.Duration, v.Live)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CachedFeed returns the feed as last saved, newest first.
func CachedFeed(conn *sql.DB) ([]Video, error) {
	rows, err := conn.Query(`SELECT id, channel_id, channel_title, title, description, thumbnail, published_at, views, duration_seconds, live
		FROM yt_videos ORDER BY published_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	videos := []Video{}
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.ChannelTitle, &v.Title, &v.Description, &v.Thumbnail, &v.PublishedAt, &v.Views, &v.Duration, &v.Live); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}
