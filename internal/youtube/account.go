package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// The signed-in user's YouTube: playlists, likes, ratings, subscriptions,
// channels and comments. Reads cost 1 quota unit; writes about 50.

var ytAPI = "https://www.googleapis.com/youtube/v3" // tests point this at httptest

func call(client *http.Client, method, path string, q url.Values, body any, out any) error {
	u := ytAPI + "/" + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
		if isQuotaError(resp.StatusCode, b) {
			return ErrQuota
		}
		return fmt.Errorf("youtube: %s", apiMessage(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type thumbs struct {
	Default  struct{ URL string } `json:"default"`
	Medium   struct{ URL string } `json:"medium"`
	High     struct{ URL string } `json:"high"`
	Standard struct{ URL string } `json:"standard"`
}

func (t thumbs) best() string {
	for _, u := range []string{t.High.URL, t.Medium.URL, t.Standard.URL, t.Default.URL} {
		if u != "" {
			return u
		}
	}
	return ""
}

// ---- playlists ----

type Playlist struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	ItemCount   int    `json:"itemCount"`
	Privacy     string `json:"privacy"`
	ChannelID   string `json:"channelId"`
}

type apiPlaylist struct {
	ID      string `json:"id"`
	Snippet struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ChannelID   string `json:"channelId"`
		Thumbnails  thumbs `json:"thumbnails"`
	} `json:"snippet"`
	Status struct {
		PrivacyStatus string `json:"privacyStatus"`
	} `json:"status"`
	ContentDetails struct {
		ItemCount int `json:"itemCount"`
	} `json:"contentDetails"`
}

func (p apiPlaylist) playlist() Playlist {
	return Playlist{ID: p.ID, Title: p.Snippet.Title, Description: p.Snippet.Description, Thumbnail: p.Snippet.Thumbnails.best(),
		ItemCount: p.ContentDetails.ItemCount, Privacy: p.Status.PrivacyStatus, ChannelID: p.Snippet.ChannelID}
}

// Playlists lists the user's own playlists (channelID "") or a channel's public ones.
func Playlists(client *http.Client, channelID, pageToken string) ([]Playlist, string, error) {
	q := url.Values{"part": {"snippet,status,contentDetails"}, "maxResults": {"50"}}
	if channelID == "" {
		q.Set("mine", "true")
	} else {
		q.Set("channelId", channelID)
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var r struct {
		Items         []apiPlaylist `json:"items"`
		NextPageToken string        `json:"nextPageToken"`
	}
	if err := call(client, "GET", "playlists", q, nil, &r); err != nil {
		return nil, "", err
	}
	out := []Playlist{}
	for _, p := range r.Items {
		out = append(out, p.playlist())
	}
	return out, r.NextPageToken, nil
}

func CreatePlaylist(client *http.Client, title, privacy string) (Playlist, error) {
	if strings.TrimSpace(title) == "" {
		return Playlist{}, errors.New("a playlist needs a name")
	}
	if privacy == "" {
		privacy = "private"
	}
	var p apiPlaylist
	err := call(client, "POST", "playlists", url.Values{"part": {"snippet,status"}},
		map[string]any{"snippet": map[string]any{"title": strings.TrimSpace(title)}, "status": map[string]any{"privacyStatus": privacy}}, &p)
	return p.playlist(), err
}

// RenamePlaylist sets a playlist's title. playlists.update replaces the whole
// snippet, so the description is sent back unchanged.
func RenamePlaylist(client *http.Client, id, title, description string) (Playlist, error) {
	if strings.TrimSpace(title) == "" {
		return Playlist{}, errors.New("a playlist needs a name")
	}
	var p apiPlaylist
	err := call(client, "PUT", "playlists", url.Values{"part": {"snippet"}},
		map[string]any{"id": id, "snippet": map[string]any{"title": strings.TrimSpace(title), "description": description}}, &p)
	return p.playlist(), err
}

func DeletePlaylist(client *http.Client, id string) error {
	return call(client, "DELETE", "playlists", url.Values{"id": {id}}, nil, nil)
}

// PlaylistItem is one video in a playlist; ItemID is what remove and reorder use.
type PlaylistItem struct {
	ItemID   string `json:"itemId"`
	Position int    `json:"position"`
	Video    Video  `json:"video"`
}

// PlaylistItems lists one page of a playlist (1 unit), with lengths added (1 unit).
func PlaylistItems(client *http.Client, playlistID, pageToken string) ([]PlaylistItem, string, error) {
	q := url.Values{"part": {"snippet,contentDetails"}, "playlistId": {playlistID}, "maxResults": {"50"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var r struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title                  string `json:"title"`
				Description            string `json:"description"`
				Position               int    `json:"position"`
				Thumbnails             thumbs `json:"thumbnails"`
				VideoOwnerChannelTitle string `json:"videoOwnerChannelTitle"`
				VideoOwnerChannelID    string `json:"videoOwnerChannelId"`
				ResourceID             struct {
					VideoID string `json:"videoId"`
				} `json:"resourceId"`
			} `json:"snippet"`
			ContentDetails struct {
				VideoPublishedAt string `json:"videoPublishedAt"`
			} `json:"contentDetails"`
		} `json:"items"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := call(client, "GET", "playlistItems", q, nil, &r); err != nil {
		return nil, "", err
	}
	out := []PlaylistItem{}
	var ids []string
	for _, it := range r.Items {
		s := it.Snippet
		if s.Title == "Deleted video" || s.Title == "Private video" {
			continue
		}
		v := Video{ID: s.ResourceID.VideoID, Title: s.Title, Description: s.Description, Thumbnail: s.Thumbnails.best(),
			ChannelTitle: s.VideoOwnerChannelTitle, ChannelID: s.VideoOwnerChannelID, PublishedAt: it.ContentDetails.VideoPublishedAt}
		out = append(out, PlaylistItem{ItemID: it.ID, Position: s.Position, Video: v})
		ids = append(ids, v.ID)
	}
	if d, err := fetchDetails(client, ids); len(ids) > 0 && err == nil {
		for i := range out {
			if x, ok := d[out[i].Video.ID]; ok {
				out[i].Video.Duration, out[i].Video.Views, out[i].Video.Live = x.duration, x.views, x.live
			}
		}
	}
	return out, r.NextPageToken, nil
}

func resource(videoID string) map[string]any {
	return map[string]any{"kind": "youtube#video", "videoId": videoID}
}

// AddToPlaylist adds a video at the end of a playlist and returns the new item's id.
func AddToPlaylist(client *http.Client, playlistID, videoID string) (string, error) {
	var r struct {
		ID string `json:"id"`
	}
	err := call(client, "POST", "playlistItems", url.Values{"part": {"snippet"}},
		map[string]any{"snippet": map[string]any{"playlistId": playlistID, "resourceId": resource(videoID)}}, &r)
	return r.ID, err
}

func RemoveFromPlaylist(client *http.Client, itemID string) error {
	return call(client, "DELETE", "playlistItems", url.Values{"id": {itemID}}, nil, nil)
}

// MovePlaylistItem puts an item at a new 0-based position.
func MovePlaylistItem(client *http.Client, itemID, playlistID, videoID string, position int) error {
	return call(client, "PUT", "playlistItems", url.Values{"part": {"snippet"}}, map[string]any{
		"id": itemID, "snippet": map[string]any{"playlistId": playlistID, "resourceId": resource(videoID), "position": position},
	}, nil)
}

// Membership says which of the playlists hold a video: playlist id -> item id.
// It costs 1 unit per playlist, so it's only asked for when the Save dialog opens.
func Membership(client *http.Client, playlistIDs []string, videoID string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range playlistIDs {
		var r struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if err := call(client, "GET", "playlistItems", url.Values{"part": {"id"}, "playlistId": {id}, "videoId": {videoID}, "maxResults": {"1"}}, nil, &r); err != nil {
			return out, err
		}
		if len(r.Items) > 0 {
			out[id] = r.Items[0].ID
		}
	}
	return out, nil
}

// ---- videos ----

type VideoInfo struct {
	Video
	Likes    int64  `json:"likes"`
	Comments int64  `json:"comments"`
	Rating   string `json:"rating"` // like, dislike or none, when asked for
}

type apiVideo struct {
	ID      string `json:"id"`
	Snippet struct {
		Title                string `json:"title"`
		Description          string `json:"description"`
		ChannelID            string `json:"channelId"`
		ChannelTitle         string `json:"channelTitle"`
		PublishedAt          string `json:"publishedAt"`
		Thumbnails           thumbs `json:"thumbnails"`
		LiveBroadcastContent string `json:"liveBroadcastContent"`
	} `json:"snippet"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
	Statistics struct {
		ViewCount    string `json:"viewCount"`
		LikeCount    string `json:"likeCount"`
		CommentCount string `json:"commentCount"`
	} `json:"statistics"`
}

func (a apiVideo) info() VideoInfo {
	s := a.Snippet
	v := VideoInfo{Video: Video{ID: a.ID, Title: s.Title, Description: s.Description, ChannelID: s.ChannelID, ChannelTitle: s.ChannelTitle,
		PublishedAt: s.PublishedAt, Thumbnail: s.Thumbnails.best(), Duration: parseDuration(a.ContentDetails.Duration)}}
	if s.LiveBroadcastContent == "live" || s.LiveBroadcastContent == "upcoming" {
		v.Live = s.LiveBroadcastContent
	}
	v.Views, _ = strconv.ParseInt(a.Statistics.ViewCount, 10, 64)
	v.Likes, _ = strconv.ParseInt(a.Statistics.LikeCount, 10, 64)
	v.Comments, _ = strconv.ParseInt(a.Statistics.CommentCount, 10, 64)
	return v
}

// VideoDetails reads one video's description, counts and the user's rating (2 units).
func VideoDetails(client *http.Client, id string) (VideoInfo, error) {
	var r struct {
		Items []apiVideo `json:"items"`
	}
	if err := call(client, "GET", "videos", url.Values{"part": {"snippet,contentDetails,statistics"}, "id": {id}}, nil, &r); err != nil {
		return VideoInfo{}, err
	}
	if len(r.Items) == 0 {
		return VideoInfo{}, errors.New("this video isn't available")
	}
	v := r.Items[0].info()
	v.Rating, _ = Rating(client, id)
	return v, nil
}

// LikedVideos lists videos the user liked, newest first.
func LikedVideos(client *http.Client, pageToken string) ([]Video, string, error) {
	q := url.Values{"part": {"snippet,contentDetails,statistics"}, "myRating": {"like"}, "maxResults": {"50"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var r struct {
		Items         []apiVideo `json:"items"`
		NextPageToken string     `json:"nextPageToken"`
	}
	if err := call(client, "GET", "videos", q, nil, &r); err != nil {
		return nil, "", err
	}
	out := []Video{}
	for _, it := range r.Items {
		out = append(out, it.info().Video)
	}
	return out, r.NextPageToken, nil
}

func Rating(client *http.Client, videoID string) (string, error) {
	var r struct {
		Items []struct {
			Rating string `json:"rating"`
		} `json:"items"`
	}
	if err := call(client, "GET", "videos/getRating", url.Values{"id": {videoID}}, nil, &r); err != nil || len(r.Items) == 0 {
		return "none", err
	}
	return r.Items[0].Rating, nil
}

// Rate likes, dislikes or clears ("none") the user's rating of a video.
func Rate(client *http.Client, videoID, rating string) error {
	if rating != "like" && rating != "dislike" && rating != "none" {
		return errors.New("rating is like, dislike or none")
	}
	return call(client, "POST", "videos/rate", url.Values{"id": {videoID}, "rating": {rating}}, nil, nil)
}

// ---- channels and subscriptions ----

type ChannelInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Handle      string `json:"handle"`
	Description string `json:"description"`
	Avatar      string `json:"avatar"`
	Banner      string `json:"banner"`
	Subscribers int64  `json:"subscribers"`
	Videos      int64  `json:"videos"`
	Uploads     string `json:"uploads"` // the uploads playlist
	HiddenCount bool   `json:"hiddenSubscriberCount"`
}

// UploadsPlaylistID derives a channel's uploads playlist: UCxxxx -> UUxxxx.
func UploadsPlaylistID(channelID string) string {
	if strings.HasPrefix(channelID, "UC") {
		return "UU" + channelID[2:]
	}
	return ""
}

// Channel reads a channel's page header (1 unit).
func Channel(client *http.Client, id string) (ChannelInfo, error) {
	var r struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title       string `json:"title"`
				CustomURL   string `json:"customUrl"`
				Description string `json:"description"`
				Thumbnails  thumbs `json:"thumbnails"`
			} `json:"snippet"`
			Statistics struct {
				SubscriberCount       string `json:"subscriberCount"`
				VideoCount            string `json:"videoCount"`
				HiddenSubscriberCount bool   `json:"hiddenSubscriberCount"`
			} `json:"statistics"`
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
			BrandingSettings struct {
				Image struct {
					BannerExternalURL string `json:"bannerExternalUrl"`
				} `json:"image"`
			} `json:"brandingSettings"`
		} `json:"items"`
	}
	err := call(client, "GET", "channels", url.Values{"part": {"snippet,statistics,contentDetails,brandingSettings"}, "id": {id}}, nil, &r)
	if err != nil {
		return ChannelInfo{}, err
	}
	if len(r.Items) == 0 {
		return ChannelInfo{}, errors.New("this channel isn't available")
	}
	it := r.Items[0]
	c := ChannelInfo{ID: it.ID, Title: it.Snippet.Title, Handle: it.Snippet.CustomURL, Description: it.Snippet.Description,
		Avatar: it.Snippet.Thumbnails.best(), Uploads: it.ContentDetails.RelatedPlaylists.Uploads, HiddenCount: it.Statistics.HiddenSubscriberCount}
	if b := it.BrandingSettings.Image.BannerExternalURL; b != "" {
		c.Banner = b + "=w2120-fcrop64=1,00005a57ffffa5a8-k-c0xffffffff-no-nd-rj"
	}
	c.Subscribers, _ = strconv.ParseInt(it.Statistics.SubscriberCount, 10, 64)
	c.Videos, _ = strconv.ParseInt(it.Statistics.VideoCount, 10, 64)
	if c.Uploads == "" {
		c.Uploads = UploadsPlaylistID(it.ID)
	}
	return c, nil
}

