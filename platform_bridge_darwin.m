// macOS platform bridge: NSServices provider and NSStatusItem (tray).
//
// Services: text selections are passed straight into Go via
// pgpGoServiceFired(); Finder file selections via pgpGoFileServiceFired()
// (one call per invocation, paths newline-joined). Tray menu clicks go
// through pgpGoTrayAction(). All Go entry points are declared in
// _cgo_export.h.

#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>
#include "_cgo_export.h"

// ── Services provider ─────────────────────────────────────────────────────────

@interface PGPServiceDelegate : NSObject
@end

static void fireTextService(NSString* action, NSPasteboard* pb) {
    NSString* text = [pb stringForType:NSPasteboardTypeString];
    if (!text || text.length == 0) return;
    pgpGoServiceFired((char*)[action UTF8String], (char*)[text UTF8String]);
}

static void fireFileService(NSString* action, NSPasteboard* pb) {
    NSArray<NSURL*>* urls = [pb readObjectsForClasses:@[[NSURL class]]
                                              options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    NSMutableArray<NSString*>* paths = [NSMutableArray array];
    for (NSURL* url in urls) {
        if (!url.path || url.path.length == 0) continue;
        [paths addObject:url.path];
    }
    if (paths.count == 0) return;
    // One callback for the whole Finder selection so multi-file actions can
    // be handled as a unit; paths are newline-joined (newlines in file names
    // are not supported).
    NSString* joined = [paths componentsJoinedByString:@"\n"];
    pgpGoFileServiceFired((char*)[action UTF8String], (char*)[joined UTF8String]);
}

@implementation PGPServiceDelegate

// ── Text services ──
- (void)handlePGPImportKey:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireTextService(@"import-key", pb);
}
- (void)handlePGPEncrypt:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireTextService(@"encrypt-text", pb);
}
- (void)handlePGPDecrypt:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireTextService(@"decrypt-text", pb);
}
- (void)handlePGPSign:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireTextService(@"sign-text", pb);
}
- (void)handlePGPVerify:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireTextService(@"verify-text", pb);
}

// ── File services (Finder) ──
- (void)handlePGPEncryptFile:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireFileService(@"encrypt-file", pb);
}
- (void)handlePGPDecryptFile:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireFileService(@"decrypt-file", pb);
}
- (void)handlePGPSignFile:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireFileService(@"sign-file", pb);
}
- (void)handlePGPVerifyFile:(NSPasteboard*)pb userData:(NSString*)ud error:(NSString**)err {
    fireFileService(@"verify-file", pb);
}

@end

// ── Tray (NSStatusItem) ───────────────────────────────────────────────────────

@interface PGPTrayDelegate : NSObject
- (void)openApp:(id)sender;
- (void)encryptClipboard:(id)sender;
- (void)signClipboard:(id)sender;
- (void)decryptClipboard:(id)sender;
- (void)verifyClipboard:(id)sender;
- (void)importClipboard:(id)sender;
- (void)quitApp:(id)sender;
@end

@implementation PGPTrayDelegate
- (void)openApp:(id)sender          { pgpGoTrayAction((char*)"open"); }
- (void)encryptClipboard:(id)sender { pgpGoTrayAction((char*)"encrypt-clipboard"); }
- (void)signClipboard:(id)sender    { pgpGoTrayAction((char*)"sign-clipboard"); }
- (void)decryptClipboard:(id)sender { pgpGoTrayAction((char*)"decrypt-clipboard"); }
- (void)verifyClipboard:(id)sender  { pgpGoTrayAction((char*)"verify-clipboard"); }
- (void)importClipboard:(id)sender  { pgpGoTrayAction((char*)"import-clipboard"); }
- (void)quitApp:(id)sender          { pgpGoTrayAction((char*)"quit"); }
@end

// ── C entry points ────────────────────────────────────────────────────────────

static PGPServiceDelegate* _serviceDelegate = nil;
static PGPTrayDelegate*    _trayDelegate    = nil;
static NSStatusItem*       _statusItem      = nil;

// Clipboard subgroup items + their separator — visibility follows the
// current clipboard content (pgp_tray_set_clipboard).
static NSMenuItem* _itEncrypt = nil;
static NSMenuItem* _itSign    = nil;
static NSMenuItem* _itDecrypt = nil;
static NSMenuItem* _itVerify  = nil;
static NSMenuItem* _itImport  = nil;
static NSMenuItem* _sepClip   = nil;

