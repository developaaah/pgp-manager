package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Clipboard key detection modes.
const (
	ClipKeysOff     = "off"     // ignore this key type entirely
	ClipKeysUnknown = "unknown" // only keys not yet in the keystore
	ClipKeysAll     = "all"     // every detected key
)

// Config holds the application configuration.
type Config struct {
	KeysDir                   string   `toml:"keys_dir"`
	PassphraseCacheTTLMinutes int      `toml:"passphrase_cache_ttl_minutes"`
	Theme                     string   `toml:"theme"`
	CustomKeyservers          []string `toml:"custom_keyservers"`

	// StartInTray starts the app hidden in the system tray instead of
	// opening the main window.
	StartInTray bool `toml:"start_in_tray"`

	// Clipboard auto-detection. Messages/Signatures are on/off; keys use
	// ClipKeysOff/ClipKeysUnknown/ClipKeysAll.
	ClipDetectMessages    bool   `toml:"clip_detect_messages"`
	ClipDetectPublicKeys  string `toml:"clip_detect_public_keys"`
	ClipDetectPrivateKeys string `toml:"clip_detect_private_keys"`
	ClipDetectSignatures  bool   `toml:"clip_detect_signatures"`

	// AutoCopyResults copies the result of Encrypt/Sign text operations to
	// the clipboard automatically.
	AutoCopyResults bool `toml:"auto_copy_results"`

	// Deprecated fields — decoded for migration only, never written back.
	Keyservers   []string `toml:"keyservers"`
	KeyserverURL string   `toml:"keyserver_url"`

	configPath string
}

// builtinKeyserverURLs lists URLs that are always available as built-ins.
// Entries found in old config fields matching these are dropped during migration.
var builtinKeyserverURLs = []string{
	"https://keys.openpgp.org",
	"https://keyserver.ubuntu.com",
	"https://pgp.mit.edu",
	"https://keys.mailvelope.com",
	"https://pgp.circl.lu",
}

func isBuiltin(u string) bool {
	for _, b := range builtinKeyserverURLs {
		if u == b {
			return true
		}
	}
	return false
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		KeysDir:                   "",
		PassphraseCacheTTLMinutes: 15,
		Theme:                     "dark",
		CustomKeyservers:          []string{},
		StartInTray:               true,
		ClipDetectMessages:        true,
		ClipDetectPublicKeys:      ClipKeysUnknown,
		ClipDetectPrivateKeys:     ClipKeysUnknown,
		ClipDetectSignatures:      true,
		AutoCopyResults:           true,
	}
}

// ConfigDir returns the default directory for config and keys: ~/.pgp on
// every platform. Config and keys live in the same directory so the whole
// folder can be moved/used portably.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pgp"), nil
}

// DefaultConfigPath returns the full path of the config file in the OS
// config directory.
func DefaultConfigPath() (string, error) {
	cfgDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "config.toml"), nil
}

