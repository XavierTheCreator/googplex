package docs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixture = `{
  "documentId": "d1", "title": "Plan",
  "lists": {
    "bul": {"listProperties": {"nestingLevels": [{"glyphSymbol": "●"}, {"glyphSymbol": "○"}]}},
    "num": {"listProperties": {"nestingLevels": [{"glyphType": "DECIMAL"}]}}
  },
  "inlineObjects": {"img1": {"inlineObjectProperties": {"embeddedObject": {"title": "Chart", "imageProperties": {"contentUri": "https://lh3.googleusercontent.com/abc"}}}}},
  "body": {"content": [
    {"sectionBreak": {}},
    {"paragraph": {"paragraphStyle": {"namedStyleType": "TITLE"}, "elements": [{"textRun": {"content": "Plan <2026>\n"}}]}},
    {"paragraph": {"paragraphStyle": {"namedStyleType": "NORMAL_TEXT"}, "elements": [
      {"textRun": {"content": "Hello ", "textStyle": {}}},
      {"textRun": {"content": "bold", "textStyle": {"bold": true}}},
      {"textRun": {"content": " and ", "textStyle": {}}},
      {"textRun": {"content": "a link", "textStyle": {"link": {"url": "https://example.com/?a=1&b=2"}, "underline": true}}},
      {"textRun": {"content": " and ", "textStyle": {}}},
      {"textRun": {"content": "bad", "textStyle": {"link": {"url": "javascript:alert(1)"}}}},
      {"textRun": {"content": "\n"}}]}},
    {"paragraph": {"bullet": {"listId": "bul"}, "elements": [{"textRun": {"content": "one\n"}}]}},
    {"paragraph": {"bullet": {"listId": "bul", "nestingLevel": 1}, "elements": [{"textRun": {"content": "one.a\n"}}]}},
    {"paragraph": {"bullet": {"listId": "bul"}, "elements": [{"textRun": {"content": "two\n"}}]}},
    {"paragraph": {"elements": [{"inlineObjectElement": {"inlineObjectId": "img1"}}, {"textRun": {"content": "\n"}}]}},
    {"paragraph": {"bullet": {"listId": "num"}, "elements": [{"textRun": {"content": "first\n", "textStyle": {"italic": true}}}]}},
    {"table": {"tableRows": [{"tableCells": [
      {"content": [{"paragraph": {"elements": [{"textRun": {"content": "A1\n"}}]}}]},
      {"content": [{"paragraph": {"elements": [{"textRun": {"content": "B1\n"}}]}}]}]}]}},
    {"paragraph": {"elements": [{"textRun": {"content": "\n"}}]}}
  ]}
}`

func TestRender(t *testing.T) {
	var d document
	if err := json.Unmarshal([]byte(fixture), &d); err != nil {
		t.Fatal(err)
	}
	got := Render(d)
	want := `<h1>Plan &lt;2026&gt;</h1>` +
		`<p>Hello <strong>bold</strong> and <a href="https://example.com/?a=1&amp;b=2">a link</a> and bad</p>` +
		`<ul><li>one<ul><li>one.a</li></ul></li><li>two</li></ul>` +
		`<p><img src="https://lh3.googleusercontent.com/abc" alt="Chart"></p>` +
		`<ol><li><em>first</em></li></ol>` +
		`<table><tr><td><p>A1</p></td><td><p>B1</p></td></tr></table>` +
		`<p><br></p>`
	if got != want {
		t.Errorf("render:\n got %s\nwant %s", got, want)
	}
	if strings.Contains(got, "javascript:") {
		t.Error("unsafe link kept")
	}
}

func TestEditRequests(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, r.Method+" "+r.URL.Path+" "+string(b))
		io.WriteString(w, `{"replies": [{"replaceAllText": {"occurrencesChanged": 3}}]}`)
	}))
	defer srv.Close()
	old := api
	api = srv.URL
	defer func() { api = old }()

	if err := Append(http.DefaultClient, "d1", "Added line"); err != nil {
		t.Fatal(err)
	}
	if bodies[0] != `POST /documents/d1:batchUpdate {"requests":[{"insertText":{"endOfSegmentLocation":{},"text":"\nAdded line"}}]}` {
		t.Errorf("append = %s", bodies[0])
	}
	n, err := ReplaceAll(http.DefaultClient, "d1", "cat", "dog", true)
	if err != nil || n != 3 {
		t.Fatalf("replace = %d, %v", n, err)
	}
	if bodies[1] != `POST /documents/d1:batchUpdate {"requests":[{"replaceAllText":{"containsText":{"matchCase":true,"text":"cat"},"replaceText":"dog"}}]}` {
		t.Errorf("replace = %s", bodies[1])
	}
	if Append(http.DefaultClient, "d1", " ") == nil {
		t.Error("empty append should fail")
	}
}
