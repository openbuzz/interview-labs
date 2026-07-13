package pack

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// WriteTar streams dir as an uncompressed tar rooted at payload/, headers
// forced to uid/gid 1000 so the candidate owns the extracted tree even
// though the VM extracts as the ssh user.
func WriteTar(w io.Writer, dir string) error {
	tw := tar.NewWriter(w)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		hdr, err := tarHeader(dir, p, info)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return err
	}
	return tw.Close()
}

// tarHeader builds one entry's header rooted at payload/, uid/gid forced to
// 1000 and directory names trailing-slashed.
func tarHeader(dir, p string, info os.FileInfo) (*tar.Header, error) {
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return nil, err
	}
	if rel == "." {
		rel = ""
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, err
	}
	hdr.Name = "payload/" + filepath.ToSlash(rel)
	hdr.Uid, hdr.Gid = 1000, 1000
	hdr.Uname, hdr.Gname = "", ""
	if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
		hdr.Name += "/"
	}
	return hdr, nil
}
