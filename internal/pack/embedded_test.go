package pack

import (
	"io/fs"
	"testing"

	interviewlabs "github.com/openbuzz/interview-labs"
)

func embeddedPack(t *testing.T, name string) *Pack {
	t.Helper()
	sub, err := fs.Sub(interviewlabs.PacksFS, "packs/"+name)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Load(sub)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

//nolint:cyclop // cyclop: dense field checks over one fixture — a split wouldn't lower it
func TestEmbeddedDefaultPack(t *testing.T) {
	p := embeddedPack(t, "default")
	if p.Name != "default" || len(p.Bundles) != 2 {
		t.Fatalf("default pack = %+v", p)
	}

	be, _ := BundleByName(p, "backend")
	if be == nil || be.Image != "backend" || len(be.Scenarios) != 4 || be.HasKind {
		t.Fatalf("backend bundle = %+v", be)
	}
	dv, _ := BundleByName(p, "devops")
	if dv == nil || dv.Image != "devops" || len(dv.Scenarios) != 3 || !dv.HasKind {
		t.Fatalf("devops bundle = %+v", dv)
	}

	manifests, err := fs.Glob(p.FS, "bundles/devops/kind/manifests/*.yaml")
	if err != nil || len(manifests) != 5 {
		t.Fatalf("manifests = %v, %v", manifests, err)
	}
}

func TestEmbeddedTemplatePack(t *testing.T) {
	p := embeddedPack(t, "template")
	d, _ := BundleByName(p, "demo")
	if p.Name != "template" || d == nil || !d.HasSetup || len(d.Scenarios) != 1 {
		t.Fatalf("template pack = %+v", p)
	}
}

func TestPacksFSShape(t *testing.T) {
	entries, err := fs.ReadDir(interviewlabs.PacksFS, "packs")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "default" ||
		entries[1].Name() != "template" {
		t.Fatalf("packs/ = %v — unexpected embedded content", entries)
	}
}
