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
		filepath.Join("cloudflare", "main.tf"),
	} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Fatalf("staged file missing: %s: %v", f, err)
		}
	}
}

func TestWriteTfvars(t *testing.T) {
	dir := t.TempDir()
	err := WriteTfvars(dir, "digitalocean", "fra1", "s-1vcpu-1gb",
		"ubuntu-26-04-x64", "calm-otter", "/state/ssh", nil)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "terraform.tfvars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vars map[string]string
	if err := json.Unmarshal(data, &vars); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"cloud_provider": "digitalocean", "region": "fra1", "size": "s-1vcpu-1gb",
		"image": "ubuntu-26-04-x64", "slug": "calm-otter", "ssh_dir": "/state/ssh",
	}
	for k, v := range want {
		if vars[k] != v {
			t.Fatalf("tfvars[%s] = %q, want %q", k, vars[k], v)
		}
	}
	if _, ok := vars["token"]; ok || len(vars) != len(want) {
		t.Fatalf("unexpected tfvars keys: %v", vars)
	}
}

func TestWriteTfvarsMergesExtra(t *testing.T) {
	dir := t.TempDir()
	extra := map[string]any{
		"dns_enabled":        true,
		"cloudflare_zone_id": "z1",
		"dns_base_domain":    "example.test",
	}
	if err := WriteTfvars(dir, "digitalocean", "fra1", "s-1vcpu-1gb",
		"ubuntu-26-04-x64", "calm-otter", "/state/ssh", extra); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "terraform.tfvars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vars map[string]any
	if err := json.Unmarshal(data, &vars); err != nil {
		t.Fatal(err)
	}
	if vars["dns_enabled"] != true || vars["cloudflare_zone_id"] != "z1" ||
		vars["dns_base_domain"] != "example.test" {
		t.Fatalf("extra vars not merged: %v", vars)
	}
	if vars["cloud_provider"] != "digitalocean" {
		t.Fatalf("base vars lost: %v", vars)
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
