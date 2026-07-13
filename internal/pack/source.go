package pack

import (
	"io/fs"

	interviewlabs "github.com/openbuzz/interview-labs"
)

// Resolve loads a pack by reference: the embedded names ("", "default",
// "template") or a local directory path.
func Resolve(ref string) (*Pack, error) {
	switch ref {
	case "", "default":
		return loadEmbedded("default")
	case "template":
		return loadEmbedded("template")
	}
	return LoadDir(ref)
}

func loadEmbedded(name string) (*Pack, error) {
	sub, err := fs.Sub(interviewlabs.PacksFS, "packs/"+name)
	if err != nil {
		return nil, err
	}
	return Load(sub)
}
