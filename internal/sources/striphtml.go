package sources

import (
	"html"
	"regexp"
	"strings"
)

var tagRe = regexp.MustCompile(`<[^>]*>`)

// stripHTML flattens feed description HTML to plain text for classification.
func stripHTML(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
