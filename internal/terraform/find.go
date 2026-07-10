// Package terraform discovers and drives the host terraform/opentofu binary.
package terraform

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// Binary is a discovered terraform-compatible executable.
type Binary struct {
	Name    string
	Path    string
	Version string
}

type versionJSON struct {
	TerraformVersion string `json:"terraform_version"`
}

// Find locates terraform first, tofu second.
func Find() (Binary, error) {
	for _, name := range []string{"terraform", "tofu"} {
		if b, err := FindNamed(name); err == nil {
			return b, nil
		}
	}
	return Binary{}, fmt.Errorf(
		"neither terraform nor opentofu found on PATH — install one of them " +
			"(https://developer.hashicorp.com/terraform/install or https://opentofu.org)")
}

// FindNamed locates one specific binary ("terraform" or "tofu").
func FindNamed(name string) (Binary, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return Binary{}, err
	}
	out, err := exec.Command(path, "version", "-json").Output()
	if err != nil {
		return Binary{}, fmt.Errorf("%s version: %w", name, err)
	}

	var v versionJSON
	if err := json.Unmarshal(out, &v); err != nil {
		return Binary{}, fmt.Errorf("%s version parse: %w", name, err)
	}
	return Binary{Name: name, Path: path, Version: v.TerraformVersion}, nil
}
