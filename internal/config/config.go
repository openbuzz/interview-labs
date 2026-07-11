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

// OpenRouter is the OpenRouter provider configuration. The management key
// mints per-session API keys; cap_usd is the per-session spend cap in USD
// (0 means the built-in default applies at use).
type OpenRouter struct {
	ManagementKey string  `yaml:"management_key"`
	CapUSD        float64 `yaml:"cap_usd"`
}

// Cloudflare is the Cloudflare provider configuration.
type Cloudflare struct {
	APIToken string `yaml:"api_token"`
	ZoneID   string `yaml:"zone_id"`
	Domain   string `yaml:"domain"`
}

// Providers holds per-provider configuration.
type Providers struct {
	DigitalOcean DigitalOcean `yaml:"digitalocean"`
	Hetzner      Hetzner      `yaml:"hetzner"`
	AWS          AWS          `yaml:"aws"`
	OpenRouter   OpenRouter   `yaml:"openrouter"`
	Cloudflare   Cloudflare   `yaml:"cloudflare"`
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
