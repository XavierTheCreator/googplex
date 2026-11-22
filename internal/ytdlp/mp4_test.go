package ytdlp

import (
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mp4Box(typ string, body []byte) []byte {
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], typ)
	return append(b, body...)
}

// sidxV0 builds a version-0 sidx with the given segment sizes and durations.
func sidxV0(timescale, earliest, firstOffset uint32, sizes, durs []uint32) []byte {
	p := make([]byte, 4+4+4+4+4+2+2)
	binary.BigEndian.PutUint32(p[4:], 1) // reference_ID
	binary.BigEndian.PutUint32(p[8:], timescale)
	binary.BigEndian.PutUint32(p[12:], earliest)
	binary.BigEndian.PutUint32(p[16:], firstOffset)
	binary.BigEndian.PutUint16(p[22:], uint16(len(sizes)))
	for i := range sizes {
		e := make([]byte, 12)
		binary.BigEndian.PutUint32(e, sizes[i])
		binary.BigEndian.PutUint32(e[4:], durs[i])
		binary.BigEndian.PutUint32(e[8:], 0x90000000) // starts with SAP
		p = append(p, e...)
	}
	return mp4Box("sidx", p)
}

func fakeDash() []byte {
	file := mp4Box("ftyp", []byte("dash\x00\x00\x00\x00iso6mp41"))
	file = append(file, mp4Box("moov", make([]byte, 100))...)
	file = append(file, sidxV0(1000, 0, 0, []uint32{500, 700, 300}, []uint32{5000, 5000, 2500})...)
	file = append(file, make([]byte, 1500)...) // the segments
	return file
}

func TestParseIndex(t *testing.T) {
	file := fakeDash()
	idx, need, err := parseIndex(file)
	if err != nil || need != 0 {
		t.Fatalf("need %d, err %v", need, err)
	}
	moovEnd := int64(8+16) + int64(8+100)
	if idx.InitEnd != moovEnd-1 || len(idx.Segments) != 3 {
		t.Fatalf("index = %+v", idx)
	}
	sidxEnd := moovEnd + int64(8+24+3*12)
	want := []Segment{{sidxEnd, sidxEnd + 499, 0, 5}, {sidxEnd + 500, sidxEnd + 1199, 5, 5}, {sidxEnd + 1200, sidxEnd + 1499, 10, 2.5}}
	for i, s := range want {
		if idx.Segments[i] != s {
			t.Errorf("segment %d = %+v, want %+v", i, idx.Segments[i], s)
		}
	}
	if idx.Segments[2].End != int64(len(file))-1 {
		t.Errorf("last segment should end at the file's end")
	}

	// Cut off inside the sidx: asks for more.
	if _, need, err := parseIndex(file[:moovEnd+20]); err != nil || need != sidxEnd {
		t.Errorf("partial: need %d, err %v", need, err)
	}
	// No sidx before the media: an error, not a loop.
	noIndex := append(append(mp4Box("ftyp", nil), mp4Box("moov", nil)...), mp4Box("moof", nil)...)
	if _, _, err := parseIndex(noIndex); err == nil {
		t.Error("missing sidx should fail")
	}
}

func TestIndexOfFetchesMore(t *testing.T) {
	file := fakeDash()
	var reqs []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs = append(reqs, r.Header.Get("Range"))
		start, end, _ := parseRange(r.Header.Get("Range"))
		if end >= int64(len(file)) {
			end = int64(len(file)) - 1
		}
		w.WriteHeader(206)
		w.Write(file[start : end+1])
	}))
	defer up.Close()
	idx, err := IndexOf(httptest.NewRequest("GET", "/", nil), Stream{}, up.URL+"/f")
	if err != nil || len(idx.Segments) != 3 {
		t.Fatalf("index %+v, %v", idx, err)
	}
	if again, _ := IndexOf(httptest.NewRequest("GET", "/", nil), Stream{}, up.URL+"/f"); len(again.Segments) != 3 || len(reqs) != 1 {
		t.Errorf("second call should be cached: %v", reqs)
	}
}
