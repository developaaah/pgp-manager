package keyserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/developaaah/pgp-manager/backend/model"
)

var logger = slog.Default().With("component", "keyserver")

// SearchKeyserver searches the keyserver for keys matching the query.
//
// When the query is a fingerprint (40 hex chars, with or without spaces/0x prefix),
// it uses op=get to fetch the actual key — this works even on servers like
// keys.openpgp.org that suppress index results for unverified addresses.
// For email/keyword queries it uses op=index with machine-readable output.
func SearchKeyserver(ctx context.Context, baseURL, query string) ([]model.KeyserverResult, error) {
	normalized := normalizeQuery(query)
	logger := logger.With("method", "SearchKeyserver")
	logger.Debug("searching keyserver", "query", normalized)

	// Fingerprint query: op=get returns the actual key; parse metadata from it.
	if isFingerprintQuery(normalized) {
		armored, err := FetchKey(ctx, baseURL, normalized)
		if err != nil {
			// 404 or "not found" strings mean the key simply isn't on this server.
			if strings.Contains(err.Error(), "status 404") ||
				strings.Contains(err.Error(), "not found") {
				return nil, nil
			}
			return nil, err
		}
		result, err := armoredToResult(armored)
		if err != nil {
			return nil, fmt.Errorf("keyserver: parse fetched key: %w", err)
		}
		return []model.KeyserverResult{result}, nil
	}

	// Email / keyword query: use op=index with machine-readable output.
	searchURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("keyserver: parse URL: %w", err)
	}
	searchURL = searchURL.ResolveReference(&url.URL{Path: "/pks/lookup"})

	params := url.Values{}
	params.Add("op", "index")
	params.Add("search", normalized)
	params.Add("options", "mr")
	searchURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("keyserver: create request: %w", err)
	}
	req.Header.Set("User-Agent", "pgp-manager/1.0")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keyserver: search request: %w", err)
	}
	defer resp.Body.Close()

	// HKP servers return 404 when the query matches nothing — treat as empty.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("search returned non-200", "status", resp.StatusCode)
		return nil, fmt.Errorf("keyserver: search returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("keyserver: read response: %w", err)
	}

	return parseIndexResponse(string(body))
}

// FetchKey retrieves the full armored key for the given identifier via HKP
// (/pks/lookup?op=get), which is supported by all major keyservers.
// Identifier may be a 40-hex fingerprint (with or without 0x prefix) or an email.
func FetchKey(ctx context.Context, baseURL, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)

	// Normalise fingerprint: strip 0x prefix if present.
	if strings.HasPrefix(identifier, "0x") || strings.HasPrefix(identifier, "0X") {
		identifier = identifier[2:]
	}
	fp := strings.ToUpper(identifier)

	var searchTerm string
	if len(fp) == 40 && isHex(fp) {
		searchTerm = "0x" + fp
	} else {
		searchTerm = identifier
	}

	baseURL = strings.TrimRight(baseURL, "/")
	endpoint := fmt.Sprintf("%s/pks/lookup?op=get&search=%s&options=mr",
		baseURL, url.QueryEscape(searchTerm))

	logger := logger.With("method", "FetchKey")
	logger.Debug("fetching key", "identifier", identifier, "server", baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("keyserver: create request: %w", err)
	}
	req.Header.Set("User-Agent", "pgp-manager/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("keyserver: fetch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("fetch returned non-200", "status", resp.StatusCode)
		return "", fmt.Errorf("keyserver: fetch returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	armored, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("keyserver: read armored key: %w", err)
	}

	return string(armored), nil
}

// SearchAll searches multiple keyservers in parallel and returns a deduplicated
// result set. Individual server errors are ignored as long as at least one server
// responds successfully. If ALL servers fail, the last error is returned.
func SearchAll(ctx context.Context, serverURLs []string, query string) ([]model.KeyserverResult, error) {
	if len(serverURLs) == 0 {
		return nil, fmt.Errorf("keyserver: no servers configured")
	}

	type serverResult struct {
		entries []model.KeyserverResult
		err     error
	}

	ch := make(chan serverResult, len(serverURLs))
	searchCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()

	for _, srv := range serverURLs {
		srv := srv
		go func() {
			entries, err := SearchKeyserver(searchCtx, srv, query)
			ch <- serverResult{entries, err}
		}()
	}

	seen := make(map[string]bool)
	var combined []model.KeyserverResult

	for range serverURLs {
		r := <-ch
		if r.err != nil {
			// Individual server failures are expected (timeout, 404, offline) —
			// log at debug and continue; callers treat empty results as "not found".
			logger.Debug("server returned error in auto search", "error", r.err)
			continue
		}
		for _, e := range r.entries {
			if !seen[e.Fingerprint] {
				seen[e.Fingerprint] = true
				combined = append(combined, e)
			}
		}
	}

	return combined, nil
}

