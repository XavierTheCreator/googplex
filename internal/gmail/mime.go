package gmail

import (
	"fmt"
	"io"
	"mime"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
)

// mimeDecoder decodes RFC 2047 headers in any charset Go's text package knows.
type mimeDecoder struct{ mime.WordDecoder }

func (d *mimeDecoder) DecodeHeader(s string) (string, error) {
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		enc, err := htmlindex.Get(strings.ToLower(charset))
		if err != nil {
			return nil, fmt.Errorf("unknown charset %s", charset)
		}
		return enc.NewDecoder().Reader(input), nil
	}
	return d.WordDecoder.DecodeHeader(s)
}
