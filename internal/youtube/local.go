package youtube

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"googleplex/internal/db"
)

// Watch later and watch history aren't in the YouTube Data API any more, so
// Googleplex keeps its own: a Watch later list, and a History of what was
// played in the app. Recent searches live in settings.

func scanVideos(rows *sql.Rows, err error) ([]Video, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Video{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var v Video
		if json.Unmarshal([]byte(data), &v) == nil {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

func AddWatchLater(conn *sql.DB, v Video) error {
	data, _ := json.Marshal(v)
	_, err := conn.Exec(`INSERT INTO yt_watch_later (video_id, data, added_at) VALUES (?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET data = excluded.data`, v.ID, string(data), time.Now().UnixMilli())
	return err
}

func RemoveWatchLater(conn *sql.DB, videoID string) error {
	_, err := conn.Exec("DELETE FROM yt_watch_later WHERE video_id = ?", videoID)
	return err
}

// WatchLater lists saved videos, most recently added first.
func WatchLater(conn *sql.DB) ([]Video, error) {
	return scanVideos(conn.Query("SELECT data FROM yt_watch_later ORDER BY added_at DESC, rowid DESC"))
}

// AddHistory records a play. Playing the same video again moves it to the top.
func AddHistory(conn *sql.DB, v Video) error {
	data, _ := json.Marshal(v)
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM yt_history WHERE video_id = ?", v.ID); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO yt_history (video_id, data, played_at) VALUES (?, ?, ?)", v.ID, string(data), time.Now().UnixMilli()); err != nil {
		return err
	}
	// Keep the newest 500.
	if _, err := tx.Exec("DELETE FROM yt_history WHERE id NOT IN (SELECT id FROM yt_history ORDER BY played_at DESC, id DESC LIMIT 500)"); err != nil {
		return err
	}
	return tx.Commit()
}

func History(conn *sql.DB) ([]Video, error) {
	return scanVideos(conn.Query("SELECT data FROM yt_history ORDER BY played_at DESC, id DESC"))
}

func ClearHistory(conn *sql.DB) error {
	_, err := conn.Exec("DELETE FROM yt_history")
	return err
}

// RecentSearches returns the last searches, newest first (at most 12).
func RecentSearches(conn *sql.DB) []string {
	out := []string{}
	json.Unmarshal([]byte(db.GetSetting(conn, "yt_recent_searches")), &out)
	return out
}

func saveRecent(conn *sql.DB, list []string) []string {
	if len(list) > 12 {
		list = list[:12]
	}
	b, _ := json.Marshal(list)
	db.SetSetting(conn, "yt_recent_searches", string(b))
	return list
}

func AddRecentSearch(conn *sql.DB, q string) []string {
	q = strings.TrimSpace(q)
	list := []string{q}
	for _, s := range RecentSearches(conn) {
		if !strings.EqualFold(s, q) {
			list = append(list, s)
		}
	}
	return saveRecent(conn, list)
}

func RemoveRecentSearch(conn *sql.DB, q string) []string {
	list := []string{}
	for _, s := range RecentSearches(conn) {
		if s != q {
			list = append(list, s)
		}
	}
	return saveRecent(conn, list)
}

func ClearRecentSearches(conn *sql.DB) []string { return saveRecent(conn, []string{}) }
