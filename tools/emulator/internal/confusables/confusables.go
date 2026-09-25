// Copied from YAGPDB (github.com/botlabs-gg/yagpdb, commit 0cf2ec5), lib/confusables/confusables.go.
// MIT license, see ../yagstd/LICENSE-YAGPDB. Changes: no go:generate line; the replacer is built in init() instead of a
// logging Init().

package confusables

import (
	"net/url"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var replacer strings.Replacer

// YAGPDB calls Init at startup; the emulator builds the replacer when the package loads.
func init() {
	replacer = *strings.NewReplacer(append(confusables, diacritics...)...)
}

// SanitizeText normalizes text and matches confusables.
// i.e. "Ĥéĺĺó" -> "Hello".
func SanitizeText(content string) string {
	content = NormalizeQueryEncodedText(content)
	return replacer.Replace(content)
}

// Normalizes QueryEscaped content in a string.
// Example: "Hello%20World%20%"" will be normalized to "Hello World "
func NormalizeQueryEncodedText(content string) string {
	decoded, err := url.QueryUnescape(content)
	if err != nil {
		decoded = content
	}
	decoded = norm.NFC.String(decoded)
	return decoded
}
