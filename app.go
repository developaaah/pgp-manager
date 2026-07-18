package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/ProtonMail/gopenpgp/v3/profile"
	"github.com/developaaah/pgp-manager/backend/autostart"
	"github.com/developaaah/pgp-manager/backend/cache"
	"github.com/developaaah/pgp-manager/backend/clipboard"
	"github.com/developaaah/pgp-manager/backend/config"
	bcrypto "github.com/developaaah/pgp-manager/backend/crypto"
	"github.com/developaaah/pgp-manager/backend/install"
	"github.com/developaaah/pgp-manager/backend/keystore"
	"github.com/developaaah/pgp-manager/backend/keyserver"
	"github.com/developaaah/pgp-manager/backend/model"
	"github.com/developaaah/pgp-manager/backend/notification"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct und seine Funktionen
type App struct {
	ctx       context.Context
	cancel    context.CancelFunc
	cfg       *config.Config
	store     *keystore.Store
	cache     *cache.PassphraseCache
	pgpHandle *crypto.PGPHandle
	notifier  notification.Notifier

	// needsSetup is true when no config exists at the default location —
	// the frontend shows the first-run setup modal and the app stays
	// inert (no store, no clipboard monitor) until ConfirmSetup.
	needsSetup bool

	// quitting marks an explicit quit (tray menu, window close without
	// tray mode) so beforeClose lets the app terminate.
	quitting bool

	// clipSuppress holds clipboard content set by the app itself so the
	// clipboard monitor does not re-offer actions on our own output.
	clipMu       sync.Mutex
	clipSuppress string

	// availableUpdate is set once by checkForUpdate() when a newer release
	// exists on GitHub. Empty string means up-to-date or check not done yet.
	availableUpdate string
}

// NewApp creates a new App with config, store, and cache initialized.
// When no config file exists yet, store initialization is deferred until
// the user confirms a config directory (ConfirmSetup).
func NewApp() *App {
	cfg, err := config.Load("")
	needsSetup := false
	switch {
	case err == nil:
		// loaded
	case cfg != nil && errors.Is(err, fs.ErrNotExist):
		needsSetup = true
	default:
		slog.Warn("config load failed, using defaults", "error", err)
		cfg = config.DefaultConfig()
	}

	ttl := time.Duration(cfg.PassphraseCacheTTLMinutes) * time.Minute
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		ctx:        ctx,
		cancel:     cancel,
		cfg:        cfg,
		needsSetup: needsSetup,
		cache:      cache.NewPassphraseCache(ctx, ttl),
		pgpHandle:  crypto.PGPWithProfile(profile.RFC4880()),
	}

	if !needsSetup {
		store, err := keystore.New(cfg)
		if err != nil {
			panic(err)
		}
		app.store = store
	}

	app.notifier = notification.New()

	return app
}

// ── First-Run Setup ───────────────────────────────────────────────────────────

// NeedsSetup reports whether the first-run setup modal must be shown
// (no config file at the default location).
//
//bind
func (a *App) NeedsSetup() bool {
	return a.needsSetup
}

// DefaultConfigDir returns the OS default config directory shown in the
// setup modal.
//
//bind
func (a *App) DefaultConfigDir() (string, error) {
	return config.ConfigDir()
}

// ConfirmSetup finalizes the first run. dir is the directory where config
// (and, for non-default directories, keys) will live. The default directory
// persists the choice permanently; any other directory makes the app run
// "standalone" — nothing is written to the default location, so the setup
// modal asks again on every start.
//
//bind
func (a *App) ConfirmSetup(dir string) error {
	if !a.needsSetup {
		return nil
	}
	defDir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == defDir {
		// Default location: persist the current (default) config there.
		path, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		a.cfg.SetConfigPath(path)
		if err := a.cfg.Save(); err != nil {
			return err
		}
	} else {
		// Standalone: config + keys live together in the chosen directory
		// (KeysDir stays empty — keys resolve relative to the config file,
		// so the directory can be moved freely).
		cfgPath := filepath.Join(dir, "config.toml")
		cfg, err := config.Load(cfgPath)
		switch {
		case err == nil:
			// existing standalone config
		case cfg != nil && errors.Is(err, fs.ErrNotExist):
			if err := cfg.Save(); err != nil {
				return err
			}
		default:
			return err
		}
		a.cfg = cfg
	}

	store, err := keystore.New(a.cfg)
	if err != nil {
		return err
	}
	a.store = store

	ttl := time.Duration(a.cfg.PassphraseCacheTTLMinutes) * time.Minute
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	a.cache = cache.NewPassphraseCache(a.ctx, ttl)

	a.needsSetup = false
	go a.watchClipboard(a.ctx)
	runtime.EventsEmit(a.ctx, "refresh-keys")
	return nil
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
		if len(paths) > 0 {
			runtime.EventsEmit(ctx, "file-drop", paths)
		}
	})
	// Ctrl+C / SIGTERM must terminate the app even in tray mode — Wails'
	// own handler only closes the window, which beforeClose would turn
	// into a hide.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigc:
			a.quit()
		case <-ctx.Done():
		}
	}()
	go func() { _ = a.notifier.RequestPermission(ctx) }()
	go a.checkForUpdate()
	registerServices(a)
	if !a.needsSetup {
		go a.watchClipboard(ctx)
	}
}

// shutdown is called when the app exits.
func (a *App) shutdown(ctx context.Context) {
	a.cancel()
	a.cache.Clear()
}

// beforeClose decides what closing the window means: in tray mode the
// window just hides and the app keeps running; otherwise the app quits.
// Explicit quits (tray menu) always pass through.
func (a *App) beforeClose(ctx context.Context) bool {
	if a.quitting {
		return false
	}
	if a.cfg.StartInTray {
		runtime.WindowHide(ctx)
		return true // prevent termination
	}
	return false
}

