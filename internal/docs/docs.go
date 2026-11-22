// Package docs reads a Google Doc and renders it as clean, read-only HTML,
// and makes small edits (append text, find and replace) with batchUpdate.
package docs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var api = "https://docs.googleapis.com/v1" // tests point this at httptest

// ---- the parts of Google's document JSON that get rendered ----

type textStyle struct {
	Bold          bool `json:"bold"`
	Italic        bool `json:"italic"`
	Underline     bool `json:"underline"`
	Strikethrough bool `json:"strikethrough"`
	Link          *struct {
		URL string `json:"url"`
	} `json:"link"`
}

type paragraph struct {
	Elements []struct {
		TextRun *struct {
			Content   string    `json:"content"`
			TextStyle textStyle `json:"textStyle"`
		} `json:"textRun"`
		InlineObjectElement *struct {
			InlineObjectID string `json:"inlineObjectId"`
		} `json:"inlineObjectElement"`
	} `json:"elements"`
	ParagraphStyle struct {
		NamedStyleType string `json:"namedStyleType"`
	} `json:"paragraphStyle"`
	Bullet *struct {
		ListID       string `json:"listId"`
		NestingLevel int    `json:"nestingLevel"`
	} `json:"bullet"`
}

type element struct {
	Paragraph *paragraph `json:"paragraph"`
	Table     *struct {
		TableRows []struct {
			TableCells []struct {
				Content []element `json:"content"`
			} `json:"tableCells"`
		} `json:"tableRows"`
	} `json:"table"`
	TableOfContents *struct {
		Content []element `json:"content"`
	} `json:"tableOfContents"`
}

type document struct {
	DocumentID string `json:"documentId"`
	Title      string `json:"title"`
	Body       struct {
		Content []element `json:"content"`
	} `json:"body"`
	Lists map[string]struct {
		ListProperties struct {
			NestingLevels []struct {
				GlyphType   string `json:"glyphType"`
				GlyphSymbol string `json:"glyphSymbol"`
			} `json:"nestingLevels"`
		} `json:"listProperties"`
	} `json:"lists"`
	InlineObjects map[string]struct {
		InlineObjectProperties struct {
			EmbeddedObject struct {
				Title           string `json:"title"`
				Description     string `json:"description"`
				ImageProperties struct {
					ContentURI string `json:"contentUri"`
				} `json:"imageProperties"`
			} `json:"embeddedObject"`
		} `json:"inlineObjectProperties"`
	} `json:"inlineObjects"`
}

