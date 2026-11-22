package google

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Channel is the signed-in user's own YouTube channel.
type Channel struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Handle      string `json:"handle"`
	Thumbnail   string `json:"thumbnail"`
	Subscribers string `json:"subscribers"`
	Videos      string `json:"videos"`
	Views       string `json:"views"`
}

// Subscription is one channel the user follows.
type Subscription struct {
	ChannelID string `json:"channelId"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
}

type ytThumbs struct {
	Default struct{ URL string } `json:"default"`
	Medium  struct{ URL string } `json:"medium"`
}

func (t ytThumbs) best() string {
	if t.Medium.URL != "" {
		return t.Medium.URL
	}
	return t.Default.URL
}

func getJSON(client *http.Client, u string, out any) error {
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, firstLine(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// MyChannel costs 1 quota unit. It returns an empty Channel when the account has none.
func MyChannel(client *http.Client) (Channel, error) {
	var r struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string   `json:"title"`
				CustomURL  string   `json:"customUrl"`
				Thumbnails ytThumbs `json:"thumbnails"`
			} `json:"snippet"`
			Statistics struct {
				SubscriberCount string `json:"subscriberCount"`
				VideoCount      string `json:"videoCount"`
				ViewCount       string `json:"viewCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	err := getJSON(client, "https://www.googleapis.com/youtube/v3/channels?part=snippet,statistics&mine=true", &r)
	if err != nil || len(r.Items) == 0 {
		return Channel{}, err
	}
	it := r.Items[0]
	return Channel{
		ID: it.ID, Title: it.Snippet.Title, Handle: it.Snippet.CustomURL, Thumbnail: it.Snippet.Thumbnails.best(),
		Subscribers: it.Statistics.SubscriberCount, Videos: it.Statistics.VideoCount, Views: it.Statistics.ViewCount,
	}, nil
}

// MySubscriptions pages through subscriptions.list (1 unit per 50 channels), up to max.
func MySubscriptions(client *http.Client, max int) ([]Subscription, error) {
	var subs []Subscription
	page := ""
	for len(subs) < max {
		q := url.Values{"part": {"snippet"}, "mine": {"true"}, "maxResults": {"50"}, "order": {"alphabetical"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		var r struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				Snippet struct {
					Title      string   `json:"title"`
					Thumbnails ytThumbs `json:"thumbnails"`
					ResourceID struct {
						ChannelID string `json:"channelId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
		}
		if err := getJSON(client, "https://www.googleapis.com/youtube/v3/subscriptions?"+q.Encode(), &r); err != nil {
			return subs, err
		}
		for _, it := range r.Items {
			subs = append(subs, Subscription{it.Snippet.ResourceID.ChannelID, it.Snippet.Title, it.Snippet.Thumbnails.best()})
		}
		if r.NextPageToken == "" {
			break
		}
		page = r.NextPageToken
	}
	return subs, nil
}