// quit terminates the app, bypassing the hide-to-tray close behavior.
func (a *App) quit() {
	a.quitting = true
	runtime.Quit(a.ctx)
}

// ── Keys ─────────────────────────────────────────────────────────────────────

// ListKeys returns all keys in the store.
//
//bind
func (a *App) ListKeys() []model.KeyInfo {
	keys, err := a.store.List()
	if err != nil {
		slog.Error("list keys failed", "error", err)
		return nil
	}
	result := make([]model.KeyInfo, len(keys))
	for i, k := range keys {
		result[i] = *k
	}
	return result
}

// ImportKey imports all keys from an armored block (supports multi-key blocks).
//
//bind
func (a *App) ImportKey(armored string) error {
	_, err := a.store.ImportAll(armored)
	if err == nil || errors.Is(err, keystore.ErrAlreadyExists) {
		runtime.EventsEmit(a.ctx, "refresh-keys")
	}
	return err
}

// ImportMultipleKeys imports multiple keys from an armored block and returns per-key results.
//
//bind
func (a *App) ImportMultipleKeys(armored string) model.MultiImportResult {
	entries := a.store.ImportMultiple(armored)
	for _, e := range entries {
		if e.Error == "" {
			runtime.EventsEmit(a.ctx, "refresh-keys")
			break
		}
	}
	return model.MultiImportResult{Entries: entries}
}

// DeleteKey removes a key from the store by fingerprint.
//
//bind
func (a *App) DeleteKey(fingerprint string) error {
	return a.store.Delete(fingerprint)
}

// ExportKey returns the armored PGP key for the given fingerprint.
//
//bind
func (a *App) ExportKey(fingerprint string) (string, error) {
	return a.store.GetArmored(fingerprint)
}

// GetPublicKey returns the armored public key for the given fingerprint.
// If the stored key is a private key, only the public portion is returned.
//
//bind
func (a *App) GetPublicKey(fingerprint string) (string, error) {
	armored, err := a.store.GetArmored(fingerprint)
	if err != nil {
		return "", err
	}
	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		return "", err
	}
	if !key.IsPrivate() {
		return armored, nil
	}
	return key.GetArmoredPublicKey()
}

// ImportPrivateKey validates the passphrase against a private key and imports it.
// If the key has no passphrase, pass an empty string.
//
//bind
func (a *App) ImportPrivateKey(armored, passphrase string) error {
	armored = keystore.SanitizeArmored(armored)
	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}
	if !key.IsPrivate() {
		return fmt.Errorf("not a private key — use Import Key for public keys")
	}
	locked, err := key.IsLocked()
	if err != nil {
		return fmt.Errorf("could not inspect key: %w", err)
	}
	if locked {
		if passphrase == "" {
			return fmt.Errorf("this key requires a passphrase")
		}
		if _, err := key.Unlock([]byte(passphrase)); err != nil {
			return fmt.Errorf("wrong passphrase")
		}
	}
	fp, importErr := a.store.Import(armored)
	if importErr != nil && !errors.Is(importErr, keystore.ErrAlreadyExists) {
		return importErr
	}
	if passphrase != "" {
		a.cache.Set(fp, passphrase)
	}
	runtime.EventsEmit(a.ctx, "refresh-keys")
	return importErr // nil or ErrAlreadyExists — both surface to frontend
}

// ExportKeyToFile opens a save-file dialog and writes the key to the chosen path.
// For private keys this exports the full private key; for public keys the public key.
//
//bind
func (a *App) ExportKeyToFile(fingerprint string) error {
	armored, err := a.store.GetArmored(fingerprint)
	if err != nil {
		return err
	}
	suffix := fingerprint
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Private Key",
		DefaultFilename: "pgp_key_" + suffix + ".asc",
		Filters: []runtime.FileFilter{
			{DisplayName: "PGP Key Files (*.asc;*.pgp)", Pattern: "*.asc;*.pgp"},
			{DisplayName: "All Files (*)", Pattern: "*"},
		},
	})
	if err != nil {
		return err
	}
	if savePath == "" {
		return nil // cancelled
	}
	return os.WriteFile(savePath, []byte(armored), 0600)
}

// ReadClipboard reads the current clipboard text via native OS API.
// Avoids browser-level permission prompts on macOS.
//
//bind
func (a *App) ReadClipboard() (string, error) {
	return runtime.ClipboardGetText(a.ctx)
}

// SetClipboardText writes text to the clipboard without re-triggering the
// clipboard auto-detect on our own output (used for auto-copy of results).
//
//bind
func (a *App) SetClipboardText(text string) {
	a.setClipboardSilent(text)
}

// AppVersion returns the application version (set via ldflags on release builds).
//
//bind
func (a *App) AppVersion() string {
	return appVersion
}

// GetAvailableUpdate returns the latest release tag if a newer version is
// available on GitHub, or an empty string when the app is up to date.
// The result is populated asynchronously after startup; the frontend should
// also listen for the "update:available" event.
//
//bind
func (a *App) GetAvailableUpdate() string {
	return a.availableUpdate
}

// checkForUpdate fetches the latest GitHub release and emits "update:available"
// if a newer version exists. Runs in a goroutine during startup.
func (a *App) checkForUpdate() {
	if appVersion == "dev" || !strings.HasPrefix(appVersion, "v") {
		return
	}
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet,
		"https://api.github.com/repos/developaaah/pgp-manager/releases/latest", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pgp-manager/"+appVersion)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var data struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}
	if semverLess(appVersion, data.TagName) {
		a.availableUpdate = data.TagName
		runtime.EventsEmit(a.ctx, "update:available", data.TagName)
	}
}

