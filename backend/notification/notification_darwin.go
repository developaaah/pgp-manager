//go:build darwin

package notification

/*
#cgo CFLAGS: -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework UserNotifications -framework Foundation
#include <stdlib.h>

// Implemented in notification_bridge_darwin.m
void pgp_setup_notifications(void);
void pgp_request_permission(void);
int  pgp_notifications_ready(void);
void pgp_show_simple_notification(const char* notifID, const char* title, const char* body);
*/
import "C"

import (
	"context"
	"fmt"
	"os/exec"
	"time"
	"unsafe"
)

type darwinNotifier struct{}

func newPlatformNotifier() Notifier {
	C.pgp_setup_notifications()
	return &darwinNotifier{}
}

func (n *darwinNotifier) RequestPermission(_ context.Context) error {
	C.pgp_request_permission()
	return nil
}

// ready reports whether UNUserNotificationCenter is usable (proper app bundle
// and authorization granted). Otherwise notifications fall back to osascript.
func (n *darwinNotifier) ready() bool {
	return C.pgp_notifications_ready() != 0
}

// osascriptNotify shows a notification via AppleScript. Works without an app
// bundle (wails dev) and without notification authorization.
func osascriptNotify(ctx context.Context, title, body string) error {
	script := fmt.Sprintf("display notification %q with title %q", body, title)
	return exec.CommandContext(ctx, "osascript", "-e", script).Run()
}

func (n *darwinNotifier) ShowInfo(ctx context.Context, title, body string) error {
	if !n.ready() {
		return osascriptNotify(ctx, title, body)
	}
	id := fmt.Sprintf("pgp-info-%d", time.Now().UnixNano())
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cBody))
	C.pgp_show_simple_notification(cID, cTitle, cBody)
	return nil
}
