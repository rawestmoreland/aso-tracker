// Package keywords parses keyword list files.
package keywords

import (
	"bufio"
	"io"
	"strings"
)

// Parse reads one keyword per line. Blank lines and lines starting with #
// are skipped. Terms are trimmed, lowercased, whitespace-collapsed and
// de-duplicated while preserving first-seen order.
func Parse(r io.Reader) ([]string, error) {
	var out []string
	seen := map[string]bool{}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		term := strings.ToLower(strings.Join(strings.Fields(line), " "))
		if seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out, sc.Err()
}
