package terraform

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStageCopiesTree(t *testing.T) {
	dst := t.TempDir()
	if err := Stage(dst); err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{
		"main.tf", "variables.tf", "outputs.tf", "versions.tf", ".terraform.lock.hcl",
		filepath.Join("digitalocean", "main.tf"),
	} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Fatalf("staged file missing: %s: %v", f, err)
		}
	}
}

func TestWriteTfvars(t *testing.T) {
	dir := t.TempDir()
	if err := WriteTfvars(dir, "fra1", "s-1vcpu-1gb", Image, "calm-otter-7f3k"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "terraform.tfvars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"region": "fra1", "size": "s-1vcpu-1gb",
		"image": "ubuntu-26-04-x64", "slug": "calm-otter-7f3k",
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("tfvars[%s] = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("tfvars has extra keys: %v", got)
	}
}

func TestValidateWithRealBinary(t *testing.T) {
	if os.Getenv("INTERVIEW_TF_TEST") == "" {
		t.Skip("set INTERVIEW_TF_TEST=1 to run terraform validate")
	}
	bin, err := Find()
	if err != nil {
		t.Skip("no tf binary on PATH")
	}

	dst := t.TempDir()
	if err := Stage(dst); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-backend=false"}, {"validate"}} {
		cmd := exec.Command(bin.Path, args...)
		cmd.Dir = dst
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
}