// semverLess reports whether version a is strictly less than b.
// Both must be in "vX.Y.Z" format; pre-release suffixes after "-" are ignored.
func semverLess(a, b string) bool {
	parse := func(v string) [3]int {
		v = strings.TrimPrefix(v, "v")
		parts := strings.SplitN(v, ".", 3)
		var nums [3]int
		for i, p := range parts {
			if i >= 3 {
				break
			}
			nums[i], _ = strconv.Atoi(strings.SplitN(p, "-", 2)[0])
		}
		return nums
	}
	pa, pb := parse(a), parse(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

// OpenExternal opens a URL in the default browser.
//
//bind
func (a *App) OpenExternal(rawURL string) {
	if !strings.HasPrefix(rawURL, "https://") {
		return
	}
	runtime.BrowserOpenURL(a.ctx, rawURL)
}

// GenerateKey generates a new PGP key pair and imports it into the store.
// keyType: "rsa2048", "rsa3072" (default), "rsa4096", "ed25519".
// expiryYears: "0" = no expiry, "1".."n" = expires in n years.
//
//bind
func (a *App) GenerateKey(name, email, passphrase, keyType, expiryYears string) error {
	if err := bcrypto.GenerateKey(a.store, a.pgpHandle, name, email, passphrase, keyType, expiryYears); err != nil {
		slog.Error("generate key failed", "name", name, "email", email, "error", err)
		return err
	}
	runtime.EventsEmit(a.ctx, "refresh-keys")
	return nil
}

// ── Passphrase Cache ──────────────────────────────────────────────────────────

// CachePassphrase stores a passphrase in the cache for the given fingerprint.
//
//bind
func (a *App) CachePassphrase(fingerprint, passphrase string) {
	a.cache.Set(fingerprint, passphrase)
}

// GetCachedPassphrase returns the cached passphrase for a fingerprint, or "" if not cached.
//
//bind
func (a *App) GetCachedPassphrase(fingerprint string) string {
	return a.cache.Get(fingerprint)
}

// HasCachedPassphrase returns true if a passphrase for this fingerprint is currently cached.
//
//bind
func (a *App) HasCachedPassphrase(fingerprint string) bool {
	return a.cache.Has(fingerprint)
}

// PreviewKeys parses an armored PGP block (one or more keys) and returns metadata.
// AlreadyExists is set when the key is already in the keystore.
//
//bind
func (a *App) PreviewKeys(armored string) []model.KeyInfo {
	result := a.store.Preview(armored)
	if result == nil {
		return []model.KeyInfo{}
	}
	return result
}

// OpenFolder opens the folder containing filePath in the system file manager.
//
//bind
func (a *App) OpenFolder(filePath string) error {
	dir := filepath.Dir(filePath)
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}

// ── Text Crypto ───────────────────────────────────────────────────────────────

// EncryptText encrypts plaintext for the given recipient fingerprints.
// If signingFingerprint and signingPassphrase are non-empty, the message is also signed.
//
//bind
func (a *App) EncryptText(plaintext string, recipientFingerprints []string, signingFingerprint, signingPassphrase string) model.EncryptResult {
	return bcrypto.EncryptText(a.store, plaintext, recipientFingerprints, signingFingerprint, signingPassphrase)
}

// DecryptText decrypts an armored message using the private key identified by fingerprint.
//
//bind
func (a *App) DecryptText(armored, fingerprint, passphrase string, signed bool) model.DecryptResult {
	return bcrypto.DecryptText(a.store, armored, fingerprint, passphrase, signed)
}

// FindDecryptionKey inspects PKESK headers in an armored PGP message and returns the
// fingerprint of a stored private key that matches, or "" if none can be identified.
//
//bind
func (a *App) FindDecryptionKey(armored string) string {
	return bcrypto.FindDecryptionKey(a.store, armored)
}

// FindDecryptionKeyFromFile reads a PGP file from disk (armored or binary) and returns
// the fingerprint of the matching stored private key, or "" if none found.
//
//bind
func (a *App) FindDecryptionKeyFromFile(filePath string) string {
	return bcrypto.FindDecryptionKeyFromFile(a.store, filePath)
}

// EncryptFile encrypts a file for the given recipients and writes output to inputPath+".pgp".
//
//bind
func (a *App) EncryptFile(inputPath string, recipientFingerprints []string, signingFingerprint string) model.FileResult {
	return bcrypto.EncryptFile(a.store, inputPath, recipientFingerprints, signingFingerprint)
}

// DecryptFile decrypts a .pgp/.asc/.gpg file using the given private key + passphrase.
//
//bind
func (a *App) DecryptFile(inputPath, fingerprint, passphrase string) model.FileResult {
	return bcrypto.DecryptFile(a.store, inputPath, fingerprint, passphrase)
}

// OpenFileDialog shows a native file-picker and returns the chosen file path.
// Returns an empty string if the user cancelled.
//
//bind
func (a *App) OpenFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a file",
	})
}

// SignText signs plaintext with the private key identified by fingerprint.
//
//bind
func (a *App) SignText(plaintext, fingerprint, passphrase string) model.SignResult {
	return bcrypto.SignText(a.store, plaintext, fingerprint, passphrase)
}

// VerifyText verifies a PGP-signed message against keys in the keystore.
//
//bind
func (a *App) VerifyText(signed string) model.VerifyResult {
	return bcrypto.VerifyText(a.store, signed)
}

// ── Keyserver ─────────────────────────────────────────────────────────────────

// builtinKeyservers lists the well-known keyservers that are always available.
var builtinKeyservers = []model.KeyserverEntry{
	{Label: "keys.openpgp.org", URL: "https://keys.openpgp.org", BuiltIn: true},
	{Label: "keyserver.ubuntu.com", URL: "https://keyserver.ubuntu.com", BuiltIn: true},
	{Label: "pgp.mit.edu", URL: "https://pgp.mit.edu", BuiltIn: true},
	{Label: "keys.mailvelope.com", URL: "https://keys.mailvelope.com", BuiltIn: true},
	{Label: "pgp.circl.lu", URL: "https://pgp.circl.lu", BuiltIn: true},
}

