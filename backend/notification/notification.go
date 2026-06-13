// Package notification provides cross-platform system notifications for PGP Manager.
// Platform-specific implementations are selected via build tags.
//
// Notifications are only used for signature-verification results — everything
// else surfaces through the UI (auto-detect opens the window).
package notification

import "context"

// Notifier sends plain system notifications.
type Notifier interface {
	// RequestPermission asks the OS for notification permission (macOS only;
	// returns nil immediately on other platforms).
	RequestPermission(ctx context.Context) error

	// ShowInfo shows a plain notification without actions.
	ShowInfo(ctx context.Context, title, body string) error
}

// New returns the platform notifier.
func New() Notifier {
	return newPlatformNotifier()
}
