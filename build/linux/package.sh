#!/usr/bin/env bash
# Builds .deb and .rpm packages from a compiled pgpmanager binary.
#
# Usage: package.sh <binary> <arch> <version> [outdir]
#   binary:  path to the compiled pgpmanager binary
#   arch:    amd64 | arm64 | 386
#   version: e.g. v1.2.3 or 1.2.3  (v-prefix is stripped)
#   outdir:  output directory (default: ./bin)
#
# Requires: fpm (gem install fpm), ImageMagick (convert), rpm (for .rpm on Debian)
set -euo pipefail

BINARY="$1"
ARCH="$2"
RAW_VERSION="$3"          # e.g. v1.2.3 — kept as-is for the output filename
VERSION="${RAW_VERSION#v}" # e.g. 1.2.3 — used in package metadata (no v-prefix)
OUTDIR="${4:-./bin}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_ID="com.developaaah.pgp-manager"
BIN_NAME="pgpmanager"
MAINTAINER="Dennis Schuster <hi@dennisschuster.net>"

# RELEASE_ARCH matches the naming used by all other release artifacts:
#   amd64 → amd, arm64 → arm, 386 → 32bit
case "$ARCH" in
  amd64) RELEASE_ARCH="amd";   DEB_ARCH="amd64"; RPM_ARCH="x86_64"  ;;
  arm64) RELEASE_ARCH="arm";   DEB_ARCH="arm64"; RPM_ARCH="aarch64"  ;;
  386)   RELEASE_ARCH="32bit"; DEB_ARCH="i386";  RPM_ARCH="i686"     ;;
  *) echo "Unknown arch: $ARCH" >&2; exit 1                           ;;
esac

# ── Dependency check ──────────────────────────────────────────────────────────
if ! command -v fpm >/dev/null 2>&1; then
  echo "warning: 'fpm' not found — skipping package build" >&2
  echo "         Install with: sudo gem install fpm" >&2
  echo "         On macOS:     brew install ruby && sudo gem install fpm" >&2
  exit 0
fi

STAGING="$(mktemp -d /tmp/pgp-pkg-XXXXXX)"
trap 'rm -rf "$STAGING"' EXIT

# install -D is a GNU extension not available on macOS — use mkdir -p instead.

# ── Binary ────────────────────────────────────────────────────────────────────
mkdir -p "$STAGING/usr/bin"
install -m755 "$BINARY" "$STAGING/usr/bin/$BIN_NAME"

# ── Icons (resize from 1024x1024 source) ─────────────────────────────────────
SRC_ICON="$SCRIPT_DIR/../appicon.png"
if command -v convert >/dev/null 2>&1; then
  for SIZE in 16 32 48 64 128 256 512; do
    ICON_DIR="$STAGING/usr/share/icons/hicolor/${SIZE}x${SIZE}/apps"
    mkdir -p "$ICON_DIR"
    convert "$SRC_ICON" -resize "${SIZE}x${SIZE}" "$ICON_DIR/$APP_ID.png"
  done
else
  echo "warning: 'convert' (ImageMagick) not found — copying source icon for all sizes" >&2
  echo "         Install with: brew install imagemagick" >&2
  for SIZE in 16 32 48 64 128 256 512; do
    ICON_DIR="$STAGING/usr/share/icons/hicolor/${SIZE}x${SIZE}/apps"
    mkdir -p "$ICON_DIR"
    cp "$SRC_ICON" "$ICON_DIR/$APP_ID.png"
  done
fi

# ── Desktop file ──────────────────────────────────────────────────────────────
mkdir -p "$STAGING/usr/share/applications"
install -m644 "$SCRIPT_DIR/pgp-manager.desktop" \
  "$STAGING/usr/share/applications/$APP_ID.desktop"

# ── AppStream metainfo ────────────────────────────────────────────────────────
mkdir -p "$STAGING/usr/share/metainfo"
install -m644 "$SCRIPT_DIR/$APP_ID.metainfo.xml" \
  "$STAGING/usr/share/metainfo/$APP_ID.metainfo.xml"

# ── Copyright ─────────────────────────────────────────────────────────────────
mkdir -p "$STAGING/usr/share/doc/$BIN_NAME"
cat > "$STAGING/usr/share/doc/$BIN_NAME/copyright" <<EOF
PGP Manager — Modern OpenPGP key management and encryption
Copyright 2026 Dennis Schuster
Licensed under the MIT License.
https://github.com/developaaah/pgp-manager/blob/main/LICENSE
EOF

mkdir -p "$OUTDIR"

COMMON_FPM=(
  fpm
  --input-type dir
  --chdir "$STAGING"
  --name "$BIN_NAME"
  --version "$VERSION"
  --maintainer "$MAINTAINER"
  --description "Modern OpenPGP key management and encryption"
  --url "https://github.com/developaaah/pgp-manager"
  --license MIT
  --vendor "Dennis Schuster"
  --category "utils"
  --force
)

# ── .deb ──────────────────────────────────────────────────────────────────────
echo "Building .deb ($DEB_ARCH)..."
"${COMMON_FPM[@]}" \
  --output-type deb \
  --architecture "$DEB_ARCH" \
  --depends "libgtk-3-0" \
  --depends "libwebkit2gtk-4.1-0" \
  --deb-compression xz \
  --package "$OUTDIR/PGP-Manager_linux_${RELEASE_ARCH}_${RAW_VERSION}.deb" \
  .

# ── .rpm ──────────────────────────────────────────────────────────────────────
echo "Building .rpm ($RPM_ARCH)..."
"${COMMON_FPM[@]}" \
  --output-type rpm \
  --architecture "$RPM_ARCH" \
  --depends "gtk3" \
  --depends "webkit2gtk4.1" \
  --package "$OUTDIR/PGP-Manager_linux_${RELEASE_ARCH}_${RAW_VERSION}.rpm" \
  .

echo "Packages written to $OUTDIR"
