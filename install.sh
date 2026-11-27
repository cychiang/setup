#!/bin/sh
# Install the setup binary from GitHub Releases of cychiang/setup.
# Env: VERSION (default latest), SETUP_INSTALL_DIR (default ~/.local/bin),
#      GITHUB_TOKEN or GH_TOKEN (private repo), SETUP_RELEASE_DIR (install from local dist/).
set -eu

REPO=cychiang/setup
API=https://api.github.com/repos/$REPO
DEST=${SETUP_INSTALL_DIR:-$HOME/.local/bin}
TOKEN=${GITHUB_TOKEN:-${GH_TOKEN:-}}

die() { printf 'install.sh: %s\n' "$1" >&2; exit 1; }

case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) die "unsupported OS $(uname -s)" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) die "unsupported architecture $(uname -m)" ;;
esac

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d ' ' -f 1
	elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | cut -d ' ' -f 1
	else die "need sha256sum or shasum"; fi
}

curl_api() {
	if [ -n "$TOKEN" ]; then
		curl -fsSL -H "Authorization: Bearer $TOKEN" -H "Accept: $1" "$2"
	else
		curl -fsSL -H "Accept: $1" "$2"
	fi
}

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

VERSION=${VERSION:-}
if [ -n "${SETUP_RELEASE_DIR:-}" ]; then
	[ -n "$VERSION" ] || die "VERSION is required with SETUP_RELEASE_DIR"
else
	if [ -z "$VERSION" ]; then
		VERSION=$(curl_api application/vnd.github+json "$API/releases/latest" |
			sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
		[ -n "$VERSION" ] || die "could not determine the latest release"
	fi
fi

archive=setup_${VERSION}_${os}_${arch}.tar.gz

fetch_asset() {
	name=$1
	if [ -n "${SETUP_RELEASE_DIR:-}" ]; then
		cp "$SETUP_RELEASE_DIR/$name" "$WORK/$name" || die "missing $SETUP_RELEASE_DIR/$name"
	elif [ -n "$TOKEN" ]; then
		release=$(curl_api application/vnd.github+json "$API/releases/tags/$VERSION") || die "release $VERSION not found"
		id=$(printf '%s' "$release" | tr '{' '\n' |
			awk -v n="\"name\": *\"$name\"" '$0 ~ n { print }' |
			sed -n 's/.*"id"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' | head -n 1)
		[ -n "$id" ] || die "asset $name not found in release $VERSION"
		curl_api application/octet-stream "$API/releases/assets/$id" >"$WORK/$name" || die "download of $name failed"
	else
		curl -fsSL -o "$WORK/$name" "https://github.com/$REPO/releases/download/$VERSION/$name" || die "download of $name failed"
	fi
}

fetch_asset "$archive"
fetch_asset checksums.txt

want=$(awk -v f="$archive" '$2 == f { print $1 }' "$WORK/checksums.txt")
[ -n "$want" ] || die "$archive is not listed in checksums.txt"
got=$(sha256_of "$WORK/$archive")
[ "$want" = "$got" ] || die "checksum mismatch for $archive (want $want, got $got)"

mkdir "$WORK/x"
tar -xzf "$WORK/$archive" -C "$WORK/x" setup
mkdir -p "$DEST"
cp "$WORK/x/setup" "$DEST/setup.tmp.$$"
chmod 755 "$DEST/setup.tmp.$$"
mv "$DEST/setup.tmp.$$" "$DEST/setup"
printf 'installed setup %s to %s/setup\n' "$VERSION" "$DEST"
case ":$PATH:" in
	*":$DEST:"*) ;;
	*) printf 'note: %s is not on your PATH\n' "$DEST" ;;
esac