// Document is a Doc ready to show.
type Document struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	HTML  string `json:"html"`
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
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
		if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
			return fmt.Errorf("docs: %s", e.Error.Message)
		}
		return fmt.Errorf("docs: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func docURL(id string) string { return api + "/documents/" + url.PathEscape(id) }

// Get reads a Doc and renders it.
func Get(client *http.Client, id string) (Document, error) {
	var d document
	if err := do(client, "GET", docURL(id), nil, &d); err != nil {
		return Document{}, err
	}
	return Document{ID: d.DocumentID, Title: d.Title, HTML: Render(d)}, nil
}

// ---- rendering ----

var headings = map[string]string{
	"TITLE": "h1", "SUBTITLE": "h2", "HEADING_1": "h1", "HEADING_2": "h2", "HEADING_3": "h3",
	"HEADING_4": "h4", "HEADING_5": "h5", "HEADING_6": "h6",
}

// Render turns the document structure into HTML. Every piece of text is
// escaped and only http(s) and mailto links are kept, so the result is safe
// to place in the page.
func Render(d document) string {
	var b strings.Builder
	renderContent(&b, d, d.Body.Content)
	return b.String()
}

func renderContent(b *strings.Builder, d document, content []element) {
	var open []string // stack of open list tags, one per nesting level
	closeTo := func(depth int) {
		for len(open) > depth {
			b.WriteString("</li></" + open[len(open)-1] + ">")
			open = open[:len(open)-1]
		}
	}
	for _, el := range content {
		p := el.Paragraph
		if p != nil && p.Bullet != nil {
			level := p.Bullet.NestingLevel + 1
			switch {
			case len(open) < level:
				for len(open) < level {
					tag := listTag(d, p.Bullet.ListID, len(open))
					b.WriteString("<" + tag + "><li>")
					open = append(open, tag)
				}
			case len(open) > level:
				closeTo(level)
				b.WriteString("</li><li>")
			default:
				b.WriteString("</li><li>")
			}
			b.WriteString(inline(d, p))
			continue
		}
		closeTo(0)
		switch {
		case p != nil:
			text := inline(d, p)
			tag := headings[p.ParagraphStyle.NamedStyleType]
			if tag == "" {
				tag = "p"
				if strings.TrimSpace(text) == "" {
					text = "<br>"
				}
			}
			b.WriteString("<" + tag + ">" + text + "</" + tag + ">")
		case el.Table != nil:
			b.WriteString("<table>")
			for _, row := range el.Table.TableRows {
				b.WriteString("<tr>")
				for _, cell := range row.TableCells {
					b.WriteString("<td>")
					renderContent(b, d, cell.Content)
					b.WriteString("</td>")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</table>")
		case el.TableOfContents != nil:
			b.WriteString(`<nav class="toc">`)
			renderContent(b, d, el.TableOfContents.Content)
			b.WriteString("</nav>")
		}
	}
	closeTo(0)
}

// listTag is ol for numbered levels, ul for bullets.
func listTag(d document, listID string, level int) string {
	levels := d.Lists[listID].ListProperties.NestingLevels
	if level < len(levels) {
		gt := levels[level].GlyphType
		if gt != "" && gt != "GLYPH_TYPE_UNSPECIFIED" && gt != "NONE" && levels[level].GlyphSymbol == "" {
			return "ol"
		}
	}
	return "ul"
}

func inline(d document, p *paragraph) string {
	var b strings.Builder
	for _, e := range p.Elements {
		if e.InlineObjectElement != nil {
			obj := d.InlineObjects[e.InlineObjectElement.InlineObjectID].InlineObjectProperties.EmbeddedObject
			if src := obj.ImageProperties.ContentURI; strings.HasPrefix(src, "https://") {
				alt := obj.Title
				if alt == "" {
					alt = obj.Description
				}
				b.WriteString(`<img src="` + html.EscapeString(src) + `" alt="` + html.EscapeString(alt) + `">`)
			}
			continue
		}
		if e.TextRun == nil {
			continue
		}
		text := strings.TrimSuffix(e.TextRun.Content, "\n")
		if text == "" {
			continue
		}
		s := strings.ReplaceAll(html.EscapeString(text), "\v", "<br>") // a soft line break inside a paragraph
		st := e.TextRun.TextStyle
		if st.Bold {
			s = "<strong>" + s + "</strong>"
		}
		if st.Italic {
			s = "<em>" + s + "</em>"
		}
		if st.Underline && st.Link == nil {
			s = "<u>" + s + "</u>"
		}
		if st.Strikethrough {
			s = "<s>" + s + "</s>"
		}
		if st.Link != nil && safeLink(st.Link.URL) {
			s = `<a href="` + html.EscapeString(st.Link.URL) + `">` + s + "</a>"
		}
		b.WriteString(s)
	}
	return b.String()
}

func safeLink(u string) bool {
	l := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "mailto:")
}

// ---- light edits ----

// Requests are the batchUpdate requests for the two edits this app makes.
func appendRequests(text string) []map[string]any {
	return []map[string]any{{"insertText": map[string]any{
		"endOfSegmentLocation": map[string]any{}, // the end of the body
		"text":                 "\n" + text,
	}}}
}

func replaceRequests(find, replace string, matchCase bool) []map[string]any {
	return []map[string]any{{"replaceAllText": map[string]any{
		"containsText": map[string]any{"text": find, "matchCase": matchCase},
		"replaceText":  replace,
	}}}
}

// Append adds a paragraph of text at the end of the document.
func Append(client *http.Client, id, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("type something to add")
	}
	return do(client, "POST", docURL(id)+":batchUpdate", map[string]any{"requests": appendRequests(text)}, nil)
}

// ReplaceAll replaces every match and returns how many were changed.
func ReplaceAll(client *http.Client, id, find, replace string, matchCase bool) (int, error) {
	if find == "" {
		return 0, errors.New("type the text to find")
	}
	var r struct {
		Replies []struct {
			ReplaceAllText struct {
				OccurrencesChanged int `json:"occurrencesChanged"`
			} `json:"replaceAllText"`
		} `json:"replies"`
	}
	err := do(client, "POST", docURL(id)+":batchUpdate", map[string]any{"requests": replaceRequests(find, replace, matchCase)}, &r)
	if err != nil || len(r.Replies) == 0 {
		return 0, err
	}
	return r.Replies[0].ReplaceAllText.OccurrencesChanged, nil
}
