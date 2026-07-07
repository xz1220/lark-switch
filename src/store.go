package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Account is one lark-cli identity, pinned to its own config home directory.
type Account struct {
	Name  string `json:"name"`
	Dir   string `json:"dir"`
	Brand string `json:"brand,omitempty"`
	Note  string `json:"note,omitempty"`
}

// Store is the on-disk registry of accounts (~/.config/lark-switch/config.json).
type Store struct {
	Accounts []Account `json:"accounts"`
	path     string
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return os.Getenv("HOME")
}

// defaultHome is lark-cli's built-in config home when LARKSUITE_CLI_CONFIG_DIR is unset.
func defaultHome() string { return filepath.Join(homeDir(), ".lark-cli") }

// isDefaultHome reports whether dir is lark-cli's default home. Accounts on the
// default home must NEVER have LARKSUITE_CLI_CONFIG_DIR set: their encrypted tokens
// live under ~/Library/Application Support/lark-cli, not under ~/.lark-cli, so
// pinning the var would make lark-cli look in the wrong place and appear logged out.
func isDefaultHome(dir string) bool {
	a, err1 := filepath.Abs(expandHome(dir))
	b, err2 := filepath.Abs(defaultHome())
	if err1 != nil || err2 != nil {
		return expandHome(dir) == defaultHome()
	}
	return a == b
}

func expandHome(p string) string {
	if p == "~" {
		return homeDir()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir(), p[2:])
	}
	return p
}

func storePath() string {
	if x := os.Getenv("LARK_SWITCH_CONFIG"); x != "" {
		return expandHome(x)
	}
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		cfg = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(cfg, "lark-switch", "config.json")
}

func loadStore() (*Store, error) {
	p := storePath()
	s := &Store{path: p}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	s.path = p
	return s, nil
}

// save writes the registry atomically: a temp file in the same directory is
// written, fsync'd, and renamed over the target, so a crash/ENOSPC can never
// leave a truncated or empty config.json. (Concurrent add/rm are still
// last-writer-wins; the registry is small and rebuildable, so no lock is taken.)
func (s *Store) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(dir, ".config-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func (s *Store) find(name string) *Account {
	for i := range s.Accounts {
		if s.Accounts[i].Name == name {
			return &s.Accounts[i]
		}
	}
	return nil
}

func (s *Store) findByDir(dir string) *Account {
	ad, err := filepath.Abs(expandHome(dir))
	if err != nil {
		ad = expandHome(dir)
	}
	for i := range s.Accounts {
		bd, err := filepath.Abs(expandHome(s.Accounts[i].Dir))
		if err != nil {
			bd = expandHome(s.Accounts[i].Dir)
		}
		if ad == bd {
			return &s.Accounts[i]
		}
	}
	return nil
}

func (s *Store) remove(name string) bool {
	for i := range s.Accounts {
		if s.Accounts[i].Name == name {
			s.Accounts = append(s.Accounts[:i], s.Accounts[i+1:]...)
			return true
		}
	}
	return false
}

// envForAccount returns a full environment for invoking lark-cli as this account:
// LARKSUITE_CLI_CONFIG_DIR is stripped for the default-home account and set for all
// others. Any inherited value is always removed first so a parent shell pinned to
// one account cannot leak into a sub-invocation for another.
func envForAccount(a *Account) []string {
	const key = "LARKSUITE_CLI_CONFIG_DIR="
	src := os.Environ()
	out := make([]string, 0, len(src)+1)
	for _, e := range src {
		if strings.HasPrefix(e, key) {
			continue
		}
		out = append(out, e)
	}
	if !isDefaultHome(a.Dir) {
		out = append(out, key+expandHome(a.Dir))
	}
	return out
}

// currentName resolves the account active in the current process environment:
// an explicit LARK_SWITCH_CURRENT wins, else the account whose dir matches
// LARKSUITE_CLI_CONFIG_DIR (or the default home when that is unset).
func currentName(s *Store) string {
	if n := os.Getenv("LARK_SWITCH_CURRENT"); n != "" {
		if s.find(n) != nil {
			return n
		}
	}
	d := os.Getenv("LARKSUITE_CLI_CONFIG_DIR")
	if d == "" {
		d = defaultHome()
	}
	if a := s.findByDir(d); a != nil {
		return a.Name
	}
	return ""
}
