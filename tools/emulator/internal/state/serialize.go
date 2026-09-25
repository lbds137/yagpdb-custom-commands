package state

import (
	"bytes"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/vmihailenco/msgpack"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// YAGPDB registers its template types so they come back as themselves
// (common/templates/context.go); the extensions also count towards a value's size.
func init() {
	msgpack.RegisterExt(1, (*types.SDict)(nil))
	msgpack.RegisterExt(2, (*types.Dict)(nil))
	msgpack.RegisterExt(3, (*types.Slice)(nil))
}

// serializeValue is YAGPDB's (customcommands/tmplextensions.go): the value as stored,
// msgpack-encoded, at most 100000 bytes. A bigger value fails with "short write".
func serializeValue(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	enc := msgpack.NewEncoder(limitWriter(&b, 100000))
	err := enc.Encode(v)
	return b.Bytes(), err
}

// limitedWriter and limitWriter are YAGPDB's LimitWriter (common/templates/context.go).
type limitedWriter struct {
	W io.Writer
	N int64
	i int64
}

func (l *limitedWriter) Write(p []byte) (n int, err error) {
	noLeadingWhitespace := trimLeftSpace(p)
	if l.N == l.i {
		if len(noLeadingWhitespace) < 1 {
			return 0, nil
		} else {
			p = noLeadingWhitespace
		}
	}

	if l.N <= 0 {
		swErr := io.ErrShortWrite
		if len(noLeadingWhitespace) < 1 {
			swErr = nil
		}
		return 0, swErr
	}
	if int64(len(p)) > l.N {
		var cut []byte
		p, cut = p[0:l.N], p[l.N:]
		if len(bytes.TrimSpace(cut)) > 0 {
			err = io.ErrShortWrite
		}
	}
	n, er := l.W.Write(p)
	if er != nil {
		err = er
	}
	l.N -= int64(n)
	return n, err
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

func trimLeftSpace(s []byte) []byte {
	// Fast path for ASCII: look for the first ASCII non-space byte
	start := 0
	for ; start < len(s); start++ {
		c := s[start]
		if c >= utf8.RuneSelf {
			// If we run into a non-ASCII byte, fall back to the
			// slower unicode-aware method on the remaining bytes
			return bytes.TrimLeftFunc(s[start:], unicode.IsSpace)
		}
		if asciiSpace[c] == 0 {
			break
		}
	}

	return s[start:]
}

// limitWriter works like io.LimitReader. It writes at most n bytes
// to the underlying Writer. It returns io.ErrShortWrite if more than n
// bytes are attempted to be written, unless those bytes are exclusively
// whitespace, in which case it will not write them and return without error.
// It will not write leading whitespace.
func limitWriter(w io.Writer, n int64) io.Writer {
	return &limitedWriter{W: w, N: n, i: n}
}
