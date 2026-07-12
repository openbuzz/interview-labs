package interviewlabs

import (
	"io/fs"
	"strings"
	"testing"
)

// A terraform init run inside terraform/ leaves .terraform/ with provider
// binaries; embedding those ships a broken tree (Stage writes files 0644).
func TestInfraFSHasSourcesOnly(t *testing.T) {
	for _, want := range []string{
		"terraform/main.tf",
		"terraform/.terraform.lock.hcl",
		"terraform/digitalocean/main.tf",
		"terraform/hetzner/main.tf",
		"terraform/aws/main.tf",
		"terraform/cloudflare/main.tf",
	} {
		if _, err := fs.Stat(InfraFS, want); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}

	err := fs.WalkDir(InfraFS, "terraform", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(path, "terraform/.terraform/") {
			t.Errorf("embedded terraform init artifact: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The docker embed carries exactly the compose file: build contexts stay in
// the repo, images are pulled prebuilt.
func TestDockerFSIsComposeOnly(t *testing.T) {
	var files []string
	err := fs.WalkDir(DockerFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "docker/compose.yaml" {
		t.Errorf("DockerFS files = %v, want docker/compose.yaml only", files)
	}
}