// ListKeyservers returns all available keyservers (built-ins + user-defined custom ones).
//
//bind
func (a *App) ListKeyservers() []model.KeyserverEntry {
	result := make([]model.KeyserverEntry, len(builtinKeyservers))
	copy(result, builtinKeyservers)
	for _, u := range a.cfg.CustomKeyservers {
		result = append(result, model.KeyserverEntry{Label: u, URL: u, BuiltIn: false})
	}
	return result
}

// AddCustomKeyserver adds a user-defined keyserver URL and persists the config.
//
//bind
func (a *App) AddCustomKeyserver(rawURL string) error {
	rawURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	if _, err := url.Parse(rawURL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	for _, u := range a.cfg.CustomKeyservers {
		if u == rawURL {
			return fmt.Errorf("keyserver already in list")
		}
	}
	// Don't let built-ins be added as custom.
	for _, b := range builtinKeyservers {
		if b.URL == rawURL {
			return fmt.Errorf("already a built-in keyserver")
		}
	}
	a.cfg.CustomKeyservers = append(a.cfg.CustomKeyservers, rawURL)
	return a.cfg.Save()
}

// RemoveCustomKeyserver removes a user-defined keyserver by URL and persists the config.
//
//bind
func (a *App) RemoveCustomKeyserver(rawURL string) error {
	var kept []string
	for _, u := range a.cfg.CustomKeyservers {
		if u != rawURL {
			kept = append(kept, u)
		}
	}
	a.cfg.CustomKeyservers = kept
	return a.cfg.Save()
}

// StartKeyserverSearch starts an async keyserver search. Returns immediately.
// Results are delivered via "keyserver:results" ([]model.KeyserverResult) or
// "keyserver:error" (string) events.
// Pass serverURL="" for auto mode (all servers in parallel).
//
//bind
func (a *App) StartKeyserverSearch(query, serverURL string) {
	go func() {
		if serverURL == "" {
			results, _ := keyserver.SearchAll(a.ctx, a.allKeyserverURLs(), query)
			if results == nil {
				results = []model.KeyserverResult{}
			}
			runtime.EventsEmit(a.ctx, "keyserver:results", results)
			return
		}
		results, err := keyserver.SearchKeyserver(a.ctx, serverURL, query)
		if err != nil {
			runtime.EventsEmit(a.ctx, "keyserver:error", keyserverErrMsg(err))
			return
		}
		if results == nil {
			results = []model.KeyserverResult{}
		}
		runtime.EventsEmit(a.ctx, "keyserver:results", results)
	}()
}

// ImportFromKeyserver fetches a key by fingerprint or email and imports it.
// Pass serverURL="" to try all keyservers in sequence until one succeeds.
//
//bind
func (a *App) ImportFromKeyserver(identifier, serverURL string) error {
	var servers []string
	if serverURL == "" {
		servers = a.allKeyserverURLs()
	} else {
		servers = []string{serverURL}
	}

	var lastErr error
	for _, srv := range servers {
		armored, err := keyserver.FetchKey(a.ctx, srv, identifier)
		if err != nil {
			lastErr = err
			continue
		}
		if _, err := a.store.ImportAll(armored); err != nil && !errors.Is(err, keystore.ErrAlreadyExists) {
			lastErr = err
			continue
		}
		runtime.EventsEmit(a.ctx, "refresh-keys")
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("key not found: %s", keyserverErrMsg(lastErr))
	}
	return fmt.Errorf("key not found on any keyserver")
}

// PublishToKeyserver exports the given key and publishes it to the specified keyserver.
// A specific serverURL is required (auto mode is not supported for publishing).
//
//bind
func (a *App) PublishToKeyserver(fingerprint, serverURL string) error {
	if serverURL == "" {
		return fmt.Errorf("please select a specific keyserver to publish to")
	}
	armored, err := a.store.GetArmored(fingerprint)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}
	if err := keyserver.PublishKey(a.ctx, serverURL, armored); err != nil {
		return fmt.Errorf("keyserver publish failed: %w", err)
	}
	return nil
}

// keyserverErr wraps a raw network/HTTP error into a user-friendly message.
func keyserverErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", keyserverErrMsg(err))
}

func keyserverErrMsg(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "deadline exceeded") || strings.Contains(s, "Client.Timeout"):
		return "server did not respond in time"
	case strings.Contains(s, "connection refused"):
		return "connection refused by server"
	case strings.Contains(s, "no such host"):
		return "server not found (DNS)"
	case strings.Contains(s, "status 404"):
		return "key not found on this server"
	case strings.Contains(s, "status 5"):
		return "server returned an error"
	default:
		return "could not reach server"
	}
}

// allKeyserverURLs returns the URLs of all available keyservers (built-ins + custom).
func (a *App) allKeyserverURLs() []string {
	urls := make([]string, 0, len(builtinKeyservers)+len(a.cfg.CustomKeyservers))
	for _, e := range builtinKeyservers {
		urls = append(urls, e.URL)
	}
	urls = append(urls, a.cfg.CustomKeyservers...)
	return urls
}

// AddSubkey adds a new signing or encryption subkey to an existing private key.
// subkeyType: "sign" | "encrypt". expiryYears: "0" = no expiry.
//
//bind
func (a *App) AddSubkey(primaryFingerprint, subkeyType, expiryYears, passphrase string) error {
	armored, err := a.store.GetArmored(primaryFingerprint)
	if err != nil {
		return fmt.Errorf("add subkey: %w", err)
	}

	expiry, err := strconv.Atoi(expiryYears)
	if err != nil {
		return fmt.Errorf("add subkey: invalid expiry: %w", err)
	}

	newArmored, err := bcrypto.AddSubkey(armored, subkeyType, passphrase, expiry)
	if err != nil {
		return fmt.Errorf("add subkey: %w", err)
	}

	if _, err := a.store.Overwrite(newArmored); err != nil {
		return fmt.Errorf("add subkey: %w", err)
	}

	runtime.EventsEmit(a.ctx, "refresh-keys")
	return nil
}

