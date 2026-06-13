#!/usr/bin/env bash
# Creates a styled release DMG: app bundle + /Applications symlink, optional
# background image, and a Finder icon layout (app left, Applications right).
#
# Usage: create-dmg.sh <path/to/App.app> <output.dmg>
#
# Background image: drop a PNG at build/dmg/background.png (recommended size
# 660x420 @1x; use background@2x.png inside a .tiff for retina if needed).
# Without the file a plain DMG is produced.
#
# The Finder styling step uses AppleScript and needs Automation permission for
# Finder on first run; if denied, a valid but unstyled DMG is still produced.
set -euo pipefail

APP_PATH="$1"
DMG_PATH="$2"
VOL_NAME="PGP Manager"
BG_IMG="$(cd "$(dirname "$0")" && pwd)/background.png"

APP_NAME="$(basename "$APP_PATH")"
STAGING="$(mktemp -d /tmp/pgpdmg-staging-XXXXXX)"
TMP_DMG="$(mktemp -u /tmp/pgpdmg-XXXXXX).dmg"
trap 'rm -rf "$STAGING" "$TMP_DMG"' EXIT

cp -R "$APP_PATH" "$STAGING/"
ln -s /Applications "$STAGING/Applications"

HAS_BG=0
if [[ -f "$BG_IMG" ]]; then
  mkdir "$STAGING/.background"
  cp "$BG_IMG" "$STAGING/.background/background.png"
  HAS_BG=1
fi

# Read-write image first so Finder can persist the view settings (.DS_Store).
hdiutil create -volname "$VOL_NAME" -srcfolder "$STAGING" -fs HFS+ \
  -format UDRW -ov "$TMP_DMG" >/dev/null

ATTACH_INFO=$(hdiutil attach "$TMP_DMG" -readwrite -noverify -noautoopen)
DEVICE=$(echo "$ATTACH_INFO" | grep "/Volumes/$VOL_NAME" | awk '{print $1}')
MOUNT_DIR=$(echo "$ATTACH_INFO" | grep "/Volumes/$VOL_NAME" | sed 's|.*\(/Volumes/.*\)|\1|')
sleep 1

# Hide .background and .fseventsd so they don't appear in the Finder window.
# chflags hidden sets the macOS Invisible attribute — works even with
# "Show hidden files" (Cmd+Shift+.) enabled, unlike dot-prefix alone.
[[ -d "$MOUNT_DIR/.background" ]] && chflags hidden "$MOUNT_DIR/.background"
[[ -d "$MOUNT_DIR/.fseventsd" ]] && chflags hidden "$MOUNT_DIR/.fseventsd"

BG_LINE=""
if [[ "$HAS_BG" == "1" ]]; then
  BG_LINE='set background picture of viewOptions to file ".background:background.png"'
fi

if ! osascript >/dev/null <<EOF
tell application "Finder"
  tell disk "$VOL_NAME"
    open
    set current view of container window to icon view
    set toolbar visible of container window to false
    set statusbar visible of container window to false
    set bounds of container window to {200, 90, 1000, 590}
    set viewOptions to icon view options of container window
    set arrangement of viewOptions to not arranged
    set icon size of viewOptions to 95
    $BG_LINE
    set position of item "$APP_NAME" of container window to {275, 280}
    set position of item "Applications" of container window to {525, 280}
    update without registering applications
    delay 1
    close
  end tell
end tell
EOF
then
  echo "warning: Finder styling skipped (Automation permission for Finder denied?)" >&2
fi

sync
hdiutil detach "$DEVICE" -quiet -force >/dev/null 2>&1 || true

rm -f "$DMG_PATH"
hdiutil convert "$TMP_DMG" -format UDZO -imagekey zlib-level=9 -o "$DMG_PATH" >/dev/null
echo "Created $DMG_PATH"
