// Package drive lists, searches, downloads, exports and uploads Google Drive
// files over plain HTTP, and caches folder listings in SQLite.
package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	api    = "https://www.googleapis.com/drive/v3"        // tests point these at httptest
	upload = "https://www.googleapis.com/upload/drive/v3" //
)

// Folder is Drive's folder mime type; Google's own files start with "application/vnd.google-apps.".
const Folder = "application/vnd.google-apps.folder"

type File struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	MimeType     string   `json:"mimeType"`
	Size         string   `json:"size"` // bytes as a string, as Drive sends it; empty for Google files
	ModifiedTime string   `json:"modifiedTime"`
	IconLink     string   `json:"iconLink"`
	ThumbLink    string   `json:"thumbnailLink"`
	WebViewLink  string   `json:"webViewLink"`
	Shared       bool     `json:"shared"`
	Starred      bool     `json:"starred"`
	Trashed      bool     `json:"trashed"`
	Parents      []string `json:"parents"`
	Owners       []struct {
		DisplayName string `json:"displayName"`
	} `json:"owners"`
	Capabilities struct {
		CanEdit   bool `json:"canEdit"`
		CanRename bool `json:"canRename"`
		CanTrash  bool `json:"canTrash"`
	} `json:"capabilities"`
}

const fileFields = "id,name,mimeType,size,modifiedTime,iconLink,thumbnailLink,webViewLink,shared,starred,trashed,parents,owners(displayName),capabilities(canEdit,canRename,canTrash)"

type Page struct {
	Files         []File `json:"files"`
	NextPageToken string `json:"nextPageToken"`
}

func do(client *http.Client, method, u string, body any, out any) error {
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
		return apiError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func apiError(resp *http.Response) error {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
	json.Unmarshal(b, &e)
	for _, x := range e.Error.Errors {
		if x.Reason == "exportSizeLimitExceeded" {
			return errors.New("this file is too large for Google to export (the limit is 10 MB); open it in the browser and download it from there")
		}
	}
	if e.Error.Message != "" {
		return fmt.Errorf("drive: %s", e.Error.Message)
	}
	return fmt.Errorf("drive: %s", resp.Status)
}

// quote makes a string safe inside a Drive query's single quotes.
func quote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// Views a listing can show.
const (
	ViewFolder  = "folder" // a folder's contents; "root" is My Drive
	ViewShared  = "shared"
	ViewRecent  = "recent"
	ViewStarred = "starred"
	ViewTrash   = "trash"
)

// listQuery builds the q and orderBy for a view.
func listQuery(view, folderID, orderBy string) (q, order string) {
	order = orderBy
	switch view {
	case ViewShared:
		q = "sharedWithMe = true and trashed = false"
	case ViewRecent:
		q, order = "trashed = false and mimeType != "+quote(Folder), "viewedByMeTime desc"
	case ViewStarred:
		q = "starred = true and trashed = false"
	case ViewTrash:
		q = "trashed = true"
	default:
		if folderID == "" {
			folderID = "root"
		}
		q = quote(folderID) + " in parents and trashed = false"
		if order == "" {
			order = "folder,name_natural"
		}
	}
	return q, order
}

func list(client *http.Client, q, order, pageToken string, size int) (Page, error) {
	v := url.Values{
		"q":                         {q},
		"fields":                    {"nextPageToken,files(" + fileFields + ")"},
		"pageSize":                  {fmt.Sprint(size)},
		"supportsAllDrives":         {"true"},
		"includeItemsFromAllDrives": {"true"},
	}
	if order != "" {
		v.Set("orderBy", order)
	}
	if pageToken != "" {
		v.Set("pageToken", pageToken)
	}
	var p Page
	err := do(client, "GET", api+"/files?"+v.Encode(), nil, &p)
	if p.Files == nil {
		p.Files = []File{}
	}
	return p, err
}

// List returns one page of a view. orderBy is a Drive orderBy ("name",
// "modifiedTime desc", …); empty uses the view's default.
func List(client *http.Client, view, folderID, orderBy, pageToken string) (Page, error) {
	q, order := listQuery(view, folderID, orderBy)
	return list(client, q, order, pageToken, 100)
}

// Search finds files whose name or content contains text.
func Search(client *http.Client, text, pageToken string) (Page, error) {
	t := quote(strings.TrimSpace(text))
	return list(client, "(name contains "+t+" or fullText contains "+t+") and trashed = false", "", pageToken, 100)
}

// RecentOfType lists recently viewed files of one mime type (Docs or Sheets landing pages).
func RecentOfType(client *http.Client, mimeType string) ([]File, error) {
	p, err := list(client, "mimeType = "+quote(mimeType)+" and trashed = false", "viewedByMeTime desc", "", 20)
	return p.Files, err
}

func fileURL(id string) string { return api + "/files/" + url.PathEscape(id) }

// Get reads one file's metadata, including whether the user can edit it.
func Get(client *http.Client, id string) (File, error) {
	var f File
	err := do(client, "GET", fileURL(id)+"?supportsAllDrives=true&fields="+url.QueryEscape(fileFields), nil, &f)
	return f, err
}

// ---- small edits (never sharing, never permanent delete) ----

func patch(client *http.Client, id string, body map[string]any) (File, error) {
	var f File
	err := do(client, "PATCH", fileURL(id)+"?supportsAllDrives=true&fields="+url.QueryEscape(fileFields), body, &f)
	return f, err
}

func Rename(client *http.Client, id, name string) (File, error) {
	if strings.TrimSpace(name) == "" {
		return File{}, errors.New("a name can't be empty")
	}
	return patch(client, id, map[string]any{"name": strings.TrimSpace(name)})
}

func SetStarred(client *http.Client, id string, starred bool) (File, error) {
	return patch(client, id, map[string]any{"starred": starred})
}

// Trash moves a file to Drive's trash, where it can be restored for 30 days.
func Trash(client *http.Client, id string) (File, error) {
	return patch(client, id, map[string]any{"trashed": true})
}

func CreateFolder(client *http.Client, parentID, name string) (File, error) {
	if strings.TrimSpace(name) == "" {
		return File{}, errors.New("a folder needs a name")
	}
	if parentID == "" {
		parentID = "root"
	}
	var f File
	err := do(client, "POST", api+"/files?supportsAllDrives=true&fields="+url.QueryEscape(fileFields),
		map[string]any{"name": strings.TrimSpace(name), "mimeType": Folder, "parents": []string{parentID}}, &f)
	return f, err
}

// ---- downloads ----

// Progress is sent while a transfer runs.
type Progress struct {
	ID      string  `json:"id"`   // transfer id
	Kind    string  `json:"kind"` // download, export, upload
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
	Status  string  `json:"status"` // running, done, error
	Path    string  `json:"path"`
	Error   string  `json:"error"`
}

// exportTypes maps a Google file type and chosen format to the export mime
// type and file extension.
var exportTypes = map[string]map[string][2]string{
	"application/vnd.google-apps.document": {
		"docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx"},
		"pdf":  {"application/pdf", ".pdf"},
		"txt":  {"text/plain", ".txt"},
	},
	"application/vnd.google-apps.spreadsheet": {
		"xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx"},
		"pdf":  {"application/pdf", ".pdf"},
		"csv":  {"text/csv", ".csv"},
	},
	"application/vnd.google-apps.presentation": {
		"pptx": {"application/vnd.openxmlformats-officedocument.presentationml.presentation", ".pptx"},
		"pdf":  {"application/pdf", ".pdf"},
	},
	"application/vnd.google-apps.drawing": {
		"png": {"image/png", ".png"},
		"pdf": {"application/pdf", ".pdf"},
	},
}

