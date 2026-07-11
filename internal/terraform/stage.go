package terraform

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	interviewlabs "github.com/openbuzz/interview-labs"
)

// Stage copies the embedded terraform tree into dst.
func Stage(dst string) error {
	src, err := fs.Sub(interviewlabs.InfraFS, "terraform")
	if err != nil {
		return err
	}
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// WriteTfvars writes terraform.tfvars.json (auto-loaded by tf); extra
// carries capability-provider variables merged over the base six.
func WriteTfvars(dir, provider, region, size, image, slug, sshDir string,
	extra map[string]any) error {
	vars := map[string]any{
		"cloud_provider": provider,
		"region":         region,
		"size":           size,
		"image":          image,
		"slug":           slug,
		"ssh_dir":        sshDir,
	}
	for k, v := range extra {
		vars[k] = v
	}
	data, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "terraform.tfvars.json"),
		append(data, '\n'), 0o644)
}
