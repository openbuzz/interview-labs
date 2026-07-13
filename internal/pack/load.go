package pack

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"sigs.k8s.io/yaml"

	interviewlabs "github.com/openbuzz/interview-labs"
)

type compiledSchemas struct {
	pack, bundle, scenario *jsonschema.Schema
}

func compileSchemas() (*compiledSchemas, error) {
	p, err := compileSpecSchema("pack.schema.json")
	if err != nil {
		return nil, err
	}
	b, err := compileSpecSchema("bundle.schema.json")
	if err != nil {
		return nil, err
	}
	s, err := compileSpecSchema("scenario.schema.json")
	if err != nil {
		return nil, err
	}
	return &compiledSchemas{pack: p, bundle: b, scenario: s}, nil
}

// compileSpecSchema compiles one spec/pack/v1 schema from the embedded spec
// tree, so Load works offline — the https $id is a label, nothing fetches.
func compileSpecSchema(filename string) (*jsonschema.Schema, error) {
	raw, err := interviewlabs.SpecFS.ReadFile(path.Join("spec/pack/v1", filename))
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", filename, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode schema %s: %w", filename, err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(filename, doc); err != nil {
		return nil, fmt.Errorf("add schema %s: %w", filename, err)
	}
	schema, err := c.Compile(filename)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", filename, err)
	}
	return schema, nil
}

// Wire shapes of the three manifests, decoded after schema validation.
type packManifest struct {
	Contract    int    `json:"contract"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type bundleManifest struct {
	Description string `json:"description"`
	Image       string `json:"image"`
}

type scenarioManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// loadManifest reads p, validates against schema, then decodes into out.
func loadManifest(fsys fs.FS, p string, schema *jsonschema.Schema, out any) error {
	raw, err := fs.ReadFile(fsys, p)
	if err != nil {
		return fmt.Errorf("read %s: %w", p, err)
	}

	jsonBytes, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return fmt.Errorf("convert %s to json: %w", p, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("decode %s: %w", p, err)
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	return yaml.Unmarshal(raw, out)
}

func dirExists(fsys fs.FS, p string) bool {
	info, err := fs.Stat(fsys, p)
	return err == nil && info.IsDir()
}

func fileExists(fsys fs.FS, p string) bool {
	info, err := fs.Stat(fsys, p)
	return err == nil && !info.IsDir()
}

// Load parses and validates a pack rooted at fsys: schema-first, then the
// semantic rules schemas can't express (dir charsets, task/ presence, kind
// pairing). Any violation is one descriptive error.
func Load(fsys fs.FS) (*Pack, error) {
	schemas, err := compileSchemas()
	if err != nil {
		return nil, err
	}

	var pm packManifest
	if err := loadManifest(fsys, "pack.yaml", schemas.pack, &pm); err != nil {
		return nil, err
	}

	bundles, err := loadBundles(fsys, schemas)
	if err != nil {
		return nil, err
	}
	return &Pack{Contract: pm.Contract, Name: pm.Name, Version: pm.Version,
		Description: pm.Description, Bundles: bundles, FS: fsys}, nil
}

func loadBundles(fsys fs.FS, schemas *compiledSchemas) ([]Bundle, error) {
	entries, err := fs.ReadDir(fsys, "bundles")
	if err != nil {
		return nil, fmt.Errorf("read bundles: %w", err)
	}

	var bundles []Bundle
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := loadBundle(fsys, schemas, e.Name())
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
	}
	if len(bundles) == 0 {
		return nil, fmt.Errorf("pack has no bundles")
	}
	return bundles, nil
}

func loadBundle(fsys fs.FS, schemas *compiledSchemas, name string) (Bundle, error) {
	if err := validateDirName("bundle", name); err != nil {
		return Bundle{}, err
	}
	dir := path.Join("bundles", name)

	var bm bundleManifest
	if err := loadManifest(fsys, path.Join(dir, "bundle.yaml"),
		schemas.bundle, &bm); err != nil {
		return Bundle{}, err
	}

	hasKind := fileExists(fsys, path.Join(dir, "kind/cluster.yaml"))
	if err := validateKindPairing(fsys, dir, hasKind); err != nil {
		return Bundle{}, err
	}
	scenarios, err := loadScenarios(fsys, schemas, name)
	if err != nil {
		return Bundle{}, err
	}

	return Bundle{
		Name: name, Description: bm.Description, Image: bm.Image,
		HasLab:   dirExists(fsys, path.Join(dir, "lab")),
		HasSetup: fileExists(fsys, path.Join(dir, "lab/setup.sh")),
		HasKind:  hasKind, Scenarios: scenarios,
	}, nil
}

func loadScenarios(fsys fs.FS, schemas *compiledSchemas,
	bundle string) ([]Scenario, error) {
	dir := path.Join("bundles", bundle, "scenarios")
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read scenarios for bundle %s: %w", bundle, err)
	}

	var out []Scenario
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := validateDirName("scenario", e.Name()); err != nil {
			return nil, err
		}
		sdir := path.Join(dir, e.Name())
		if !dirExists(fsys, path.Join(sdir, "task")) {
			return nil, fmt.Errorf("scenario %s/%s: task/ directory required",
				bundle, e.Name())
		}

		var sm scenarioManifest
		if err := loadManifest(fsys, path.Join(sdir, "scenario.yaml"),
			schemas.scenario, &sm); err != nil {
			return nil, err
		}
		out = append(out, Scenario{Name: e.Name(), Title: sm.Name,
			Description: sm.Description})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("bundle %s has no scenarios", bundle)
	}
	return out, nil
}
