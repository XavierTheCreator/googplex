package ytdlp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// YouTube's DASH files are fragmented MP4: ftyp and moov (the "init
// segment" a Media Source player appends first), then a sidx box listing
// every media segment's byte range and duration, then the segments. Index
// reads that layout so the player can fetch and seek segment by segment.

// Segment is one media segment: bytes [Start, End] cover [Time, Time+Dur) seconds.
type Segment struct {
	Start int64   `json:"start"`
	End   int64   `json:"end"`
	Time  float64 `json:"time"`
	Dur   float64 `json:"dur"`
}

type Index struct {
	InitEnd  int64     `json:"initEnd"` // last byte of ftyp+moov
	Segments []Segment `json:"segments"`
}

type box struct {
	typ        string
	start, end int64 // end is exclusive
	header     int64
}

// readBox reads the box header at off within b (which starts at file offset base).
func readBox(b []byte, base, off int64) (box, bool) {
	i := off - base
	if i < 0 || i+8 > int64(len(b)) {
		return box{}, false
	}
	size := int64(binary.BigEndian.Uint32(b[i:]))
	typ := string(b[i+4 : i+8])
	header := int64(8)
	if size == 1 {
		if i+16 > int64(len(b)) {
			return box{}, false
		}
		size = int64(binary.BigEndian.Uint64(b[i+8:]))
		header = 16
	}
	if size < header {
		return box{}, false
	}
	return box{typ: typ, start: off, end: off + size, header: header}, true
}

// parseIndex finds moov and sidx in the first bytes of the file. need > 0
// means more bytes are required (read at least that many from the start).
func parseIndex(b []byte) (idx Index, need int64, err error) {
	var off int64
	var moovEnd int64 = -1
	for {
		bx, ok := readBox(b, 0, off)
		if !ok {
			return Index{}, off + 64*1024, nil
		}
		switch bx.typ {
		case "moov":
			moovEnd = bx.end
		case "sidx":
			if bx.end > int64(len(b)) {
				return Index{}, bx.end, nil
			}
			if moovEnd < 0 {
				return Index{}, 0, errors.New("mp4: sidx before moov")
			}
			segs, err := parseSidx(b[bx.start+bx.header:bx.end], bx.end)
			if err != nil {
				return Index{}, 0, err
			}
			return Index{InitEnd: moovEnd - 1, Segments: segs}, 0, nil
		case "moof", "mdat":
			return Index{}, 0, errors.New("mp4: no sidx index")
		}
		off = bx.end
		if off > 4<<20 {
			return Index{}, 0, errors.New("mp4: index not found in the first 4 MB")
		}
	}
}

// parseSidx reads a sidx body (after the box header); after is the file
// offset just past the sidx box, where first_offset counts from.
func parseSidx(p []byte, after int64) ([]Segment, error) {
	if len(p) < 4 {
		return nil, errors.New("mp4: short sidx")
	}
	version := p[0]
	p = p[4:] // version + flags
	need := 8 + 8 + 4
	if version != 0 {
		need = 8 + 16 + 4
	}
	if len(p) < need {
		return nil, errors.New("mp4: short sidx")
	}
	timescale := float64(binary.BigEndian.Uint32(p[4:]))
	if timescale == 0 {
		return nil, errors.New("mp4: sidx timescale is 0")
	}
	var earliest, firstOffset uint64
	if version == 0 {
		earliest = uint64(binary.BigEndian.Uint32(p[8:]))
		firstOffset = uint64(binary.BigEndian.Uint32(p[12:]))
		p = p[16:]
	} else {
		earliest = binary.BigEndian.Uint64(p[8:])
		firstOffset = binary.BigEndian.Uint64(p[16:])
		p = p[24:]
	}
	count := int(binary.BigEndian.Uint16(p[2:]))
	p = p[4:]
	if len(p) < count*12 {
		return nil, errors.New("mp4: sidx entries cut short")
	}
	segs := make([]Segment, 0, count)
	pos := after + int64(firstOffset)
	t := float64(earliest) / timescale
	for i := 0; i < count; i++ {
		e := p[i*12:]
		ref := binary.BigEndian.Uint32(e)
		if ref>>31 == 1 {
			return nil, errors.New("mp4: nested sidx not supported")
		}
		size := int64(ref & 0x7fffffff)
		dur := float64(binary.BigEndian.Uint32(e[4:])) / timescale
		segs = append(segs, Segment{Start: pos, End: pos + size - 1, Time: t, Dur: dur})
		pos += size
		t += dur
	}
	return segs, nil
}

var (
	indexMu    sync.Mutex
	indexCache = map[string]Index{}
)

// IndexOf fetches the start of one stream file (link is s.URL or s.Audio)
// and returns its segment index. Results are cached per link.
func IndexOf(r *http.Request, s Stream, link string) (Index, error) {
	indexMu.Lock()
	idx, ok := indexCache[link]
	indexMu.Unlock()
	if ok {
		return idx, nil
	}
	var buf []byte
	want := int64(64 * 1024)
	for tries := 0; tries < 5; tries++ {
		resp, err := fetchRange(r, s, link, int64(len(buf)), want-1)
		if err != nil {
			return Index{}, err
		}
		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return Index{}, fmt.Errorf("mp4: index fetch: %s", resp.Status)
		}
		chunk, err := io.ReadAll(io.LimitReader(resp.Body, want-int64(len(buf))))
		resp.Body.Close()
		if err != nil {
			return Index{}, err
		}
		buf = append(buf, chunk...)
		var need int64
		idx, need, err = parseIndex(buf)
		if err != nil {
			return Index{}, err
		}
		if need == 0 {
			indexMu.Lock()
			indexCache[link] = idx
			indexMu.Unlock()
			return idx, nil
		}
		if need <= int64(len(buf)) || len(chunk) == 0 {
			return Index{}, errors.New("mp4: index unreadable")
		}
		want = need
	}
	return Index{}, errors.New("mp4: index too large")
}
