package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestQuoteAndListQuery(t *testing.T) {
	if got := quote(`it's a \ test`); got != `'it\'s a \\ test'` {
		t.Errorf("quote = %s", got)
	}
	cases := []struct{ view, folder, order, wantQ, wantOrder string }{
		{ViewFolder, "", "", "'root' in parents and trashed = false", "folder,name_natural"},
		{ViewFolder, "abc", "modifiedTime desc", "'abc' in parents and trashed = false", "modifiedTime desc"},
		{ViewShared, "", "", "sharedWithMe = true and trashed = false", ""},
		{ViewRecent, "", "name", "trashed = false and mimeType != 'application/vnd.google-apps.folder'", "viewedByMeTime desc"},
		{ViewStarred, "", "", "starred = true and trashed = false", ""},
		{ViewTrash, "", "", "trashed = true", ""},
	}
	for _, c := range cases {
		q, o := listQuery(c.view, c.folder, c.order)
		if q != c.wantQ || o != c.wantOrder {
			t.Errorf("listQuery(%s, %q) = %q, %q", c.view, c.folder, q, o)
		}
	}
}

func serve(t *testing.T, h http.HandlerFunc) func() {
	srv := httptest.NewServer(h)
	oldAPI, oldUp := api, upload
	api, upload = srv.URL, srv.URL+"/upload"
	return func() { api, upload = oldAPI, oldUp; srv.Close() }
}

func TestListAndSearchRequests(t *testing.T) {
	var got []reqURL
	done := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, reqURL{r.URL.Path, r.URL.Query()})
		io.WriteString(w, `{"files": [{"id": "1", "name": "a"}], "nextPageToken": "next"}`)
	})
	defer done()
	p, err := List(http.DefaultClient, ViewFolder, "f1", "", "tok")
	if err != nil || len(p.Files) != 1 || p.NextPageToken != "next" {
		t.Fatalf("page = %+v, %v", p, err)
	}
	q := got[0].q
	if q.Get("q") != "'f1' in parents and trashed = false" || q.Get("pageToken") != "tok" ||
		q.Get("supportsAllDrives") != "true" || q.Get("includeItemsFromAllDrives") != "true" || !strings.Contains(q.Get("fields"), "capabilities(canEdit") {
		t.Errorf("list query = %v", q)
	}
	Search(http.DefaultClient, `o'neil`, "")
	if s := got[1].q.Get("q"); s != `(name contains 'o\'neil' or fullText contains 'o\'neil') and trashed = false` {
		t.Errorf("search q = %s", s)
	}
}

type reqURL struct {
	path string
	q    url.Values
}

func TestTrashAndRenameBodies(t *testing.T) {
	var bodies []map[string]any
	var methods []string
	done := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		bodies, methods = append(bodies, b), append(methods, r.Method+" "+r.URL.Path)
		io.WriteString(w, `{"id": "x"}`)
	})
	defer done()
	Trash(http.DefaultClient, "x")
	Rename(http.DefaultClient, "x", "  New name ")
	CreateFolder(http.DefaultClient, "", "Stuff")
	if methods[0] != "PATCH /files/x" || bodies[0]["trashed"] != true || len(bodies[0]) != 1 {
		t.Errorf("trash = %s %v", methods[0], bodies[0])
	}
	if bodies[1]["name"] != "New name" {
		t.Errorf("rename = %v", bodies[1])
	}
	if methods[2] != "POST /files" || bodies[2]["mimeType"] != Folder || bodies[2]["parents"].([]any)[0] != "root" {
		t.Errorf("create folder = %s %v", methods[2], bodies[2])
	}
	if _, err := Rename(http.DefaultClient, "x", " "); err == nil {
		t.Error("empty name should fail")
	}
}

