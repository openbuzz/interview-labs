package session

import petname "github.com/dustinkirkland/golang-petname"

// newSlug mints a two-word petname, retrying while exists reports true.
func newSlug(exists func(string) bool) string {
	for {
		slug := petname.Generate(2, "-")
		if !exists(slug) {
			return slug
		}
	}
}
