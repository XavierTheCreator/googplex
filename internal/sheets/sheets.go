// Package sheets reads a spreadsheet's tabs and cell values a window at a
// time, and makes small edits (one cell, append a row) over plain HTTP.
package sheets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var api = "https://sheets.googleapis.com/v4" // tests point this at httptest

type Sheet struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Rows   int    `json:"rows"`
	Cols   int    `json:"cols"`
	Frozen int    `json:"frozenRows"`
}

type Spreadsheet struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Sheets []Sheet `json:"sheets"`
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
			return fmt.Errorf("sheets: %s", e.Error.Message)
		}
		return fmt.Errorf("sheets: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func sheetURL(id string) string { return api + "/spreadsheets/" + url.PathEscape(id) }

// Get reads the spreadsheet's title and tabs (no cell data).
func Get(client *http.Client, id string) (Spreadsheet, error) {
	var r struct {
		SpreadsheetID string `json:"spreadsheetId"`
		Properties    struct {
			Title string `json:"title"`
		} `json:"properties"`
		Sheets []struct {
			Properties struct {
				SheetID        int    `json:"sheetId"`
				Title          string `json:"title"`
				GridProperties struct {
					RowCount       int `json:"rowCount"`
					ColumnCount    int `json:"columnCount"`
					FrozenRowCount int `json:"frozenRowCount"`
				} `json:"gridProperties"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	fields := "spreadsheetId,properties.title,sheets.properties(sheetId,title,gridProperties)"
	if err := do(client, "GET", sheetURL(id)+"?fields="+url.QueryEscape(fields), nil, &r); err != nil {
		return Spreadsheet{}, err
	}
	s := Spreadsheet{ID: r.SpreadsheetID, Title: r.Properties.Title}
	for _, sh := range r.Sheets {
		p := sh.Properties
		s.Sheets = append(s.Sheets, Sheet{ID: p.SheetID, Title: p.Title, Rows: p.GridProperties.RowCount,
			Cols: p.GridProperties.ColumnCount, Frozen: p.GridProperties.FrozenRowCount})
	}
	return s, nil
}

// Column turns a 0-based column index into its letters: 0 -> A, 25 -> Z, 26 -> AA.
func Column(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// quoteSheet quotes a tab name for A1 notation: 'Q1 ”24'!A1.
func quoteSheet(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// Range builds A1 notation for rows [row, row+count) and columns [0, cols),
// 0-based: Range("Sheet1", 0, 200, 26) is 'Sheet1'!A1:Z200.
func Range(sheet string, row, count, cols int) string {
	return fmt.Sprintf("%s!A%d:%s%d", quoteSheet(sheet), row+1, Column(max(cols, 1)-1), row+count)
}

// Cell builds A1 notation for one 0-based cell.
func Cell(sheet string, row, col int) string {
	return fmt.Sprintf("%s!%s%d", quoteSheet(sheet), Column(col), row+1)
}

// Values reads a window of formatted cell values (what the sheet shows).
func Values(client *http.Client, id, sheet string, row, count, cols int) ([][]string, error) {
	return values(client, id, Range(sheet, row, count, cols), "FORMATTED_VALUE")
}

// Formula reads one cell as typed (its formula, or its plain value).
func Formula(client *http.Client, id, sheet string, row, col int) (string, error) {
	v, err := values(client, id, Cell(sheet, row, col), "FORMULA")
	if err != nil || len(v) == 0 || len(v[0]) == 0 {
		return "", err
	}
	return v[0][0], nil
}

func values(client *http.Client, id, rng, render string) ([][]string, error) {
	var r struct {
		Values [][]any `json:"values"`
	}
	u := sheetURL(id) + "/values/" + url.PathEscape(rng) + "?" + url.Values{"valueRenderOption": {render}, "majorDimension": {"ROWS"}}.Encode()
	if err := do(client, "GET", u, nil, &r); err != nil {
		return nil, err
	}
	out := make([][]string, len(r.Values))
	for i, row := range r.Values {
		out[i] = make([]string, len(row))
		for j, v := range row {
			out[i][j] = fmt.Sprint(v)
		}
	}
	return out, nil
}

// updateBody is the values.update body for one cell.
func updateBody(rng, value string) map[string]any {
	return map[string]any{"range": rng, "majorDimension": "ROWS", "values": [][]string{{value}}}
}

// Update sets one cell as if typed (formulas work) and returns what it now shows.
func Update(client *http.Client, id, sheet string, row, col int, value string) (string, error) {
	rng := Cell(sheet, row, col)
	q := url.Values{"valueInputOption": {"USER_ENTERED"}, "includeValuesInResponse": {"true"}, "responseValueRenderOption": {"FORMATTED_VALUE"}}
	var r struct {
		UpdatedData struct {
			Values [][]any `json:"values"`
		} `json:"updatedData"`
	}
	if err := do(client, "PUT", sheetURL(id)+"/values/"+url.PathEscape(rng)+"?"+q.Encode(), updateBody(rng, value), &r); err != nil {
		return "", err
	}
	if len(r.UpdatedData.Values) > 0 && len(r.UpdatedData.Values[0]) > 0 {
		return fmt.Sprint(r.UpdatedData.Values[0][0]), nil
	}
	return "", nil // the cell is empty now
}

// Append adds a row after the last row with data.
func Append(client *http.Client, id, sheet string, cells []string) error {
	rng := quoteSheet(sheet) + "!A1"
	q := url.Values{"valueInputOption": {"USER_ENTERED"}, "insertDataOption": {"INSERT_ROWS"}}
	body := map[string]any{"range": rng, "majorDimension": "ROWS", "values": [][]string{cells}}
	return do(client, "POST", sheetURL(id)+"/values/"+url.PathEscape(rng)+":append?"+q.Encode(), body, nil)
}
