// Objective-C bridge for SMAppService login items (macOS 13+).
// Compiled only on darwin (OS suffix in filename).

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

// pgp_has_bundle mirrors the notification bridge: SMAppService requires a
// proper .app bundle (wails dev runs from a temp directory).
static BOOL pgp_autostart_has_bundle(void) {
    NSString* bid = [[NSBundle mainBundle] bundleIdentifier];
    return (bid != nil && bid.length > 0);
}

// pgp_autostart_supported returns 1 when SMAppService is available.
int pgp_autostart_supported(void) {
    if (@available(macOS 13.0, *)) {
        return pgp_autostart_has_bundle() ? 1 : 0;
    }
    return 0;
}

// pgp_autostart_status returns 1 (enabled), 0 (disabled) or -1 (unsupported).
int pgp_autostart_status(void) {
    if (@available(macOS 13.0, *)) {
        if (!pgp_autostart_has_bundle()) return -1;
        return ([SMAppService mainAppService].status == SMAppServiceStatusEnabled) ? 1 : 0;
    }
    return -1;
}

// pgp_autostart_set returns 0 (ok), -1 (unsupported) or -2 (error).
int pgp_autostart_set(int enable) {
    if (@available(macOS 13.0, *)) {
        if (!pgp_autostart_has_bundle()) return -2;
        NSError* err = nil;
        SMAppService* svc = [SMAppService mainAppService];
        BOOL ok = enable ? [svc registerAndReturnError:&err]
                         : [svc unregisterAndReturnError:&err];
        if (!ok && !enable && svc.status == SMAppServiceStatusNotRegistered) {
            return 0; // unregistering something never registered is fine
        }
        return ok ? 0 : -2;
    }
    return -1;
}