// SubscriptionID returns the user's subscription to a channel, or "" (1 unit).
func SubscriptionID(client *http.Client, channelID string) (string, error) {
	var r struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	err := call(client, "GET", "subscriptions", url.Values{"part": {"id"}, "mine": {"true"}, "forChannelId": {channelID}}, nil, &r)
	if err != nil || len(r.Items) == 0 {
		return "", err
	}
	return r.Items[0].ID, nil
}

// Subscribe subscribes to a channel and returns the subscription id.
func Subscribe(client *http.Client, channelID string) (string, error) {
	var r struct {
		ID string `json:"id"`
	}
	err := call(client, "POST", "subscriptions", url.Values{"part": {"snippet"}},
		map[string]any{"snippet": map[string]any{"resourceId": map[string]any{"kind": "youtube#channel", "channelId": channelID}}}, &r)
	return r.ID, err
}

func Unsubscribe(client *http.Client, subscriptionID string) error {
	return call(client, "DELETE", "subscriptions", url.Values{"id": {subscriptionID}}, nil, nil)
}

// ---- comments ----

type Comment struct {
	ID              string `json:"id"`
	Author          string `json:"author"`
	AuthorChannelID string `json:"authorChannelId"`
	AuthorAvatar    string `json:"authorAvatar"`
	Text            string `json:"text"` // plain text, never HTML
	Likes           int64  `json:"likes"`
	PublishedAt     string `json:"publishedAt"`
	Edited          bool   `json:"edited"`
}

