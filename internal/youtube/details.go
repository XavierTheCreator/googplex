package youtube

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type detail struct {
	duration int
	live     string
	views    int64
}

// AddDetails fills in each video's length and live state. Lengths already in
// the cache are reused, so only new videos (and live or upcoming ones, which
// change) are looked up: videos.list costs 1 quota unit per 50 ids.
func AddDetails(client *http.Client, conn *sql.DB, videos []Video) error {
	known, err := cachedDetails(conn)
	if err != nil {
		return err
	}
	var missing []string
	for i, v := range videos {
		if d, ok := known[v.ID]; ok {
			videos[i].Duration, videos[i].Live = d.duration, d.live
		} else {
			missing = append(missing, v.ID)
		}
	}
	fetched := map[string]detail{}
	for len(missing) > 0 {
		n := min(50, len(missing))
		got, err := fetchDetails(client, missing[:n])
		if err != nil {
			return err
		}
		for id, d := range got {
			fetched[id] = d
		}
		missing = missing[n:]
	}
	for i, v := range videos {
		if d, ok := fetched[v.ID]; ok {
			videos[i].Duration, videos[i].Live = d.duration, d.live
		}
	}
	return nil
}

// cachedDetails returns finished videos whose length is already known.
func cachedDetails(conn *sql.DB) (map[string]detail, error) {
	rows, err := conn.Query("SELECT id, duration_seconds FROM yt_videos WHERE duration_seconds > 0 AND live = ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]detail{}
	for rows.Next() {
		var id string
		var d detail
		if err := rows.Scan(&id, &d.duration); err != nil {
			return nil, err
		}
		out[id] = d
	}
	return out, rows.Err()
}

func fetchDetails(client *http.Client, ids []string) (map[string]detail, error) {
	q := url.Values{
		"part":       {"contentDetails,snippet,statistics"},
		"id":         {strings.Join(ids, ",")},
		"maxResults": {"50"},
		"fields":     {"items(id,contentDetails/duration,snippet/liveBroadcastContent,statistics/viewCount)"},
	}
	resp, err := client.Get("https://www.googleapis.com/youtube/v3/videos?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		return nil, fmt.Errorf("youtube videos.list: %s: %s", resp.Status, b)
	}
	var r struct {
		Items []struct {
			ID             string `json:"id"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
			Snippet struct {
				LiveBroadcastContent string `json:"liveBroadcastContent"`
			} `json:"snippet"`
			Statistics struct {
				ViewCount string `json:"viewCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	out := make(map[string]detail, len(r.Items))
	for _, it := range r.Items {
		d := detail{duration: parseDuration(it.ContentDetails.Duration)}
		d.views, _ = strconv.ParseInt(it.Statistics.ViewCount, 10, 64)
		if l := it.Snippet.LiveBroadcastContent; l == "live" || l == "upcoming" {
			d.live = l
		}
		out[it.ID] = d
	}
	return out, nil
}

var isoDuration = regexp.MustCompile(`^P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

// parseDuration turns an ISO 8601 duration like PT1H2M3S or P1DT2H into
// seconds. It returns 0 for anything it can't read.
func parseDuration(s string) int {
	m := isoDuration.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	total := 0
	for i, unit := range []int{7 * 86400, 86400, 3600, 60, 1} {
		n, _ := strconv.Atoi(m[i+1])
		total += n * unit
	}
	return total
}
