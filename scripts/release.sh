#!/bin/sh
set -eu

VERSION=${1:?usage: release.sh <version>}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

EPOCH=${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD)}
DATE=$(date -u -r "$EPOCH" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$EPOCH" +%Y-%m-%dT%H:%M:%SZ)
COMMIT=$(git rev-parse HEAD)
PKG=github.com/cychiang/setup/internal/version
LDFLAGS="-s -w -X $PKG.Version=$VERSION -X $PKG.Commit=$COMMIT -X $PKG.Date=$DATE"

DIST="$ROOT/dist"
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT
rm -rf "$DIST"
mkdir "$DIST"

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
	os=${target%/*}
	arch=${target#*/}
	bin="$STAGE/setup_${os}_${arch}"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$bin" ./cmd/setup
	chmod 755 "$bin"
	go run scripts/tarball.go "$EPOCH" "$DIST/setup_${VERSION}_${os}_${arch}.tar.gz" "setup=$bin" "README.md=README.md"
done

(cd "$DIST" && for f in setup_*.tar.gz; do
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$f"; else shasum -a 256 "$f"; fi
done >checksums.txt)
cat "$DIST/checksums.txt"