// RevokeSubkey revokes a subkey of an existing private key.
//
//bind
func (a *App) RevokeSubkey(primaryFingerprint, subkeyFingerprint, passphrase string) error {
	armored, err := a.store.GetArmored(primaryFingerprint)
	if err != nil {
		return fmt.Errorf("revoke subkey: %w", err)
	}

	newArmored, err := bcrypto.RevokeSubkey(armored, subkeyFingerprint, passphrase)
	if err != nil {
		return fmt.Errorf("revoke subkey: %w", err)
	}

	if _, err := a.store.Overwrite(newArmored); err != nil {
		return fmt.Errorf("revoke subkey: %w", err)
	}

	runtime.EventsEmit(a.ctx, "refresh-keys")
	return nil
}

// ── Config ────────────────────────────────────────────────────────────────────

// GetConfig returns the current user-visible configuration.
//
//bind
func (a *App) GetConfig() model.AppConfig {
	keysDir, _ := keystore.StorageDir(a.cfg)
	customKS := a.cfg.CustomKeyservers
	if customKS == nil {
		customKS = []string{}
	}
	return model.AppConfig{
		Theme:                     a.cfg.Theme,
		PassphraseCacheTTLMinutes: a.cfg.PassphraseCacheTTLMinutes,
		KeysDir:                   keysDir,
		CustomKeyservers:          customKS,
		StartInTray:               a.cfg.StartInTray,
		ClipDetectMessages:        a.cfg.ClipDetectMessages,
		ClipDetectPublicKeys:      a.cfg.ClipDetectPublicKeys,
		ClipDetectPrivateKeys:     a.cfg.ClipDetectPrivateKeys,
		ClipDetectSignatures:      a.cfg.ClipDetectSignatures,
		AutoCopyResults:           a.cfg.AutoCopyResults,
	}
}

// SaveConfig persists updated user-visible configuration.
//
//bind
func (a *App) SaveConfig(cfg model.AppConfig) error {
	a.cfg.Theme = cfg.Theme
	a.cfg.PassphraseCacheTTLMinutes = cfg.PassphraseCacheTTLMinutes
	if cfg.KeysDir != "" {
		a.cfg.KeysDir = cfg.KeysDir
	}
	a.cfg.CustomKeyservers = cfg.CustomKeyservers
	a.cfg.StartInTray = cfg.StartInTray
	a.cfg.ClipDetectMessages = cfg.ClipDetectMessages
	a.cfg.ClipDetectPublicKeys = cfg.ClipDetectPublicKeys
	a.cfg.ClipDetectPrivateKeys = cfg.ClipDetectPrivateKeys
	a.cfg.ClipDetectSignatures = cfg.ClipDetectSignatures
	a.cfg.AutoCopyResults = cfg.AutoCopyResults
	return a.cfg.Save()
}

// ── Autostart ─────────────────────────────────────────────────────────────────

// AutostartSupported reports whether launch-at-login can be managed on this
// platform (macOS needs 13+ and an app bundle).
//
//bind
func (a *App) AutostartSupported() bool {
	return autostart.Supported()
}

// GetAutostart reports whether the app is registered to launch at login.
//
//bind
func (a *App) GetAutostart() bool {
	enabled, err := autostart.Enabled()
	if err != nil {
		slog.Debug("autostart status failed", "error", err)
	}
	return enabled
}

// SetAutostart registers or unregisters the app as a login item.
//
//bind
func (a *App) SetAutostart(enable bool) error {
	return autostart.SetEnabled(enable)
}

// ── System-wide install ───────────────────────────────────────────────────────

// InstallSupported reports whether a system-wide install is possible on
// this platform (macOS: running from an app bundle, Linux: always).
//
//bind
func (a *App) InstallSupported() bool {
	return install.Supported()
}

// IsInstalled reports whether the app is already installed system-wide
// (macOS: runs from /Applications, Linux: desktop entry exists).
//
//bind
func (a *App) IsInstalled() bool {
	return install.Installed()
}

// InstallApp installs the app system-wide: on macOS the bundle is copied
// to /Applications, on Linux a desktop entry + icon are created.
//
//bind
func (a *App) InstallApp() error {
	return install.Install(appIcon)
}

// OpenDirectoryDialog opens a native directory picker and returns the chosen path.
// Returns an empty string if the user cancelled.
//
//bind
func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Keys Directory",
	})
}

// ── Window ────────────────────────────────────────────────────────────────────

// WindowClose closes the window: hides it in tray mode, quits otherwise.
//
//bind
func (a *App) WindowClose() {
	if a.cfg.StartInTray {
		runtime.WindowHide(a.ctx)
		return
	}
	a.quit()
}

// WindowMinimise minimises the window.
//
//bind
func (a *App) WindowMinimise() {
	runtime.WindowMinimise(a.ctx)
}

// WindowMaximise toggles maximised state.
//
//bind
func (a *App) WindowMaximise() {
	runtime.WindowToggleMaximise(a.ctx)
}

// ── Context Actions (IPC) ─────────────────────────────────────────────────────

// handleSecondInstance handles a second app launch (Windows/Linux context menu,
// URL protocol activation). The first instance receives the new instance's args.
func (a *App) handleSecondInstance(data options.SecondInstanceData) {
	a.showMainWindow()
	for _, arg := range data.Args {
		if strings.HasPrefix(arg, "pgp-manager://") {
			a.handleActionRequest(arg)
			return
		}
	}
}

