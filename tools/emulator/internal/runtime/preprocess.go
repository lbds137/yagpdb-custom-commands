// Package runtime provides template preprocessing for YAGPDB compatibility.
package runtime

import (
	"regexp"
	"strings"
)

// PreprocessTemplate converts YAGPDB-specific template syntax to standard Go templates.
// This handles:
// - try-catch blocks (converts to simplified error-safe execution)
// - Other YAGPDB extensions as needed
func PreprocessTemplate(source string) string {
	source = preprocessTryCatch(source)
	return source
}

// preprocessTryCatch converts {{ try }}...{{ catch }}...{{ end }} blocks.
// Since we can't truly implement try-catch in standard Go templates,
// we keep the try block content and remove the catch block.
func preprocessTryCatch(source string) string {
	// Process try-catch blocks iteratively from innermost to outermost
	maxIterations := 100 // Safety limit for deeply nested templates
	for i := 0; i < maxIterations; i++ {
		result, changed := processSingleTryCatch(source)
		if !changed {
			break
		}
		source = result
	}
	return source
}

// processSingleTryCatch finds and processes one try-catch block.
// Returns the modified source and whether a replacement was made.
func processSingleTryCatch(source string) (string, bool) {
	tryPattern := regexp.MustCompile(`\{\{\s*try\s*\}\}`)
	catchPattern := regexp.MustCompile(`\{\{\s*catch\s*\}\}`)
	endPattern := regexp.MustCompile(`\{\{\s*end\s*\}\}`)

	// Find all try positions
	tryMatches := tryPattern.FindAllStringIndex(source, -1)
	if len(tryMatches) == 0 {
		return source, false
	}

	// For each try, find its matching catch and end
	// We need to find the innermost try-catch-end block (no nested try inside)
	for i := len(tryMatches) - 1; i >= 0; i-- {
		tryStart := tryMatches[i][0]
		tryEnd := tryMatches[i][1]

		// Find the next catch after this try
		remainingAfterTry := source[tryEnd:]
		catchMatch := catchPattern.FindStringIndex(remainingAfterTry)
		if catchMatch == nil {
			continue
		}

		catchStart := tryEnd + catchMatch[0]
		catchEnd := tryEnd + catchMatch[1]

		// Check if there's another try between this try and the catch
		// If so, this isn't the innermost block
		betweenTryAndCatch := source[tryEnd:catchStart]
		if tryPattern.MatchString(betweenTryAndCatch) {
			continue
		}

		// Find the next end after the catch
		remainingAfterCatch := source[catchEnd:]
		endMatch := endPattern.FindStringIndex(remainingAfterCatch)
		if endMatch == nil {
			continue
		}

		endStart := catchEnd + endMatch[0]
		endEnd := catchEnd + endMatch[1]

		// Check if there's another catch between our catch and end
		// (which would indicate nested structure we need to handle differently)
		betweenCatchAndEnd := source[catchEnd:endStart]
		if catchPattern.MatchString(betweenCatchAndEnd) {
			continue
		}

		// We found a valid innermost try-catch-end block
		tryContent := source[tryEnd:catchStart]

		// Replace the entire block with just the try content
		result := source[:tryStart] + tryContent + source[endEnd:]
		return result, true
	}

	return source, false
}

// PreprocessForParsing handles constructs that would cause parse errors.
// This is more aggressive than PreprocessTemplate - it makes the template parseable
// even if it loses some semantic meaning.
func PreprocessForParsing(source string) string {
	source = PreprocessTemplate(source)

	// Additional parsing fixups can go here

	return source
}

// StripComments removes YAGPDB template comments for cleaner output.
func StripComments(source string) string {
	// Remove {{/* ... */}} comments
	pattern := regexp.MustCompile(`(?s)\{\{/\*.*?\*/\}\}`)
	return pattern.ReplaceAllString(source, "")
}

// NormalizeWhitespace cleans up excessive whitespace in template output.
func NormalizeWhitespace(output string) string {
	// Collapse multiple newlines to a maximum of 2
	pattern := regexp.MustCompile(`\n{3,}`)
	output = pattern.ReplaceAllString(output, "\n\n")

	// Trim leading/trailing whitespace
	output = strings.TrimSpace(output)

	return output
}