void pgp_register_services(void) {
    // setServicesProvider must run on the main thread; Go callbacks arrive on
    // arbitrary threads.
    dispatch_async(dispatch_get_main_queue(), ^{
        _serviceDelegate = [[PGPServiceDelegate alloc] init];
        [NSApp setServicesProvider:_serviceDelegate];
        NSUpdateDynamicServices();
    });
}

static NSMenuItem* addTrayItem(NSMenu* menu, NSString* title, SEL action) {
    NSMenuItem* item = [[NSMenuItem alloc] initWithTitle:title
                                                  action:action
                                           keyEquivalent:@""];
    item.target = _trayDelegate;
    [menu addItem:item];
    return item;
}

void pgp_setup_tray(const void* iconData, int iconLen) {
    NSData* data = [NSData dataWithBytes:iconData length:iconLen];
    dispatch_async(dispatch_get_main_queue(), ^{
        _trayDelegate = [[PGPTrayDelegate alloc] init];

        // statusItemWithLength: returns an autoreleased object — retain it
        // (compiled without ARC) or the system removes the item again.
        _statusItem = [[[NSStatusBar systemStatusBar]
            statusItemWithLength:NSSquareStatusItemLength] retain];

        // carbon "ibm--cloud--key-protect" as template image (black + alpha).
        NSImage* icon = [[NSImage alloc] initWithData:data];
        [icon setSize:NSMakeSize(16, 16)];
        [icon setTemplate:YES];
        _statusItem.button.image = icon;

        NSMenu* menu = [[NSMenu alloc] init];

        addTrayItem(menu, @"Open PGP Manager", @selector(openApp:));

        _sepClip = [NSMenuItem separatorItem];
        [menu addItem:_sepClip];

        _itEncrypt = addTrayItem(menu, @"Encrypt Clipboard",         @selector(encryptClipboard:));
        _itSign    = addTrayItem(menu, @"Sign Clipboard",            @selector(signClipboard:));
        _itDecrypt = addTrayItem(menu, @"Decrypt Clipboard",         @selector(decryptClipboard:));
        _itVerify  = addTrayItem(menu, @"Verify Clipboard",          @selector(verifyClipboard:));
        _itImport  = addTrayItem(menu, @"Import Key from Clipboard", @selector(importClipboard:));

        // Hidden until the clipboard monitor reports applicable content.
        _sepClip.hidden   = YES;
        _itEncrypt.hidden = YES;
        _itSign.hidden    = YES;
        _itDecrypt.hidden = YES;
        _itVerify.hidden  = YES;
        _itImport.hidden  = YES;

        [menu addItem:[NSMenuItem separatorItem]];
        addTrayItem(menu, @"Quit PGP Manager", @selector(quitApp:));

        _statusItem.menu = menu;
    });
}

// pgp_tray_set_clipboard shows only the clipboard actions that apply to the
// current clipboard content. Kinds: 0 none, 1 text, 2 message, 3 signed, 4 key.
void pgp_tray_set_clipboard(int kind) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (_itEncrypt == nil) return; // tray not set up yet
        _itEncrypt.hidden = (kind != 1);
        _itSign.hidden    = (kind != 1);
        _itDecrypt.hidden = (kind != 2);
        _itVerify.hidden  = (kind != 3);
        _itImport.hidden  = (kind != 4);
        _sepClip.hidden   = (kind == 0);
    });
}

// pgp_window_move_to_active_space makes the app windows follow the user to
// the currently active Space when shown from the tray.
void pgp_window_move_to_active_space(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        for (NSWindow* w in [NSApp windows]) {
            w.collectionBehavior |= NSWindowCollectionBehaviorMoveToActiveSpace;
        }
    });
}

// pgp_set_dock_icon_visible controls whether the app appears in the Dock.
// Wails forces NSApplicationActivationPolicyRegular at startup (which shows a
// Dock icon), overriding LSUIElement=true in Info.plist. Passing visible=0
// resets the policy to NSApplicationActivationPolicyAccessory, which keeps
// windows fully functional while suppressing the Dock entry and Cmd+Tab slot.
void pgp_set_dock_icon_visible(int visible) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplicationActivationPolicy policy = visible
            ? NSApplicationActivationPolicyRegular
            : NSApplicationActivationPolicyAccessory;
        [NSApp setActivationPolicy:policy];
    });
}
