#!/bin/sh
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
export SETUP_HOME="$WORK/home"
SETUP="$WORK/setup"
ALL="base finance concise-explaining diagrams-and-charts pdf-reports"
ALL_CSV=$(echo $ALL | tr ' ' ',')

pass() { printf 'ok   %s\n' "$1"; }
fail() { printf 'FAIL %s\n' "$1" >&2; exit 1; }

expect_code() {
	want=$1; label=$2; shift 2
	set +e
	"$@" >"$WORK/out.txt" 2>&1
	got=$?
	set -e
	[ "$got" -eq "$want" ] || { cat "$WORK/out.txt" >&2; fail "$label: exit $got, want $want"; }
	pass "$label (exit $want)"
}

cd "$ROOT"
go build -trimpath -o "$SETUP" ./cmd/setup || fail "build binary"
pass "build binary"

expect_code 0 "source sync" "$SETUP" source sync
expect_code 0 "bundle check" "$SETUP" bundle check

mkdir "$WORK/A" "$WORK/B"
for b in $ALL; do
	"$SETUP" bundle release "$b" --out "$WORK/A" --allow-missing-scanners >"$WORK/out.txt" 2>&1 \
		|| { cat "$WORK/out.txt" >&2; fail "release $b"; }
done
pass "release five bundles"
for b in base finance; do
	"$SETUP" bundle release "$b" --out "$WORK/B" --allow-missing-scanners >"$WORK/out.txt" 2>&1 \
		|| { cat "$WORK/out.txt" >&2; fail "re-release $b"; }
done
n=0
for f in "$WORK"/B/*; do
	cmp "$f" "$WORK/A/$(basename "$f")" || fail "archive $(basename "$f") differs between releases"
	n=$((n + 1))
done
[ "$n" -ge 2 ] || fail "expected archives in B, found $n"
pass "base and finance archives byte-identical across releases ($n files)"

PROJ="$WORK/proj"
mkdir "$PROJ"
git -C "$PROJ" init -q
printf 'my own line\n' >"$PROJ/AGENTS.md"
cd "$PROJ"
expect_code 0 "init" "$SETUP" init --bundles "$ALL_CSV" --catalog "$WORK/A" --yes

grep -q '^my own line$' AGENTS.md || fail "user line lost from AGENTS.md"
pass "user line survives in AGENTS.md"
for dir in .claude/skills .agents/skills; do
	for s in setup-workbench money-precision financial-statements concise-explaining diagrams-and-charts pdf-reports; do
		[ -f "$dir/$s/SKILL.md" ] || fail "$dir/$s/SKILL.md missing"
	done
	count=$(find "$dir" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
	[ "$count" -eq 6 ] || fail "$dir has $count skills, want 6"
	pass "$dir has the 6 expected skills"
done

expect_code 0 "check after init" "$SETUP" check

find . -path ./.git -prune -o -type f -exec touch -t 200001010000 {} +
touch -t 200001010001 "$WORK/marker"
expect_code 0 "rerun init" "$SETUP" init --bundles "$ALL_CSV" --catalog "$WORK/A" --yes
touched=$(find . -path ./.git -prune -o -type f -newer "$WORK/marker" -print)
[ -z "$touched" ] || fail "rerun init touched: $touched"
pass "rerun init changed no file mtimes"

DRIFTED=.claude/skills/money-precision/SKILL.md
printf '\nlocal edit\n' >>"$DRIFTED"
expect_code 1 "check detects drift" "$SETUP" check
grep -q "$DRIFTED" "$WORK/out.txt" || { cat "$WORK/out.txt" >&2; fail "check output does not name $DRIFTED"; }
pass "check names $DRIFTED"

expect_code 1 "update refuses without --force" "$SETUP" update
expect_code 0 "update --force" "$SETUP" update --force
expect_code 0 "check after update --force" "$SETUP" check
