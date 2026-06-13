// Objective-C bridge for macOS UNUserNotificationCenter.
// Compiled only on darwin (OS suffix in filename).

#import <UserNotifications/UserNotifications.h>
#import <Foundation/Foundation.h>

// ── Delegate ──────────────────────────────────────────────────────────────────

@interface PGPNotifDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation PGPNotifDelegate

// Allow notifications to appear even while the app is in the foreground.
- (void)userNotificationCenter:(UNUserNotificationCenter*)center
    willPresentNotification:(UNNotification*)notification
    withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler
{
    completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionSound);
}

@end

// ── C-callable setup helpers ──────────────────────────────────────────────────

static PGPNotifDelegate* _delegate = nil;
static BOOL _notifAvailable = NO;   // bundle present, center set up
static BOOL _notifAuthorized = NO;  // user granted notification permission

// pgp_has_bundle returns YES when the process is running inside a proper .app
// bundle with a CFBundleIdentifier. UNUserNotificationCenter crashes without one
// (e.g. during wails dev where the binary runs from a temp directory).
static BOOL pgp_has_bundle(void) {
    NSString* bid = [[NSBundle mainBundle] bundleIdentifier];
    return (bid != nil && bid.length > 0);
}

void pgp_setup_notifications(void) {
    if (!pgp_has_bundle()) {
        return; // running without app bundle (wails dev) — osascript fallback is used
    }

    UNUserNotificationCenter* center = [UNUserNotificationCenter currentNotificationCenter];
    _delegate = [[PGPNotifDelegate alloc] init];
    [center setDelegate:_delegate];
    _notifAvailable = YES;
}

void pgp_request_permission(void) {
    if (!_notifAvailable) return;
    UNUserNotificationCenter* center = [UNUserNotificationCenter currentNotificationCenter];
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                          completionHandler:^(BOOL granted, NSError* err) {
        (void)err;
        _notifAuthorized = granted;
    }];
    // Also pick up a previously granted state (the completion handler above
    // only resolves after the user dealt with the permission dialog).
    [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings* settings) {
        if (settings.authorizationStatus == UNAuthorizationStatusAuthorized ||
            settings.authorizationStatus == UNAuthorizationStatusProvisional) {
            _notifAuthorized = YES;
        }
    }];
}

// pgp_notifications_ready returns 1 when UNUserNotificationCenter can be used
// for delivery (bundle present and authorization granted).
int pgp_notifications_ready(void) {
    return (_notifAvailable && _notifAuthorized) ? 1 : 0;
}

// pgp_show_simple_notification shows a plain notification without action buttons.
void pgp_show_simple_notification(const char* notifID, const char* title, const char* body) {
    if (!_notifAvailable) return;

    UNMutableNotificationContent* content = [[UNMutableNotificationContent alloc] init];
    content.title = [NSString stringWithUTF8String:title];
    content.body  = [NSString stringWithUTF8String:body];
    content.sound = UNNotificationSound.defaultSound;

    UNNotificationRequest* req =
        [UNNotificationRequest requestWithIdentifier:[NSString stringWithUTF8String:notifID]
                                             content:content
                                             trigger:nil];
    [[UNUserNotificationCenter currentNotificationCenter]
        addNotificationRequest:req withCompletionHandler:nil];
}
