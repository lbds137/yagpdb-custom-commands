// Copied from YAGPDB (github.com/botlabs-gg/yagpdb, commit 0cf2ec5), common/templates/context.go.
// MIT license, see LICENSE-YAGPDB. Changes: package name.

package yagstd

import (
	"bytes"
	"io"
	"unicode"
	"unicode/utf8"
)

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

// LimitWriter works like io.LimitReader. It writes at most n bytes
// to the underlying Writer. It returns io.ErrShortWrite if more than n
// bytes are attempted to be written, unless those bytes are exclusively
// whitespace, in which case it will not write them and return without error.
// It will not write leading whitespace.
func LimitWriter(w io.Writer, n int64) io.Writer {
	return &limitedWriter{W: w, N: n, i: n}
}