type CommentThread struct {
	ID         string    `json:"id"`
	Top        Comment   `json:"top"`
	ReplyCount int       `json:"replyCount"`
	Replies    []Comment `json:"replies"` // the few YouTube includes; Replies() gets the rest
}

type apiComment struct {
	ID      string `json:"id"`
	Snippet struct {
		AuthorDisplayName     string `json:"authorDisplayName"`
		AuthorProfileImageURL string `json:"authorProfileImageUrl"`
		AuthorChannelID       struct {
			Value string `json:"value"`
		} `json:"authorChannelId"`
		TextOriginal string `json:"textOriginal"`
		TextDisplay  string `json:"textDisplay"`
		LikeCount    int64  `json:"likeCount"`
		PublishedAt  string `json:"publishedAt"`
		UpdatedAt    string `json:"updatedAt"`
	} `json:"snippet"`
}

func (c apiComment) comment() Comment {
	s := c.Snippet
	text := s.TextOriginal
	if text == "" {
		text = html.UnescapeString(s.TextDisplay) // asked for as plain text, but YouTube may still escape it
	}
	return Comment{ID: c.ID, Author: s.AuthorDisplayName, AuthorChannelID: s.AuthorChannelID.Value, AuthorAvatar: s.AuthorProfileImageURL,
		Text: text, Likes: s.LikeCount, PublishedAt: s.PublishedAt, Edited: s.UpdatedAt != "" && s.UpdatedAt != s.PublishedAt}
}

