package gmail

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

type LabelColor struct {
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
}

// Label is a Gmail label: a system one (INBOX, SENT, CATEGORY_SOCIAL…) or the user's own.
type Label struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Type          string      `json:"type"` // system or user
	ThreadsTotal  int         `json:"threadsTotal"`
	ThreadsUnread int         `json:"threadsUnread"`
	Color         *LabelColor `json:"color,omitempty"`
}

// LabelNode is a user label in the tree Gmail draws from "Parent/Child" names.
type LabelNode struct {
	Label    Label       `json:"label"` // ID is "" for a parent that exists only as a prefix
	Short    string      `json:"short"` // the last part of the name
	Children []LabelNode `json:"children"`
}

// ListLabels returns every label with its counts. labels.list has no counts,
// so each label is read with labels.get, 8 at a time.
func ListLabels(client *http.Client) ([]Label, error) {
	var r struct {
		Labels []Label `json:"labels"`
	}
	if err := getJSON(client, api+"/labels", &r); err != nil {
		return nil, err
	}
	out := make([]Label, len(r.Labels))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, l := range r.Labels {
		wg.Add(1)
		go func(i int, l Label) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := getJSON(client, api+"/labels/"+url.PathEscape(l.ID), &out[i]); err != nil {
				out[i] = l // counts are nice to have; keep the label without them
			}
		}(i, l)
	}
	wg.Wait()
	return out, nil
}

// LabelTree nests user labels by the slashes in their names, sorted by name.
func LabelTree(labels []Label) []LabelNode {
	var user []Label
	for _, l := range labels {
		if l.Type == "user" {
			user = append(user, l)
		}
	}
	sort.Slice(user, func(i, j int) bool { return strings.ToLower(user[i].Name) < strings.ToLower(user[j].Name) })

	var roots []LabelNode
	var place func(nodes *[]LabelNode, parts []string, prefix string, l Label)
	place = func(nodes *[]LabelNode, parts []string, prefix string, l Label) {
		full := strings.TrimPrefix(prefix+"/"+parts[0], "/")
		i := -1
		for k := range *nodes {
			if (*nodes)[k].Short == parts[0] {
				i = k
				break
			}
		}
		if i < 0 {
			*nodes = append(*nodes, LabelNode{Short: parts[0], Label: Label{Name: full, Type: "user"}})
			i = len(*nodes) - 1
		}
		if len(parts) == 1 {
			(*nodes)[i].Label = l
			return
		}
		place(&(*nodes)[i].Children, parts[1:], full, l)
	}
	for _, l := range user {
		place(&roots, strings.Split(l.Name, "/"), "", l)
	}
	return roots
}

func CreateLabel(client *http.Client, name string) (Label, error) {
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" {
		return Label{}, errors.New("a label needs a name")
	}
	var l Label
	err := do(client, "POST", api+"/labels", map[string]any{
		"name": name, "labelListVisibility": "labelShow", "messageListVisibility": "show",
	}, &l)
	return l, err
}

func RenameLabel(client *http.Client, id, name string) (Label, error) {
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" {
		return Label{}, errors.New("a label needs a name")
	}
	var l Label
	err := do(client, "PATCH", api+"/labels/"+url.PathEscape(id), map[string]any{"name": name}, &l)
	return l, err
}

// DeleteLabel removes a user label. The mail itself stays.
func DeleteLabel(client *http.Client, id string) error {
	return do(client, "DELETE", api+"/labels/"+url.PathEscape(id), nil, nil)
}

// ---- actions ----

func labelBody(add, remove []string) map[string]any {
	b := map[string]any{}
	if len(add) > 0 {
		b["addLabelIds"] = add
	}
	if len(remove) > 0 {
		b["removeLabelIds"] = remove
	}
	return b
}

// ModifyThread adds and removes labels on every message of a thread. Archive
// is removing INBOX; star, read, important and spam are labels too.
func ModifyThread(client *http.Client, id string, add, remove []string) error {
	return do(client, "POST", api+"/threads/"+url.PathEscape(id)+"/modify", labelBody(add, remove), nil)
}

// BatchModify changes labels on many messages in one call (up to 1,000).
func BatchModify(client *http.Client, messageIDs []string, add, remove []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	for start := 0; start < len(messageIDs); start += 1000 {
		end := min(start+1000, len(messageIDs))
		body := labelBody(add, remove)
		body["ids"] = messageIDs[start:end]
		if err := do(client, "POST", api+"/messages/batchModify", body, nil); err != nil {
			return err
		}
	}
	return nil
}

// TrashThread moves a thread to the trash (Gmail empties it after 30 days).
func TrashThread(client *http.Client, id string) error {
	return do(client, "POST", api+"/threads/"+url.PathEscape(id)+"/trash", nil, nil)
}

func UntrashThread(client *http.Client, id string) error {
	return do(client, "POST", api+"/threads/"+url.PathEscape(id)+"/untrash", nil, nil)
}
