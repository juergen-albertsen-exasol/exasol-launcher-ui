#!/bin/sh
# Installs Exasol Launcher UI on macOS and Linux:
#   curl -fsSL https://github.com/juergen-albertsen-exasol/exasol-launcher-ui/releases/latest/download/install.sh | sh
#
# EXASOL_LAUNCHER_UI_VERSION      release to install instead of the latest, e.g. 0.1.0
# EXASOL_LAUNCHER_UI_INSTALL_DIR  directory to install into instead of ~/.local/bin
# EXASOL_LAUNCHER_UI_DOWNLOAD_URL base URL of the releases, for mirrors and tests
set -eu

NAME="exasol-launcher-ui"
REPOSITORY_URL="https://github.com/juergen-albertsen-exasol/exasol-launcher-ui"
DOWNLOAD_URL="${EXASOL_LAUNCHER_UI_DOWNLOAD_URL:-$REPOSITORY_URL/releases}"
LAUNCHER_INSTALL_COMMAND="curl https://downloads.exasol.com/exasol-personal/installer.sh | sh"

fail() {
    echo "Error: $*" >&2
    exit 1
}

case "$(uname -s)" in
    Darwin) OS="macos" ;;
    Linux) OS="linux" ;;
    *) fail "unsupported operating system $(uname -s). Download a release from $REPOSITORY_URL/releases." ;;
esac

case "$(uname -m)" in
    x86_64 | amd64) ARCH="x86_64" ;;
    arm64 | aarch64) ARCH="arm64" ;;
    *) fail "unsupported architecture $(uname -m). Releases exist for x86_64 and arm64." ;;
esac

VERSION="${EXASOL_LAUNCHER_UI_VERSION:-}"
VERSION="${VERSION#v}"
if [ -n "$VERSION" ]; then
    RELEASE_URL="$DOWNLOAD_URL/download/v$VERSION"
    RELEASE="version $VERSION"
else
    RELEASE_URL="$DOWNLOAD_URL/latest/download"
    RELEASE="the latest release"
fi

INSTALL_DIR="${EXASOL_LAUNCHER_UI_INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_PATH="$INSTALL_DIR/$NAME"
ARCHIVE="${NAME}_${OS}_${ARCH}.tar.gz"

if command -v sha256sum >/dev/null 2>&1; then
    sha256() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
    sha256() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
    fail "neither sha256sum nor shasum is available to verify the download"
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
trap 'exit 130' INT TERM

download() {
    curl -fsSL --proto-redir '=https' "$RELEASE_URL/$1" -o "$WORK_DIR/$1" ||
        fail "cannot download $1 of $RELEASE from $RELEASE_URL. Check that the release exists."
}

echo "Downloading $ARCHIVE of $RELEASE..."
download checksums.txt
download "$ARCHIVE"

EXPECTED="$(awk -v archive="$ARCHIVE" '$2 == archive { print $1 }' "$WORK_DIR/checksums.txt")"
[ -n "$EXPECTED" ] || fail "checksums.txt has no checksum for $ARCHIVE"
ACTUAL="$(sha256 "$WORK_DIR/$ARCHIVE")"
[ "$EXPECTED" = "$ACTUAL" ] ||
    fail "checksum mismatch for $ARCHIVE: expected $EXPECTED, got $ACTUAL. Nothing was installed."

mkdir "$WORK_DIR/extracted"
tar -xzf "$WORK_DIR/$ARCHIVE" -C "$WORK_DIR/extracted" ||
    fail "cannot extract $ARCHIVE"
[ -f "$WORK_DIR/extracted/$NAME" ] || fail "$ARCHIVE does not contain $NAME"

mkdir -p "$INSTALL_DIR" || fail "cannot create $INSTALL_DIR"
# Stage next to the target so the final mv replaces it atomically.
STAGED="$INSTALL_DIR/.$NAME.$$"
cp "$WORK_DIR/extracted/$NAME" "$STAGED" || fail "cannot write to $INSTALL_DIR"
chmod 755 "$STAGED"
mv -f "$STAGED" "$INSTALL_PATH" || {
    rm -f "$STAGED"
    fail "cannot replace $INSTALL_PATH"
}

INSTALLED_VERSION="$("$INSTALL_PATH" --version)" ||
    fail "the installed $INSTALL_PATH does not run"

echo
echo "Installed $INSTALLED_VERSION to $INSTALL_PATH"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo
        echo "$INSTALL_DIR is not on your PATH. Add it to your shell profile:"
        echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
esac

if ! command -v exasol >/dev/null 2>&1; then
    echo
    echo "Exasol Launcher UI needs the Exasol Personal launcher (the 'exasol' command)."
    echo "Install it with:"
    echo "    $LAUNCHER_INSTALL_COMMAND"
fi

echo
echo "Start the UI and open it in your browser with:"
echo "    $NAME --open"
