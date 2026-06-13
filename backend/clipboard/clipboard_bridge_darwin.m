// Objective-C bridge for the NSPasteboard change counter.
// Compiled only on darwin (OS suffix in filename).

#import <AppKit/AppKit.h>

long pgp_clipboard_change_count(void) {
    return (long)[[NSPasteboard generalPasteboard] changeCount];
}