func TestExportTypes(t *testing.T) {
	doc := "application/vnd.google-apps.document"
	if mt, ext, err := exportType(doc, "docx"); err != nil || ext != ".docx" || !strings.Contains(mt, "wordprocessingml") {
		t.Errorf("docx = %s %s %v", mt, ext, err)
	}
	if mt, _, _ := exportType("application/vnd.google-apps.spreadsheet", "xlsx"); !strings.Contains(mt, "spreadsheetml") {
		t.Errorf("xlsx = %s", mt)
	}
	if _, _, err := exportType(doc, "xlsx"); err == nil {
		t.Error("a doc can't become xlsx")
	}
	if got := ExportFormats("application/vnd.google-apps.presentation"); strings.Join(got, ",") != "pptx,pdf" {
		t.Errorf("slides formats = %v", got)
	}
	if ExportName(File{Name: "Notes", MimeType: doc}, "pdf") != "Notes.pdf" || ExportName(File{Name: "a.pdf", MimeType: doc}, "pdf") != "a.pdf" {
		t.Error("export names")
	}
}

func TestExportTooLarge(t *testing.T) {
	done := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		io.WriteString(w, `{"error": {"message": "This file is too large to be exported.", "errors": [{"reason": "exportSizeLimitExceeded"}]}}`)
	})
	defer done()
	err := Export(context.Background(), http.DefaultClient, File{ID: "x", MimeType: "application/vnd.google-apps.document"}, "pdf", filepath.Join(t.TempDir(), "x.pdf"), func(float64) {})
	if err == nil || !strings.Contains(err.Error(), "10 MB") {
		t.Errorf("err = %v", err)
	}
}

// fakeUpload accepts a resumable upload but, like a flaky network, only keeps
// part of the first chunk and fails the second request outright.
func fakeUpload(t *testing.T, total int) (*[]string, *[]byte, func()) {
	var mu sync.Mutex
	var log []string
	var received []byte
	puts := 0
	done := serve(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == "POST" {
			if r.URL.Query().Get("uploadType") != "resumable" || r.Header.Get("X-Upload-Content-Length") != strconv.Itoa(total) {
				http.Error(w, "bad start", 400)
				return
			}
			w.Header().Set("Location", "http://"+r.Host+"/session/1")
			return
		}
		cr := r.Header.Get("Content-Range")
		log = append(log, cr)
		puts++
		body, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(cr, "bytes */") { // status query
			w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", len(received)-1))
			w.WriteHeader(308)
			return
		}
		if puts == 2 { // drop the connection-like failure
			http.Error(w, "backend error", 503)
			return
		}
		var start, end, n int
		fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &n)
		if start != len(received) {
			http.Error(w, "wrong offset", 400)
			return
		}
		keep := body
		if puts == 1 {
			keep = body[:len(body)/2] // only half of the first chunk arrived
		}
		received = append(received, keep...)
		if len(received) == total {
			w.WriteHeader(200)
			io.WriteString(w, `{"id": "up1", "name": "file.bin"}`)
			return
		}
		w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", len(received)-1))
		w.WriteHeader(308)
	})
	return &log, &received, done
}

func TestResumableUploadResumes(t *testing.T) {
	old := chunkSize
	chunkSize = 256 * 1024
	defer func() { chunkSize = old }()
	total := 600 * 1024
	data := make([]byte, total)
	for i := range data {
		data[i] = byte(i % 251)
	}
	path := filepath.Join(t.TempDir(), "file.bin")
	os.WriteFile(path, data, 0o644)

	log, received, done := fakeUpload(t, total)
	defer done()
	var last float64
	f, err := Upload(context.Background(), http.DefaultClient, path, "folder1", func(p float64) { last = p })
	if err != nil || f.ID != "up1" {
		t.Fatalf("upload = %+v, %v (log %v)", f, err, *log)
	}
	if string(*received) != string(data) || last != 100 {
		t.Errorf("received %d of %d bytes, last progress %v", len(*received), total, last)
	}
	// 1st chunk half kept (308), 2nd attempt failed (503), status query, then the rest from 128 KB on.
	if (*log)[0] != "bytes 0-262143/614400" || (*log)[2] != "bytes */614400" || (*log)[3] != "bytes 131072-393215/614400" {
		t.Errorf("content ranges = %v", *log)
	}
}

func TestReceivedUpTo(t *testing.T) {
	if receivedUpTo("bytes=0-1048575") != 1048576 || receivedUpTo("") != 0 || receivedUpTo("junk") != 0 {
		t.Error("receivedUpTo")
	}
}
