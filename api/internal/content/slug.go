package content

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

// SlugRe is the slug rule shared with the site (schemas.ts `slug`).
var SlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,99}$`)

var pinyinArgs = pinyin.NewArgs()

// Slugify derives a URL slug from a title: Han characters become pinyin, everything else is
// lower-cased ASCII letters and digits joined by single hyphens.
func Slugify(title string) string {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range title {
		switch {
		case unicode.Is(unicode.Han, r):
			flush()
			if py := pinyin.LazyPinyin(string(r), pinyinArgs); len(py) > 0 {
				words = append(words, py[0])
			}
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			cur.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			cur.WriteRune(unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	s := strings.Join(words, "-")
	if len(s) > 80 {
		s = s[:80]
		s = strings.TrimRight(s, "-")
	}
	return s
}