// domReady fires once the frontend has loaded. On a fresh launch via context
// menu or URL protocol (Windows/Linux), the URL arrives as a CLI argument —
// handle it now that the frontend can receive events.
func (a *App) domReady(_ context.Context) {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "pgp-manager://") {
			a.handleActionRequest(arg)
			return
		}
	}
}

// decodeBase64Loose decodes base64 that may have travelled through a URL query:
// URL-safe alphabet, standard alphabet, with or without padding. A literal '+'
// is decoded to a space by Go's query parser, so repair that before trying the
// standard alphabet.
func decodeBase64Loose(enc string) ([]byte, error) {
	enc = strings.TrimSpace(enc)
	if b, err := base64.URLEncoding.DecodeString(enc); err == nil {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(enc); err == nil {
		return b, nil
	}
	std := strings.ReplaceAll(enc, " ", "+")
	if b, err := base64.StdEncoding.DecodeString(std); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(std)
}

// maxActionFileSize caps files read for text actions (armored PGP data is small).
const maxActionFileSize = 10 << 20 // 10 MiB

// textFromParams resolves the input text of a context action: either a
// base64-encoded "text" parameter or a "file" parameter pointing to a file
// whose content is used (Windows/Linux file context menus).
func textFromParams(params url.Values) (string, error) {
	if enc := params.Get("text"); enc != "" {
		b, err := decodeBase64Loose(enc)
		if err != nil {
			return "", fmt.Errorf("invalid text encoding: %w", err)
		}
		return string(b), nil
	}
	if file := params.Get("file"); file != "" {
		info, err := os.Stat(file)
		if err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		if info.Size() > maxActionFileSize {
			return "", fmt.Errorf("file too large for a text action (max 10 MB)")
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		return string(b), nil
	}
	return "", fmt.Errorf("no text or file provided")
}

// handleActionRequest parses a pgp-manager:// URL and dispatches to the appropriate handler.
func (a *App) handleActionRequest(rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		slog.Error("context action: invalid URL", "url", rawURL, "error", err)
		return
	}
	if u.Scheme != "pgp-manager" {
		return
	}
	action := u.Host
	params := u.Query()

	switch action {
	case "import-key":
		text, _ := textFromParams(params)
		a.importKeyAction(text)
	case "decrypt-clipboard":
		// The message to decrypt is in the clipboard (tray action).
		a.decryptToWindow("")
	case "decrypt-text", "encrypt-text", "sign-text", "verify-text":
		text, err := textFromParams(params)
		if err != nil {
			a.emitActionResult(model.ActionResult{Action: action, Error: err.Error()})
			return
		}
		a.runTextAction(action, text)
	default:
		slog.Warn("context action: unknown action", "action", action)
	}
}

// runTextAction executes a text-based context action (called from the URL
// dispatcher and directly from the macOS services bridge). The actions
// behave exactly like their tray counterparts: encrypt/decrypt/sign open
// the window with the result, verify reports via notification.
func (a *App) runTextAction(action, text string) {
	if a.store == nil { // first-run setup not completed yet
		a.showMainWindow()
		return
	}
	switch action {
	case "import-key":
		a.importKeyAction(text)
	case "encrypt-text":
		a.encryptTextRequest(text)
	case "decrypt-text":
		a.decryptTextAction(text, true)
	case "sign-text":
		a.signTextToWindow(text)
	case "verify-text":
		a.verifyTextAction(text)
	default:
		slog.Warn("text action: unknown action", "action", action)
	}
}

// runFileAction executes a file-based context action (macOS Finder services)
// for all files of the Finder selection. Encrypt opens the recipient picker
// with the whole selection; the other actions run per file and write their
// results next to the input file.
func (a *App) runFileAction(action string, paths []string) {
	if a.store == nil { // first-run setup not completed yet
		a.showMainWindow()
		return
	}
	if len(paths) == 0 {
		return
	}
	if action == "encrypt-file" {
		a.encryptFilesRequest(paths)
		return
	}
	for _, path := range paths {
		a.runSingleFileAction(action, path)
	}
}

// runSingleFileAction executes a headless file action on one file.
func (a *App) runSingleFileAction(action, path string) {
	switch action {
	case "decrypt-file":
		fp := bcrypto.FindDecryptionKeyFromFile(a.store, path)
		passphrase := ""
		if fp != "" {
			passphrase = a.cache.Get(fp)
		}
		r := bcrypto.DecryptFile(a.store, path, fp, passphrase)
		if r.Error != "" {
			a.emitActionResult(model.ActionResult{Action: "decrypt-file", Error: r.Error})
		}
	case "sign-file":
		fp := a.defaultKeyFingerprint()
		if fp == "" {
			a.emitActionResult(model.ActionResult{Action: "sign-file", Error: "No private key available — import a private key first"})
			return
		}
		r := bcrypto.SignFile(a.store, path, path+".asc", fp, a.cache.Get(fp))
		if r.Error != "" {
			a.emitActionResult(model.ActionResult{Action: "sign-file", Error: r.Error})
		}
	case "verify-file":
		r := bcrypto.VerifyFile(a.store, path)
		switch {
		case r.Error != "":
			a.notify("Verification failed", r.Error)
		case r.Valid:
			signer := r.UID
			if signer == "" {
				signer = r.SignedBy
			}
			a.notify("Valid signature", "Signed by "+signer)
		default:
			a.notify("Invalid signature", filepath.Base(path))
		}
	default:
		slog.Warn("file action: unknown action", "action", action)
	}
}

// handleTrayAction reacts to tray menu clicks.
func (a *App) handleTrayAction(action string) {
	switch action {
	case "open":
		a.showMainWindow()
	case "quit":
		a.quit()
		return
	}
	if a.store == nil { // first-run setup not completed yet
		a.showMainWindow()
		return
	}
	switch action {
	case "open", "quit":
		// handled above
	case "encrypt-clipboard":
		a.encryptTextFromClipboard()
	case "decrypt-clipboard":
		a.decryptToWindow("")
	case "sign-clipboard":
		a.signTextFromClipboard()
	case "verify-clipboard":
		a.verifyTextFromClipboard()
	case "import-clipboard":
		a.importKeyAction("")
	default:
		slog.Warn("tray: unknown action", "action", action)
	}
}

// encryptTextFromClipboard loads the clipboard text into the encrypt view
// and opens the recipient picker.
func (a *App) encryptTextFromClipboard() {
	text, _ := runtime.ClipboardGetText(a.ctx)
	a.encryptTextRequest(text)
}

// signTextFromClipboard signs the clipboard text and shows the result in the
// main window.
func (a *App) signTextFromClipboard() {
	text, _ := runtime.ClipboardGetText(a.ctx)
	a.signTextToWindow(text)
}

// verifyTextFromClipboard verifies the signed message in the clipboard and
// reports the result via system notification.
func (a *App) verifyTextFromClipboard() {
	text, _ := runtime.ClipboardGetText(a.ctx)
	a.verifyTextAction(text)
}

// notify shows a plain system notification (fire and forget).
func (a *App) notify(title, body string) {
	go func() {
		if err := a.notifier.ShowInfo(a.ctx, title, body); err != nil {
			slog.Debug("notification failed", "title", title, "error", err)
		}
	}()
}

// setClipboardSilent writes text to the clipboard without triggering the
// clipboard monitor on our own output.
func (a *App) setClipboardSilent(text string) {
	a.clipMu.Lock()
	a.clipSuppress = strings.TrimSpace(text)
	a.clipMu.Unlock()
	_ = runtime.ClipboardSetText(a.ctx, text)
}

func (a *App) emitActionResult(res model.ActionResult) {
	a.showMainWindow()
	runtime.EventsEmit(a.ctx, "action:result", res)
}

// defaultKeyFingerprint returns the user's primary key (the first private key
// in the store) — used as default recipient and default signing key.
func (a *App) defaultKeyFingerprint() string {
	keys, _ := a.store.List()
	for _, k := range keys {
		if k.IsPrivate {
			return k.Fingerprint
		}
	}
	return ""
}

// importKeyAction imports armored key material from a context action or the
// tray menu. Empty text falls back to the clipboard.
func (a *App) importKeyAction(text string) {
	if strings.TrimSpace(text) == "" {
		text, _ = runtime.ClipboardGetText(a.ctx)
	}
	if !strings.Contains(text, "BEGIN PGP PUBLIC KEY BLOCK") &&
		!strings.Contains(text, "BEGIN PGP PRIVATE KEY BLOCK") {
		a.emitActionResult(model.ActionResult{Action: "import-key", Error: "No PGP key block found in the selection or clipboard"})
		return
	}

	fps, err := a.store.ImportAll(text)
	if err != nil && !errors.Is(err, keystore.ErrAlreadyExists) {
		slog.Warn("import-key: import failed", "error", err)
		a.emitActionResult(model.ActionResult{Action: "import-key", Error: err.Error()})
		return
	}

	// Close the import modal (if open) and refresh the key list.
	fp := ""
	if len(fps) > 0 {
		fp = fps[0]
	}
	runtime.EventsEmit(a.ctx, "notification:imported", fp)
	runtime.EventsEmit(a.ctx, "refresh-keys")
	a.showMainWindow()
}

// decryptToWindow decrypts armored text (clipboard when empty) and presents
// the result in the UI. Used by notifications, the tray menu, and the
// "Decrypt in New Window" service.
func (a *App) decryptToWindow(armored string) {
	if strings.TrimSpace(armored) == "" {
		armored, _ = runtime.ClipboardGetText(a.ctx)
	}
	a.decryptTextAction(armored, true)
}

func (a *App) decryptTextAction(text string, window bool) {
	fp := bcrypto.FindDecryptionKey(a.store, text)
	passphrase := ""
	if fp != "" {
		passphrase = a.cache.Get(fp)
	}
	r := bcrypto.DecryptText(a.store, text, fp, passphrase, false)
	if window || r.Error != "" {
		a.emitActionResult(model.ActionResult{Action: "decrypt-text", Output: r.Plaintext, Error: r.Error})
		return
	}
	a.setClipboardSilent(r.Plaintext)
}

// encryptTextRequest loads text into the encrypt view and opens the
// recipient picker there — encryption happens in the UI once the user has
// chosen recipients (context action and tray behave identically).
func (a *App) encryptTextRequest(text string) {
	a.showMainWindow()
	runtime.EventsEmit(a.ctx, "encrypt-text-requested", text)
}

// encryptFilesRequest loads files into the encrypt view and opens the
// recipient picker there (the Finder service behaves like the text service).
func (a *App) encryptFilesRequest(paths []string) {
	a.showMainWindow()
	runtime.EventsEmit(a.ctx, "encrypt-file-requested", paths)
}

// signTextToWindow signs text and presents the result in the UI (tray action).
func (a *App) signTextToWindow(text string) {
	fp := a.defaultKeyFingerprint()
	if fp == "" {
		a.emitActionResult(model.ActionResult{Action: "sign-text", Error: "No private key available — import a private key first"})
		return
	}
	r := bcrypto.SignText(a.store, text, fp, a.cache.Get(fp))
	a.emitActionResult(model.ActionResult{Action: "sign-text", Output: r.Armored, Error: r.Error})
}

func (a *App) verifyTextAction(text string) {
	r := bcrypto.VerifyText(a.store, text)
	switch {
	case r.Error != "":
		a.notify("Verification failed", r.Error)
	case r.Valid:
		signer := r.UID
		if signer == "" {
			signer = r.SignedBy
		}
		if r.Email != "" && r.UID != "" {
			signer += " <" + r.Email + ">"
		}
		a.notify("Valid signature", "Signed by "+signer)
	default:
		a.notify("Invalid signature", "The signature could not be verified")
	}
}

// Clipboard content kinds — drive the context-sensitive tray menu and the
// auto-detect dispatch. Values mirror backend/tray.
const (
	clipKindNone    = 0
	clipKindText    = 1
	clipKindMessage = 2
	clipKindSigned  = 3
	clipKindKey     = 4
)

// clipboardKind classifies clipboard text.
func clipboardKind(trimmed string) int {
	switch {
	case trimmed == "":
		return clipKindNone
	case strings.HasPrefix(trimmed, "-----BEGIN PGP PUBLIC KEY BLOCK-----"),
		strings.HasPrefix(trimmed, "-----BEGIN PGP PRIVATE KEY BLOCK-----"):
		return clipKindKey
	case strings.HasPrefix(trimmed, "-----BEGIN PGP SIGNED MESSAGE-----"):
		return clipKindSigned
	case strings.HasPrefix(trimmed, "-----BEGIN PGP MESSAGE-----"):
		return clipKindMessage
	default:
		return clipKindText
	}
}

// watchClipboard monitors the clipboard, keeps the tray's clipboard
// subgroup in sync, and auto-detects PGP content according to the user's
// settings: keys open the import modal, encrypted messages open the decrypt
// flow, signed messages are verified with a system notification. Where the
// OS provides a cheap change counter (macOS, Windows) the monitor polls it
// at a short interval and only reads the clipboard content when it actually
// changed.
func (a *App) watchClipboard(ctx context.Context) {
	interval := 200 * time.Millisecond
	lastCount, counterSupported := clipboard.ChangeCount()
	if !counterSupported {
		// Linux: no counter — reading the text every tick is the only option.
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial tray state from the current clipboard.
	last := ""
	if text, err := runtime.ClipboardGetText(a.ctx); err == nil {
		last = strings.TrimSpace(text)
	}
	trayUpdateClipboard(clipboardKind(last))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if counterSupported {
			c, _ := clipboard.ChangeCount()
			if c == lastCount {
				continue
			}
			lastCount = c
		}

		text, err := runtime.ClipboardGetText(a.ctx)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == last {
			continue
		}
		last = trimmed

		// Tray follows every clipboard change — including our own output
		// (offering "Decrypt" on a message we just encrypted is fine).
		trayUpdateClipboard(clipboardKind(trimmed))

		if trimmed == "" {
			continue
		}

		a.clipMu.Lock()
		suppressed := trimmed == a.clipSuppress
		a.clipMu.Unlock()
		if suppressed {
			continue
		}

		a.dispatchClipboard(trimmed)
	}
}

// dispatchClipboard routes detected PGP clipboard content according to the
// auto-detect settings.
func (a *App) dispatchClipboard(trimmed string) {
	switch {
	case strings.HasPrefix(trimmed, "-----BEGIN PGP PUBLIC KEY BLOCK-----"):
		a.clipboardKeyDetected(trimmed, a.cfg.ClipDetectPublicKeys)
	case strings.HasPrefix(trimmed, "-----BEGIN PGP PRIVATE KEY BLOCK-----"):
		a.clipboardKeyDetected(trimmed, a.cfg.ClipDetectPrivateKeys)
	case strings.HasPrefix(trimmed, "-----BEGIN PGP SIGNED MESSAGE-----"):
		// Verify in the background — result arrives as a notification,
		// the window stays closed.
		if !a.cfg.ClipDetectSignatures {
			return
		}
		go a.verifyTextAction(trimmed)
	case strings.HasPrefix(trimmed, "-----BEGIN PGP MESSAGE-----"):
		// Only open the decrypt flow when one of our private keys matches.
		if !a.cfg.ClipDetectMessages {
			return
		}
		if bcrypto.FindDecryptionKey(a.store, trimmed) == "" {
			return
		}
		a.showMainWindow()
		runtime.EventsEmit(a.ctx, "clipboard-message-detected", trimmed)
	}
}

// clipboardKeyDetected opens the import modal for a detected key block when
// the configured mode applies ("unknown" only fires for keys not yet in the
// keystore).
func (a *App) clipboardKeyDetected(armored, mode string) {
	switch mode {
	case config.ClipKeysAll:
		// always offer
	case config.ClipKeysUnknown:
		anyNew := false
		for _, k := range a.store.Preview(armored) {
			if !k.AlreadyExists {
				anyNew = true
				break
			}
		}
		if !anyNew {
			return
		}
	default: // ClipKeysOff
		return
	}
	a.showMainWindow()
	runtime.EventsEmit(a.ctx, "clipboard-key-detected", armored)
}

// showMainWindow ensures the Wails window is visible and focused on the
// currently active workspace (the app may live in the tray).
func (a *App) showMainWindow() {
	prepareWindowForShow()
	runtime.WindowShow(a.ctx)
	runtime.WindowCenter(a.ctx)
}

// EncryptFileWithOutput encrypts a file for the given recipients and writes to outputPath.
//
//bind
func (a *App) EncryptFileWithOutput(inputPath string, recipientFingerprints []string, outputPath, signingFingerprint string) model.FileResult {
	return bcrypto.EncryptFileWithOutput(a.store, inputPath, recipientFingerprints, outputPath, signingFingerprint)
}

// DecryptFileWithOutput decrypts a file using the given private key + passphrase and writes to outputPath.
//
//bind
func (a *App) DecryptFileWithOutput(inputPath, outputPath, fingerprint, passphrase string) model.FileResult {
	return bcrypto.DecryptFileWithOutput(a.store, inputPath, outputPath, fingerprint, passphrase)
}
