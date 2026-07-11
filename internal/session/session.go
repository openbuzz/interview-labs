// Package session stores per-session state under XDG_STATE_HOME.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"golang.org/x/sys/unix"
)

// Status values for Metadata.Status.
const (
	StatusLaunching     = "launching"
	StatusReady         = "ready"
	StatusFailed        = "failed"
	StatusDestroying    = "destroying"
	StatusFailedDestroy = "failed-destroy"
)

// TerraformInfo records how a session was applied.
type TerraformInfo struct {
	Binary  string `json:"binary"`
	Version string `json:"version"`
}

// Metadata is the metadata.json schema.
type Metadata struct {
	Schema          int               `json:"schema"`
	Slug            string            `json:"slug"`
	CreatedAt       time.Time         `json:"created_at"`
	Region          string            `json:"region"`
	Size            string            `json:"size"`
	Image           string            `json:"image"`
	Profile         string            `json:"profile,omitempty"`
	Roles           map[string]string `json:"roles"`
	SSHUser         string            `json:"ssh_user,omitempty"`
	Terraform       TerraformInfo     `json:"terraform"`
	IP              string            `json:"ip,omitempty"`
	FQDN            string            `json:"fqdn,omitempty"`
	AIKeyHash       string            `json:"ai_key_hash,omitempty"`
	AICapUSD        float64           `json:"ai_cap_usd,omitempty"`
	GatewayPassword string            `json:"gateway_password,omitempty"`
	URL             string            `json:"url,omitempty"`
	Status          string            `json:"status"`
	Phase           string            `json:"phase"`
}

// Session is one lab session on disk.
type Session struct {
	Dir  string
	Meta Metadata
}

// Root returns $XDG_STATE_HOME/interview (fallback ~/.local/state/interview).
func Root() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "interview"), nil
}

func sessionsDir(root string) string { return filepath.Join(root, "sessions") }
func archiveDir(root string) string  { return filepath.Join(root, "archive") }

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// New mints a session: slug, directory layout, initial metadata.
func New(region, size, image, sshUser string, roles map[string]string,
	tf TerraformInfo) (*Session, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	slug := newSlug(func(s string) bool {
		return exists(filepath.Join(sessionsDir(root), s)) ||
			exists(filepath.Join(archiveDir(root), s))
	})

	dir := filepath.Join(sessionsDir(root), slug)
	s := &Session{
		Dir: dir,
		Meta: Metadata{
			Schema:    2,
			Slug:      slug,
			CreatedAt: time.Now().UTC(),
			Region:    region,
			Size:      size,
			Image:     image,
			Roles:     roles,
			SSHUser:   sshUser,
			Terraform: tf,
			Status:    StatusLaunching,
			Phase:     "session",
		},
	}
	for _, d := range []string{s.SSHDir(), s.TerraformDir(), s.LogsDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	return s, nil
}

// List returns all sessions, oldest first.
func List() ([]*Session, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(sessionsDir(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []*Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := Get(e.Name())
		if err != nil {
			continue // unreadable session dirs never break list
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Meta.CreatedAt.Before(out[j].Meta.CreatedAt)
	})
	return out, nil
}

// Get loads one session by exact slug.
func Get(slug string) (*Session, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(sessionsDir(root), slug)
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("session %q: %w", slug, err)
	}

	s := &Session{Dir: dir}
	if err := json.Unmarshal(data, &s.Meta); err != nil {
		return nil, fmt.Errorf("session %q: %w", slug, err)
	}
	if s.Meta.Roles == nil {
		s.Meta.Roles = map[string]string{"vm": "digitalocean"}
	}
	if s.Meta.SSHUser == "" && s.Meta.Schema < 2 {
		s.Meta.SSHUser = "root"
	}
	return s, nil
}

// Save writes metadata.json (0600).
func (s *Session) Save() error {
	data, err := json.MarshalIndent(s.Meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.MetadataPath(), append(data, '\n'), 0o600)
}

// SetPhase persists the phase before the step it names runs.
func (s *Session) SetPhase(p string) error { s.Meta.Phase = p; return s.Save() }

// SetStatus persists a status change.
func (s *Session) SetStatus(st string) error { s.Meta.Status = st; return s.Save() }

// SetIP persists the droplet address.
func (s *Session) SetIP(ip string) error { s.Meta.IP = ip; return s.Save() }

// SetFQDN persists the session's DNS name.
func (s *Session) SetFQDN(fqdn string) error { s.Meta.FQDN = fqdn; return s.Save() }

// SetURL persists the session's handover URL.
func (s *Session) SetURL(u string) error { s.Meta.URL = u; return s.Save() }

// Path helpers.
func (s *Session) MetadataPath() string   { return filepath.Join(s.Dir, "metadata.json") }
func (s *Session) SSHDir() string         { return filepath.Join(s.Dir, "ssh") }
func (s *Session) TerraformDir() string   { return filepath.Join(s.Dir, "terraform") }
func (s *Session) StackDir() string       { return filepath.Join(s.Dir, "docker") }
func (s *Session) LogsDir() string        { return filepath.Join(s.Dir, "logs") }
func (s *Session) KeyPath() string        { return filepath.Join(s.SSHDir(), "key") }
func (s *Session) PubKeyPath() string     { return filepath.Join(s.SSHDir(), "key.pub") }
func (s *Session) KnownHostsPath() string { return filepath.Join(s.SSHDir(), "known_hosts") }

// Lock takes a non-blocking exclusive flock on the session.
func (s *Session) Lock() (func(), error) {
	f, err := os.OpenFile(filepath.Join(s.Dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("session busy: %s locked by another interview process",
			s.Meta.Slug)
	}
	return func() {
		unix.Flock(int(f.Fd()), unix.LOCK_UN)
		f.Close()
	}, nil
}

// Archive moves metadata + logs to archive/<slug> and removes the session dir.
func (s *Session) Archive() error {
	root, err := Root()
	if err != nil {
		return err
	}
	dst := filepath.Join(archiveDir(root), s.Meta.Slug)
	for i := 2; exists(dst); i++ {
		dst = filepath.Join(archiveDir(root), fmt.Sprintf("%s-%d", s.Meta.Slug, i))
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}

	if err := os.Rename(s.MetadataPath(), filepath.Join(dst, "metadata.json")); err != nil {
		return err
	}
	if exists(s.LogsDir()) {
		if err := os.Rename(s.LogsDir(), filepath.Join(dst, "logs")); err != nil {
			return err
		}
	}
	return os.RemoveAll(s.Dir)
}

// Age is the time since creation.
func (s *Session) Age(now time.Time) time.Duration { return now.Sub(s.Meta.CreatedAt) }
