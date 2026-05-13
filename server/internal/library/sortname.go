package library

import "strings"

// leadingArticles are case-insensitive words that, when they appear as the
// first word followed by whitespace, are stripped to produce a sort_name.
// Covers English, German, and French — the languages most music libraries
// in those locales actually contain. "Theory of a Deadman" stays put because
// only the *first* word is examined.
var leadingArticles = []string{
	"the", "a", "an", // EN
	"die", "der", "das", // DE
	"le", "la", "les", // FR
}

// computeSortName returns name with a leading article + whitespace stripped.
// "The Beatles" → "Beatles". If the input is just an article ("The") or has
// no following word, the original name is returned so the artist still has
// a non-empty sort key.
func computeSortName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return name
	}
	first, rest, found := splitFirstWord(trimmed)
	if !found {
		// Single-word input — nothing to strip.
		return name
	}
	for _, article := range leadingArticles {
		if strings.EqualFold(first, article) {
			return rest
		}
	}
	return name
}

// splitFirstWord splits "Foo  Bar" into ("Foo", "Bar", true), trimming any
// extra whitespace between words. Returns found=false if there's no space.
func splitFirstWord(s string) (first, rest string, found bool) {
	idx := strings.IndexAny(s, " \t")
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], strings.TrimLeft(s[idx:], " \t"), true
}
