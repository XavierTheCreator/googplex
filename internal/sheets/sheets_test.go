package sheets

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestColumnAndRanges(t *testing.T) {
	for i, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 51: "AZ", 52: "BA", 701: "ZZ", 702: "AAA"} {
		if got := Column(i); got != want {
			t.Errorf("Column(%d) = %s, want %s", i, got, want)
		}
	}
	if got := Range("Sheet1", 0, 200, 26); got != "'Sheet1'!A1:Z200" {
		t.Errorf("Range = %s", got)
	}
	if got := Range("Q1 '24", 200, 100, 3); got != "'Q1 ''24'!A201:C300" {
		t.Errorf("Range with quote = %s", got)
	}
	if got := Cell("Data", 4, 27); got != "'Data'!AB5" {
		t.Errorf("Cell = %s", got)
	}
}

type call struct {
	method, path, query string
	body                map[string]any
}

func serve(t *testing.T, reply string) (*[]call, func()) {
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := call{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery}
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			json.Unmarshal(b, &c.body)
		}
		calls = append(calls, c)
		io.WriteString(w, reply)
	}))
	old := api
	api = srv.URL
	return &calls, func() { api = old; srv.Close() }
}

func TestValuesRequest(t *testing.T) {
	calls, done := serve(t, `{"values": [["Name", "Total"], ["Ana", 12.5]]}`)
	defer done()
	v, err := Values(http.DefaultClient, "s1", "My Tab", 0, 200, 26)
	if err != nil || len(v) != 2 || v[1][1] != "12.5" {
		t.Fatalf("values = %v, %v", v, err)
	}
	c := (*calls)[0]
	if c.path != "/spreadsheets/s1/values/%27My%20Tab%27%21A1:Z200" || c.query != "majorDimension=ROWS&valueRenderOption=FORMATTED_VALUE" {
		t.Errorf("request = %s?%s", c.path, c.query)
	}
}

func TestUpdateBody(t *testing.T) {
	calls, done := serve(t, `{"updatedData": {"values": [["42"]]}}`)
	defer done()
	shown, err := Update(http.DefaultClient, "s1", "Sheet1", 1, 2, "=SUM(A1:A3)")
	if err != nil || shown != "42" {
		t.Fatalf("update = %q, %v", shown, err)
	}
	c := (*calls)[0]
	if c.method != "PUT" || c.path != "/spreadsheets/s1/values/%27Sheet1%27%21C2" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	if c.query != "includeValuesInResponse=true&responseValueRenderOption=FORMATTED_VALUE&valueInputOption=USER_ENTERED" {
		t.Errorf("query = %s", c.query)
	}
	if c.body["range"] != "'Sheet1'!C2" || c.body["values"].([]any)[0].([]any)[0] != "=SUM(A1:A3)" || c.body["majorDimension"] != "ROWS" {
		t.Errorf("body = %v", c.body)
	}
}

func TestAppendBody(t *testing.T) {
	calls, done := serve(t, `{}`)
	defer done()
	if err := Append(http.DefaultClient, "s1", "Sheet1", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.method != "POST" || c.path != "/spreadsheets/s1/values/%27Sheet1%27%21A1:append" || c.query != "insertDataOption=INSERT_ROWS&valueInputOption=USER_ENTERED" {
		t.Errorf("request = %s %s?%s", c.method, c.path, c.query)
	}
	if row := c.body["values"].([]any)[0].([]any); len(row) != 2 || row[1] != "b" {
		t.Errorf("body = %v", c.body)
	}
}

func TestFormula(t *testing.T) {
	calls, done := serve(t, `{"values": [["=A1*2"]]}`)
	defer done()
	f, err := Formula(http.DefaultClient, "s1", "Sheet1", 0, 1)
	if err != nil || f != "=A1*2" || (*calls)[0].query != "majorDimension=ROWS&valueRenderOption=FORMULA" {
		t.Errorf("formula = %q %v %s", f, err, (*calls)[0].query)
	}
}
