package pack

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// goodFS loads the on-disk good fixture.
func goodFS(t *testing.T) *Pack {
	t.Helper()
	p, err := Load(os.DirFS("testdata/goodpack"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

//nolint:cyclop // cyclop: dense field checks over one fixture — a split wouldn't lower it
func TestLoadGoodPack(t *testing.T) {
	p := goodFS(t)
	if p.Name != "goodpack" || p.Contract != 1 || len(p.Bundles) != 2 {
		t.Fatalf("pack = %+v", p)
	}

	be, ok := BundleByName(p, "backend")
	if !ok || be.Image != "backend" || len(be.Scenarios) != 1 || be.HasKind || be.HasLab {
		t.Fatalf("backend = %+v", be)
	}
	dv, ok := BundleByName(p, "devops")
	if !ok || !dv.HasKind || !dv.HasLab || !dv.HasSetup {
		t.Fatalf("devops = %+v", dv)
	}
	if dv.Scenarios[0].Title != "Broken pods" {
		t.Fatalf("scenario title = %q", dv.Scenarios[0].Title)
	}
}

// badPack clones the wire shape of a minimal pack in-memory and applies one mutation.
func badPack(mut func(m map[string]string)) fstest.MapFS {
	files := map[string]string{
		"pack.yaml":                   "contract: 1\nname: bad\nversion: 1.0.0\n",
		"bundles/backend/bundle.yaml": "description: d\nimage: backend\n",
		"bundles/backend/scenarios/one/scenario.yaml":  "name: One\n",
		"bundles/backend/scenarios/one/task/README.md": "x\n",
	}
	mut(files)
	fsys := fstest.MapFS{}
	for k, v := range files {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return fsys
}

func TestLoadRejections(t *testing.T) {
	cases := []struct {
		name, wantErr string
		mut           func(m map[string]string)
	}{
		{"bad contract", "contract", func(m map[string]string) {
			m["pack.yaml"] = "contract: 2\nname: bad\nversion: 1.0.0\n"
		}},
		{"bad pack name charset", "pattern", func(m map[string]string) {
			m["pack.yaml"] = "contract: 1\nname: Bad_Name\nversion: 1.0.0\n"
		}},
		{"bad image enum", "image", func(m map[string]string) {
			m["bundles/backend/bundle.yaml"] = "description: d\nimage: gpu\n"
		}},
		{"unknown bundle key", "additional", func(m map[string]string) {
			m["bundles/backend/bundle.yaml"] = "description: d\nimage: backend\nenv: [X]\n"
		}},
		{"bad scenario dir charset", "scenario", func(m map[string]string) {
			m["bundles/backend/scenarios/One_Bad/scenario.yaml"] = "name: X\n"
			m["bundles/backend/scenarios/One_Bad/task/README.md"] = "x\n"
		}},
		{"scenario without task", "task", func(m map[string]string) {
			delete(m, "bundles/backend/scenarios/one/task/README.md")
		}},
		{"manifests without cluster", "cluster.yaml", func(m map[string]string) {
			m["bundles/backend/kind/manifests/00-ns.yaml"] = "apiVersion: v1\n"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(badPack(c.mut))
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, c.wantErr)
			}
		})
	}
}

func TestLoadRejectsEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	root := dir + "/pack"
	for p, data := range map[string]string{
		"pack.yaml":                            "contract: 1\nname: sym\nversion: 1.0.0\n",
		"bundles/b/bundle.yaml":                "description: d\nimage: backend\n",
		"bundles/b/scenarios/s/scenario.yaml":  "name: S\n",
		"bundles/b/scenarios/s/task/README.md": "x\n",
	} {
		mustWrite(t, root+"/"+p, data)
	}
	mustWrite(t, dir+"/outside.txt", "secret")
	if err := os.Symlink(dir+"/outside.txt",
		root+"/bundles/b/scenarios/s/task/link"); err != nil {
		t.Fatal(err)
	}

	_, err := LoadDir(root)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("err = %v, want symlink rejection", err)
	}
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(path[:strings.LastIndex(path, "/")], 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
