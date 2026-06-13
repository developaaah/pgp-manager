package model

import "time"

// SubkeyInfo holds metadata about a PGP subkey.
type SubkeyInfo struct {
	Fingerprint string
	Algorithm   string
	Usage       []string   // values: "certify", "sign", "encrypt" (subset)
	CreatedAt   time.Time
	ExpiresAt   *time.Time // nil = no expiry
	IsRevoked   bool
}

// KeyInfo holds metadata about a PGP key.
type KeyInfo struct {
	Fingerprint   string
	PrimaryUID    string
	Email         string
	CreatedAt     *time.Time
	Expiry        *time.Time
	Status        string // "valid", "expired", "revoked"
	IsPrivate     bool
	AlreadyExists bool // set by PreviewKeys: key is already in the keystore
	Subkeys       []SubkeyInfo
}

// EncryptResult holds the result of an encryption operation.
type EncryptResult struct {
	Armored string
	Error   string
}

// DecryptResult holds the result of a decryption operation.
type DecryptResult struct {
	Plaintext string
	SignedBy  string
	Error     string
}

// SignResult holds the result of a sign operation.
type SignResult struct {
	Armored    string
	OutputPath string
	Error      string
}

// VerifyResult holds the result of a signature verification.
type VerifyResult struct {
	Valid    bool
	SignedBy string // fingerprint of the signer
	UID      string // display name of the signer
	Email    string // email of the signer
	Error    string
}

// AppConfig holds the user-visible configuration fields.
type AppConfig struct {
	Theme                     string
	PassphraseCacheTTLMinutes int
	KeysDir                   string   // resolved path — set to change the storage directory
	CustomKeyservers          []string // user-added keyservers; built-ins are hardcoded in app
	StartInTray               bool     // start hidden in the system tray
	ClipDetectMessages        bool     // auto-detect PGP messages in the clipboard
	ClipDetectPublicKeys      string   // "off" | "unknown" | "all"
	ClipDetectPrivateKeys     string   // "off" | "unknown" | "all"
	ClipDetectSignatures      bool     // auto-verify signed messages in the clipboard
	AutoCopyResults           bool     // copy encrypt/sign results to the clipboard automatically
}

// KeyserverEntry describes one keyserver (built-in or user-added).
type KeyserverEntry struct {
	Label   string
	URL     string
	BuiltIn bool
}

// FileResult holds the result of an encrypt/decrypt file operation.
type FileResult struct {
	OutputPath string // absolute path of the output file
	Error      string
}

// KeyserverResult represents a key found on a keyserver.
type KeyserverResult struct {
	Fingerprint string
	UID         string
	Email       string
}

// KeyImportEntry holds the result of importing a single key from a multi-key block.
type KeyImportEntry struct {
	Fingerprint string
	UID         string
	Error       string
}

// MultiImportResult holds the per-key results of importing multiple keys.
type MultiImportResult struct {
	Entries []KeyImportEntry
}

// ActionResult holds the result of a context-action triggered via URL scheme or system service.
type ActionResult struct {
	Action string `json:"action"` // "decrypt-text", "encrypt-text", "sign-text", "verify-text", "import-key"
	Output string `json:"output"`
	Error  string `json:"error"`
}