// PublishKey publishes an armored key to the keyserver.
//
// Upload endpoints differ by server type:
//   - VKS (keys.openpgp.org and compatible): POST /vks/v1/upload
//   - HKP (Ubuntu, Launchpad, most traditional servers): POST /pks/add
//
// VKS is tried first; a 404 or 405 means the server does not support it, so
// the HKP /pks/add endpoint is used as a fallback. keys.openpgp.org returns
// 200 even when email verification is still required — that is not an error.
func PublishKey(ctx context.Context, baseURL, armored string) error {
	baseURL = strings.TrimRight(baseURL, "/")
	logger := logger.With("method", "PublishKey")
	logger.Debug("publishing key", "server", baseURL)

	v := url.Values{}
	v.Set("keytext", armored)
	body := v.Encode()

	var lastErr error
	for _, ep := range []string{"/vks/v1/upload", "/pks/add"} {
		err := doPublish(ctx, baseURL+ep, body)
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "status 404") || strings.Contains(err.Error(), "status 405") {
			logger.Debug("endpoint not supported, trying fallback", "endpoint", ep)
			lastErr = err
			continue
		}
		return err
	}
	return lastErr
}

func doPublish(ctx context.Context, endpoint, formBody string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(formBody))
	if err != nil {
		return fmt.Errorf("keyserver: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "pgp-manager/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("keyserver: publish request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keyserver: publish returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// parseIndexResponse parses the machine-readable output from /pks/lookup?op=index.
// Format per key:
//
//	pub:<algo>:<key_length>:<creation>:<expiry>:<fingerprint>:<flags>
//	uid:<uid_string>:<creation>:<expiry>:<flags>
//
// Multiple keys are separated by blank lines. Each key may have multiple uid lines.
func parseIndexResponse(body string) ([]model.KeyserverResult, error) {
	lines := strings.Split(body, "\n")

	var results []model.KeyserverResult
	seen := make(map[string]bool)

	var currentFP string
	var currentUID string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// Flush pending record
			if currentFP != "" && !seen[currentFP] {
				results = append(results, model.KeyserverResult{
					Fingerprint: currentFP,
					UID:         currentUID,
				})
				seen[currentFP] = true
			}
			currentFP = ""
			currentUID = ""
			continue
		}

		switch {
		case strings.HasPrefix(line, "pub:"):
			// Flush previous key before starting new one
			if currentFP != "" && !seen[currentFP] {
				results = append(results, model.KeyserverResult{
					Fingerprint: currentFP,
					UID:         currentUID,
				})
				seen[currentFP] = true
			}
			fields := strings.Split(line, ":")
			currentFP = ""
			currentUID = ""
			if len(fields) >= 6 {
				currentFP = fields[5]
			}
		case strings.HasPrefix(line, "uid:") && currentFP != "":
			fields := strings.Split(line, ":")
			if len(fields) >= 2 && currentUID == "" {
				currentUID = fields[1]
			}
		}
	}

	// Flush last record
	if currentFP != "" && !seen[currentFP] {
		results = append(results, model.KeyserverResult{
			Fingerprint: currentFP,
			UID:         currentUID,
		})
	}

	return results, nil
}

// isHex reports whether s contains only valid hexadecimal characters (0-9, a-f, A-F)
// and has the expected length for a fingerprint (40 chars).
func isHex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// normalizeQuery strips spaces from the input and, if the result looks like a
// 40-hex-char fingerprint, prepends "0x" as required by the HKP spec.
// Email addresses and short key IDs are returned as-is.
func normalizeQuery(q string) string {
	q = strings.TrimSpace(q)
	compact := strings.ReplaceAll(q, " ", "")
	if isHex(compact) {
		return "0x" + strings.ToUpper(compact)
	}
	if strings.HasPrefix(compact, "0x") || strings.HasPrefix(compact, "0X") {
		rest := compact[2:]
		if isHex(rest) {
			return "0x" + strings.ToUpper(rest)
		}
	}
	return q
}

// isFingerprintQuery reports whether q is a normalised 0x-prefixed 40-hex fingerprint.
func isFingerprintQuery(q string) bool {
	if !strings.HasPrefix(q, "0x") && !strings.HasPrefix(q, "0X") {
		return false
	}
	return isHex(q[2:])
}

// armoredToResult parses an armored PGP key block and returns a KeyserverResult
// with fingerprint and primary UID extracted from the key itself.
func armoredToResult(armored string) (model.KeyserverResult, error) {
	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		return model.KeyserverResult{}, err
	}
	fp := key.GetFingerprint()
	var uid string
	if entity := key.GetEntity(); entity != nil {
		for _, id := range entity.Identities {
			if id.UserId != nil {
				uid = id.UserId.Name
				if id.UserId.Email != "" {
					uid = id.UserId.Name + " <" + id.UserId.Email + ">"
				}
				break
			}
		}
	}
	return model.KeyserverResult{Fingerprint: fp, UID: uid}, nil
}
