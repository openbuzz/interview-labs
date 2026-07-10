// Package config reads and writes the interview config file.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DigitalOcean is the DigitalOcean provider configuration.
type DigitalOcean struct {
	Token    string `yaml:"token"`
	Region   string `yaml:"region"`
	Instance string `yaml:"instance"`
}

// Hetzner is the Hetzner Cloud provider configuration.
type Hetzner struct {
	Token    string `yaml:"token"`
	Region   string `yaml:"region"`
	Instance string `yaml:"instance"`
}

// AWS is the AWS provider configuration (long-term IAM user credentials).
type AWS struct {
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Region          string `yaml:"region"`
	Instance        string `yaml:"instance"`
}

// Providers holds per-provider configuration.
type Providers struct {
	DigitalOcean DigitalOcean `yaml:"digitalocean"`
	Hetzner      Hetzner      `yaml:"hetzner"`
	AWS          AWS          `yaml:"aws"`
}

// Roles maps a role to the provider that fulfills it.
type Roles struct {
	VM string `yaml:"vm"`
}

// Config is the whole config.yaml.
type Config struct {
	Providers Providers `yaml:"providers"`
	Roles     Roles     `yaml:"roles"`
}

// Path returns $XDG_CONFIG_HOME/interview/config.yaml (fallback ~/.config).
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "interview", "config.yaml"), nil
}

// Load reads the config; a missing file yields a zero Config.
func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Write persists the config atomically with 0600 permissions.
func (c Config) Write() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(p), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}