// CommentThreads lists top-level comments, by "relevance" or "time".
func CommentThreads(client *http.Client, videoID, order, pageToken string) ([]CommentThread, string, error) {
	if order != "time" {
		order = "relevance"
	}
	q := url.Values{"part": {"snippet,replies"}, "videoId": {videoID}, "order": {order}, "maxResults": {"20"}, "textFormat": {"plainText"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var r struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				TopLevelComment apiComment `json:"topLevelComment"`
				TotalReplyCount int        `json:"totalReplyCount"`
			} `json:"snippet"`
			Replies struct {
				Comments []apiComment `json:"comments"`
			} `json:"replies"`
		} `json:"items"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := call(client, "GET", "commentThreads", q, nil, &r); err != nil {
		if strings.Contains(err.Error(), "disabled comments") {
			return nil, "", errors.New("comments are turned off for this video")
		}
		return nil, "", err
	}
	out := []CommentThread{}
	for _, it := range r.Items {
		t := CommentThread{ID: it.ID, Top: it.Snippet.TopLevelComment.comment(), ReplyCount: it.Snippet.TotalReplyCount}
		for _, c := range it.Replies.Comments {
			t.Replies = append(t.Replies, c.comment())
		}
		out = append(out, t)
	}
	return out, r.NextPageToken, nil
}

// Replies lists the replies to a comment, oldest first.
func Replies(client *http.Client, parentID, pageToken string) ([]Comment, string, error) {
	q := url.Values{"part": {"snippet"}, "parentId": {parentID}, "maxResults": {"50"}, "textFormat": {"plainText"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var r struct {
		Items         []apiComment `json:"items"`
		NextPageToken string       `json:"nextPageToken"`
	}
	if err := call(client, "GET", "comments", q, nil, &r); err != nil {
		return nil, "", err
	}
	out := []Comment{}
	for _, c := range r.Items {
		out = append(out, c.comment())
	}
	return out, r.NextPageToken, nil
}

// PostComment adds a top-level comment to a video.
func PostComment(client *http.Client, videoID, text string) (CommentThread, error) {
	if strings.TrimSpace(text) == "" {
		return CommentThread{}, errors.New("write something first")
	}
	var r struct {
		ID      string `json:"id"`
		Snippet struct {
			TopLevelComment apiComment `json:"topLevelComment"`
		} `json:"snippet"`
	}
	err := call(client, "POST", "commentThreads", url.Values{"part": {"snippet"}}, map[string]any{"snippet": map[string]any{
		"videoId": videoID, "topLevelComment": map[string]any{"snippet": map[string]any{"textOriginal": text}},
	}}, &r)
	return CommentThread{ID: r.ID, Top: r.Snippet.TopLevelComment.comment()}, err
}

// Reply answers a comment.
func Reply(client *http.Client, parentID, text string) (Comment, error) {
	if strings.TrimSpace(text) == "" {
		return Comment{}, errors.New("write something first")
	}
	var c apiComment
	err := call(client, "POST", "comments", url.Values{"part": {"snippet"}},
		map[string]any{"snippet": map[string]any{"parentId": parentID, "textOriginal": text}}, &c)
	return c.comment(), err
}

// DeleteComment deletes one of the user's own comments.
func DeleteComment(client *http.Client, id string) error {
	return call(client, "DELETE", "comments", url.Values{"id": {id}}, nil, nil)
}