// ExportFormats lists the formats a Google file can be exported to.
func ExportFormats(mimeType string) []string {
	order := []string{"docx", "xlsx", "pptx", "pdf", "csv", "txt", "png"}
	var out []string
	for _, f := range order {
		if _, ok := exportTypes[mimeType][f]; ok {
			out = append(out, f)
		}
	}
	return out
}

// exportType returns the mime type and extension for exporting a file.
func exportType(mimeType, format string) (string, string, error) {
	t, ok := exportTypes[mimeType][format]
	if !ok {
		return "", "", fmt.Errorf("this kind of file can't be exported as %s", format)
	}
	return t[0], t[1], nil
}

// IsGoogleFile reports whether the file is a Google Doc, Sheet, Slides…, which
// have no bytes of their own and must be exported.
func IsGoogleFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "application/vnd.google-apps.")
}

// Download saves a regular file's bytes to path.
func Download(ctx context.Context, client *http.Client, f File, path string, onProgress func(float64)) error {
	return save(ctx, client, fileURL(f.ID)+"?alt=media&supportsAllDrives=true", path, f.Size, onProgress)
}

// Export saves a Google file converted to format (docx, xlsx, pptx, pdf, …).
func Export(ctx context.Context, client *http.Client, f File, format, path string, onProgress func(float64)) error {
	mt, _, err := exportType(f.MimeType, format)
	if err != nil {
		return err
	}
	return save(ctx, client, fileURL(f.ID)+"/export?mimeType="+url.QueryEscape(mt), path, "", onProgress)
}

// ExportName is the file name an export is saved under.
func ExportName(f File, format string) string {
	_, ext, err := exportType(f.MimeType, format)
	if err != nil || strings.HasSuffix(strings.ToLower(f.Name), ext) {
		return f.Name
	}
	return f.Name + ext
}

// Open streams a regular file's bytes (for previews).
func Open(ctx context.Context, client *http.Client, id string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL(id)+"?alt=media&supportsAllDrives=true", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, apiError(resp)
	}
	return resp, nil
}

func save(ctx context.Context, client *http.Client, u, path, size string, onProgress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return apiError(resp)
	}
	total := resp.ContentLength
	if total <= 0 {
		fmt.Sscan(size, &total)
	}
	tmp := path + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(tmp)
				return werr
			}
			done += int64(n)
			if total > 0 {
				onProgress(float64(done) * 100 / float64(total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(tmp)
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
