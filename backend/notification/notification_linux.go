//go:build linux

package notification

import (
	"context"
	"log/slog"
	"os/exec"
)

// linuxNotifier sends notifications via notify-send (libnotify).
type linuxNotifier struct{}

func newPlatformNotifier() Notifier {
	return &linuxNotifier{}
}

func (n *linuxNotifier) RequestPermission(_ context.Context) error {
	return nil // Linux does not require explicit notification permission.
}

func (n *linuxNotifier) ShowInfo(ctx context.Context, title, body string) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx,
		"notify-send", "--app-name", "PGP Manager", title, body)
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Debug("notify-send failed", "error", err, "output", string(out))
	}
	return nil
}
