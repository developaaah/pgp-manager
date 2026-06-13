//go:build windows

package notification

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
)

// powershellAUMID is PowerShell's registered AppUserModelID. Toasts from
// unpackaged Win32 apps are only displayed when the AUMID passed to
// CreateToastNotifier is registered with a Start-Menu shortcut — our app has
// none, so we borrow PowerShell's (the toast shows "Windows PowerShell" as
// the source app; the content is ours).
const powershellAUMID = `{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe`

// windowsNotifier sends Toast notifications via PowerShell.
type windowsNotifier struct{}

func newPlatformNotifier() Notifier {
	return &windowsNotifier{}
}

func (n *windowsNotifier) RequestPermission(_ context.Context) error {
	return nil // Windows does not require explicit permission for Toast notifications.
}

func xmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;",
	).Replace(s)
}

// showToast renders the given toast XML via PowerShell.
func showToast(ctx context.Context, toastXML string) error {
	ps := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom,ContentType=WindowsRuntime] | Out-Null
$xml = [Windows.Data.Xml.Dom.XmlDocument]::new()
$xml.LoadXml('%s')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('%s').Show($toast)
`, strings.ReplaceAll(toastXML, "'", "''"), powershellAUMID)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Debug("windows toast failed", "error", err, "output", string(out))
		return fmt.Errorf("notification: powershell: %w", err)
	}
	return nil
}

func (n *windowsNotifier) ShowInfo(ctx context.Context, title, body string) error {
	toastXML := fmt.Sprintf(`<toast>
  <visual>
    <binding template="ToastGeneric">
      <text>%s</text>
      <text>%s</text>
    </binding>
  </visual>
</toast>`, xmlEscape(title), xmlEscape(body))
	return showToast(ctx, toastXML)
}
