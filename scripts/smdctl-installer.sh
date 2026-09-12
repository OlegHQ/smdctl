#!/bin/sh
set -eu

APP_NAME=smdctl
APP_VERSION=${SMDCTL_VERSION:-latest}
REPOSITORY=${SMDCTL_REPOSITORY:-OlegHQ/smdctl}

usage() {
    cat <<EOF
smdctl-installer.sh

Download, verify, and install smdctl ${APP_VERSION} for Linux.

Usage: smdctl-installer.sh [-q|--quiet] [-h|--help]

Environment:
  SMDCTL_VERSION       release version to install (default: latest)
  SMDCTL_DOWNLOAD_URL  release asset base URL (for mirrors/testing)
  SMDCTL_INSTALL_DIR   binary destination (default: \$HOME/.local/bin)
  SMDCTL_REPOSITORY    GitHub owner/repository (default: ${REPOSITORY})
EOF
}

quiet=${SMDCTL_PRINT_QUIET:-0}
for argument in "$@"; do
    case "$argument" in
        -h|--help) usage; exit 0 ;;
        -q|--quiet) quiet=1 ;;
        *) printf 'smdctl installer: unknown option: %s\n' "$argument" >&2; exit 2 ;;
    esac
done

say() {
    if [ "$quiet" != "1" ]; then
        printf '%s\n' "$*"
    fi
}

fail() {
    printf 'smdctl installer: %s\n' "$*" >&2
    exit 1
}

for command in uname mktemp mkdir mv chmod awk tar curl; do
    command -v "$command" >/dev/null 2>&1 || fail "required command not found: $command"
done

case "$(uname -s)" in
    Linux) ;;
    *) fail "unsupported operating system: $(uname -s); smdctl requires Linux with systemd" ;;
esac

case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
esac

case "$APP_VERSION" in
    v*) tag=$APP_VERSION; APP_VERSION=${APP_VERSION#v} ;;
    latest) tag= ;;
    *) tag="v$APP_VERSION" ;;
esac
if [ "$APP_VERSION" != "latest" ]; then
    case "$APP_VERSION" in
        ''|*[!A-Za-z0-9._+-]*) fail "invalid SMDCTL_VERSION: $APP_VERSION" ;;
    esac
fi

if [ -n "${SMDCTL_INSTALL_DIR:-}" ]; then
    install_dir=$SMDCTL_INSTALL_DIR
else
    [ -n "${HOME:-}" ] || fail 'HOME is unset; set SMDCTL_INSTALL_DIR'
    install_dir="$HOME/.local/bin"
fi

temporary=$(mktemp -d "${TMPDIR:-/tmp}/smdctl-install.XXXXXX")
trap 'rm -rf "$temporary"' EXIT HUP INT TERM

download() {
    source_url=$1
    destination=$2
    case "$source_url" in
        http://127.0.0.1:*|http://localhost:*)
            curl -fsSL "$source_url" -o "$destination"
            ;;
        *)
            curl --proto '=https' --tlsv1.2 -fsSL "$source_url" -o "$destination"
            ;;
    esac
}

if [ -n "${SMDCTL_DOWNLOAD_URL:-}" ]; then
    [ -n "$tag" ] || fail 'set SMDCTL_VERSION when using SMDCTL_DOWNLOAD_URL'
    base_url=${SMDCTL_DOWNLOAD_URL%/}
elif [ -n "$tag" ]; then
    base_url="https://github.com/${REPOSITORY}/releases/download/${tag}"
else
    latest_url="https://github.com/${REPOSITORY}/releases/latest/download/checksums.txt"
    resolved_url=$(curl --proto '=https' --tlsv1.2 -fsS -o /dev/null -w '%{redirect_url}' "$latest_url") || fail 'could not resolve the latest release'
    download_prefix="https://github.com/${REPOSITORY}/releases/download/"
    case "$resolved_url" in
        "$download_prefix"v*/checksums.txt) tag=${resolved_url#"$download_prefix"}; tag=${tag%/checksums.txt} ;;
        *) fail 'could not determine the latest release version' ;;
    esac
    APP_VERSION=${tag#v}
    case "$APP_VERSION" in
        ''|*[!A-Za-z0-9._+-]*) fail 'latest release returned an invalid version' ;;
    esac
    base_url="${download_prefix}${tag%/}"
fi

archive="${APP_NAME}_${APP_VERSION}_linux_${arch}.tar.gz"

say "Downloading smdctl ${APP_VERSION} (linux/${arch})"
download "$base_url/$archive" "$temporary/$archive"
if [ ! -f "$temporary/checksums.txt" ]; then
    download "$base_url/checksums.txt" "$temporary/checksums.txt"
fi
expected=$(awk -v name="$archive" '$2 == name || $2 == "*" name { print $1; exit }' "$temporary/checksums.txt")
[ -n "$expected" ] || fail "checksum for $archive is missing"
if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$temporary/$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$temporary/$archive" | awk '{print $1}')
elif command -v openssl >/dev/null 2>&1; then
    actual=$(openssl dgst -sha256 "$temporary/$archive" | awk '{print $NF}')
else
    fail 'sha256sum, shasum, or openssl is required'
fi
[ "$actual" = "$expected" ] || fail "checksum mismatch for $archive"

mkdir -p "$temporary/unpacked"
tar -xzf "$temporary/$archive" -C "$temporary/unpacked"
[ -f "$temporary/unpacked/$APP_NAME" ] || fail "$APP_NAME is missing from $archive"
mkdir -p "$install_dir"
chmod 755 "$temporary/unpacked/$APP_NAME"
mv "$temporary/unpacked/$APP_NAME" "$install_dir/$APP_NAME.new"
mv "$install_dir/$APP_NAME.new" "$install_dir/$APP_NAME"

say "Installed smdctl ${APP_VERSION} to $install_dir/$APP_NAME"