// Load reads the config file from configPath.
// If configPath is empty, the OS config directory is used.
// If the file does not exist, defaults (with configPath set, so Save() works)
// are returned together with the underlying not-exist error — the caller
// decides whether and where to create the file (first-run setup).
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath == "" {
		var err error
		configPath, err = DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	var raw struct {
		KeysDir                   string   `toml:"keys_dir"`
		PassphraseCacheTTLMinutes int      `toml:"passphrase_cache_ttl_minutes"`
		Theme                     string   `toml:"theme"`
		CustomKeyservers          []string `toml:"custom_keyservers"`
		StartInTray               *bool    `toml:"start_in_tray"`
		ClipDetectMessages        *bool    `toml:"clip_detect_messages"`
		ClipDetectPublicKeys      string   `toml:"clip_detect_public_keys"`
		ClipDetectPrivateKeys     string   `toml:"clip_detect_private_keys"`
		ClipDetectSignatures      *bool    `toml:"clip_detect_signatures"`
		AutoCopyResults           *bool    `toml:"auto_copy_results"`
		Keyservers                []string `toml:"keyservers"`
		KeyserverURL              string   `toml:"keyserver_url"`
	}

	if _, err := toml.DecodeFile(configPath, &raw); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		cfg.configPath = configPath
		return cfg, err
	}

	cfg.KeysDir = raw.KeysDir
	cfg.PassphraseCacheTTLMinutes = raw.PassphraseCacheTTLMinutes
	cfg.Theme = raw.Theme
	cfg.CustomKeyservers = raw.CustomKeyservers
	cfg.configPath = configPath

	// Booleans absent from older configs keep their (true) defaults.
	if raw.StartInTray != nil {
		cfg.StartInTray = *raw.StartInTray
	}
	if raw.ClipDetectMessages != nil {
		cfg.ClipDetectMessages = *raw.ClipDetectMessages
	}
	if raw.ClipDetectSignatures != nil {
		cfg.ClipDetectSignatures = *raw.ClipDetectSignatures
	}
	if raw.AutoCopyResults != nil {
		cfg.AutoCopyResults = *raw.AutoCopyResults
	}
	if m := normalizeClipKeysMode(raw.ClipDetectPublicKeys); m != "" {
		cfg.ClipDetectPublicKeys = m
	}
	if m := normalizeClipKeysMode(raw.ClipDetectPrivateKeys); m != "" {
		cfg.ClipDetectPrivateKeys = m
	}

	// Apply defaults for zero-value fields.
	if cfg.PassphraseCacheTTLMinutes == 0 {
		cfg.PassphraseCacheTTLMinutes = DefaultConfig().PassphraseCacheTTLMinutes
	}
	if cfg.Theme == "" {
		cfg.Theme = DefaultConfig().Theme
	}
	if cfg.CustomKeyservers == nil {
		cfg.CustomKeyservers = []string{}
	}

	// Migration: old Keyservers/KeyserverURL → CustomKeyservers (non-builtins only).
	if len(cfg.CustomKeyservers) == 0 {
		candidates := raw.Keyservers
		if len(candidates) == 0 && raw.KeyserverURL != "" {
			candidates = []string{raw.KeyserverURL}
		}
		for _, u := range candidates {
			if !isBuiltin(u) {
				cfg.CustomKeyservers = append(cfg.CustomKeyservers, u)
			}
		}
	}

	return cfg, nil
}

// normalizeClipKeysMode validates a key-detection mode; returns "" for
// unknown values so the caller keeps the default.
func normalizeClipKeysMode(m string) string {
	switch m {
	case ClipKeysOff, ClipKeysUnknown, ClipKeysAll:
		return m
	default:
		return ""
	}
}

// Save writes the config to the file set by Load() or SetConfigPath().
func (c *Config) Save() error {
	cfgPath := c.configPath
	if cfgPath == "" {
		cfgDir, err := ConfigDir()
		if err != nil {
			return err
		}
		cfgPath = filepath.Join(cfgDir, "config.toml")
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(cfgPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write only current fields — deprecated fields are never written back.
	type saveCfg struct {
		KeysDir                   string   `toml:"keys_dir"`
		PassphraseCacheTTLMinutes int      `toml:"passphrase_cache_ttl_minutes"`
		Theme                     string   `toml:"theme"`
		CustomKeyservers          []string `toml:"custom_keyservers"`
		StartInTray               bool     `toml:"start_in_tray"`
		ClipDetectMessages        bool     `toml:"clip_detect_messages"`
		ClipDetectPublicKeys      string   `toml:"clip_detect_public_keys"`
		ClipDetectPrivateKeys     string   `toml:"clip_detect_private_keys"`
		ClipDetectSignatures      bool     `toml:"clip_detect_signatures"`
		AutoCopyResults           bool     `toml:"auto_copy_results"`
	}
	return toml.NewEncoder(f).Encode(saveCfg{
		KeysDir:                   c.KeysDir,
		PassphraseCacheTTLMinutes: c.PassphraseCacheTTLMinutes,
		Theme:                     c.Theme,
		CustomKeyservers:          c.CustomKeyservers,
		StartInTray:               c.StartInTray,
		ClipDetectMessages:        c.ClipDetectMessages,
		ClipDetectPublicKeys:      c.ClipDetectPublicKeys,
		ClipDetectPrivateKeys:     c.ClipDetectPrivateKeys,
		ClipDetectSignatures:      c.ClipDetectSignatures,
		AutoCopyResults:           c.AutoCopyResults,
	})
}

// SetConfigPath sets the path where Save() will write the config.
func (c *Config) SetConfigPath(path string) {
	c.configPath = path
}

// Dir returns the directory containing the config file. Keys are stored in
// this directory by default, which keeps config + keys together (portable).
func (c *Config) Dir() (string, error) {
	if c.configPath != "" {
		return filepath.Dir(c.configPath), nil
	}
	return ConfigDir()
}

// RuntimeOS returns the current OS name.
func RuntimeOS() string {
	return runtime.GOOS
}
