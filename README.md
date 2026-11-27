# setup

> [!WARNING]
> **Experimental.** This is a personal project for exploring how to manage agent skills. Commands, file formats and bundle contents may change without notice or migration, and there is no stable release yet. Review any skill before you let an agent use it, especially third-party ones. Use at your own risk.

`setup` builds reusable bundles of agent skills and installs them into projects for Claude Code, Codex and Cursor.

A bundle covers one concern, such as finance knowledge or producing PDF reports. A project combines bundles, for example `base + finance + pdf-reports`. After `setup init`, any coding agent opened in the project has the skills and instructions it needs. This works for local agents and for cloud agents that only see a clone of the repo.

## How it works

`setup` has two modes. A release archive connects them.

```
 BUILD MODE (your machine)                 INIT MODE (any agent, any sandbox)
 register sources, pick skills             setup init --bundles base,finance,...
 bundle check (names, pins, security)        -> download released archives
 bundle release ── finance-0.1.0.tar.gz ──▶  -> verify checksums
                                             -> write skills + AGENTS.md, CLAUDE.md
                                             -> setup.lock
```

- **Bundles** belong to one of three layers. `base` goes into every project. A **domain** bundle holds subject knowledge, such as `finance`. A **type** bundle covers a way of working, such as `pdf-reports`. A project uses `base` plus any number of domain bundles and any number of type bundles.
- **Skills** in a bundle are either your own or individual skills picked from registered third-party repos. Every third-party source is pinned to one commit, which all bundles share.
- **Releases** are reproducible `.tar.gz` archives with a checksum. A security gate must pass before a release is written.
- **Projects** get real copies of each skill in `.claude/skills/` (Claude Code) and `.agents/skills/` (Codex and Cursor). Instructions go into managed blocks in `AGENTS.md`, the single source for every agent. `CLAUDE.md` holds only an `@AGENTS.md` import, so Claude Code loads each instruction once. Everything is pinned in `setup.lock`. Commit these files, so that a fresh clone works with no install step.
- **Nothing is written at user scope.** `setup` never touches `~/.claude`, `~/.agents` or `~/.cursor`, so projects cannot affect each other.

## Install

Pick one. All three give you the same binary, and `setup --version` prints its version, commit and build date.

```sh
go install github.com/cychiang/setup/cmd/setup@latest    # needs Go 1.27 or later
```

```sh
curl -fsSL https://raw.githubusercontent.com/cychiang/setup/main/install.sh | sh
```

`install.sh` downloads a release archive, verifies its checksum and installs `setup` into `~/.local/bin`. Set `VERSION=v0.1.0` to pin a release and `SETUP_INSTALL_DIR` to change the target. No credentials are needed. `install.sh` also reads `GITHUB_TOKEN` or `GH_TOKEN` if set, which raises the GitHub API rate limit.

To build from a clone:

```sh
git clone git@github.com:cychiang/setup.git
cd setup
make build        # writes bin/setup
make install      # or install into your GOBIN
```

The binary has no runtime dependencies.

## Develop

`make help` lists the targets. Only Go, git, tar and a POSIX shell are required.

| Command | What it does |
|---|---|
| `make build` | Builds `bin/setup` with the version stamped in. |
| `make run ARGS="check --json"` | Builds and runs `bin/setup` with those arguments. |
| `make test` | Runs the unit and in-process end-to-end tests. |
| `make lint` | Checks gofmt, runs `go vet` and staticcheck. |
| `make vuln` | Runs govulncheck. |
| `make e2e` | Replays the whole workflow against the real catalog in `bundles/`: sync, check, release, init, drift, update. Needs network access to GitHub. |
| `make dist VERSION=v0.1.0` | Cross-compiles release archives and `checksums.txt` into `dist/`. |
| `make release VERSION=v0.1.0` | Runs the tests, builds `dist/`, and publishes a GitHub release with `gh`. |
| `make clean` | Removes `bin/` and `dist/`. |

CI runs `lint`, `test` (Linux and macOS), `vuln` and `e2e` on every pull request. Pushing a tag such as `v0.1.0` runs the tests and publishes the release archives, and `make release VERSION=v0.1.0` does the same from your machine. Bundle releases use tags like `finance/v0.1.0` and do not trigger a tool release.

## Quick start

To build the bundles and initialize a project from the local archives:

```sh
cd setup
setup source sync                       # fetch pinned third-party sources
setup bundle check                      # validate every bundle
for b in base finance concise-explaining diagrams-and-charts pdf-reports; do
  setup bundle release "$b" --out dist --allow-missing-scanners
done

cd ~/Projects/my-report-project
setup init --bundles base,finance,pdf-reports --catalog ~/path/to/setup/dist
setup check                             # prints "ok"
```

Open Claude Code, Codex or Cursor in the project. The skills are available from the first turn.

Without `--catalog`, init uses `SETUP_CATALOG` if it is set, and otherwise the built-in default `github:cychiang/setup`. Bundles must be published there with `setup bundle release <bundle> --publish` before the default works.

To see which third-party skills you can add, and which bundles exist:

```sh
setup skills --source anthropic-kwp --query finance   # REF, NAME, USED BY, DESCRIPTION
setup skills --markdown > skills.md                   # full catalog as a Markdown table
setup bundle list                                     # bundles by layer, with their skills
setup bundle add finance <REF>                        # paste a REF from the list
setup list                                            # bundles in the project's catalog, and which are installed
setup list --json                                     # the same as data, for agents
```

`setup skills` lists every skill in each pinned source plus your own local skills. `--json` gives the full data.

For the full walkthrough, which covers writing your own bundle, updates, CI and cloud agents, read [docs/guide.md](docs/guide.md).

## Iterate on bundles with dev mode

A released bundle only reaches a project after a release, a version bump and `setup update`. To try edits right away, link a project to your working catalog:

```sh
cd ~/Projects/my-report-project
setup init --catalog ~/path/to/setup/bundles   # or `setup link <dir>` in an initialized project
# edit a skill under bundles/, then:
setup update                                   # rebuilds from your checkout
setup unlink                                   # back to released bundles (the default catalog, or --catalog <dist>)
```

The link lives in `setup.local.yaml`, which init adds to `.gitignore`. Dev builds run the same security gate as a release and use the third-party commits pinned in `sources.lock`. The lock marks them `source: dev`, so `setup check` fails until you run `setup unlink`, and a dev-built install can't be committed by accident. The tool only reads your catalog. Releases stay the only way to share.

## Bundles in this catalog

| Bundle | Layer | What it gives the agent |
|---|---|---|
| `base` | base | `setup-workbench`, a skill for adding, removing and updating bundles from inside the agent. Also general conventions. |
| `finance` | domain | `money-precision` (decimals, rounding, currency) and `financial-statements` (from `anthropics/knowledge-work-plugins`, Apache-2.0). |
| `concise-explaining` | type | Answers that lead with the result and leave out filler. |
| `diagrams-and-charts` | type | Choosing and drawing the right diagram or chart, with labelled units and sourced data. |
| `pdf-reports` | type | Structured PDF reports with cited sources, built with Typst. Suggests `diagrams-and-charts`. |

## Repository layout

```
cmd/setup/   main package of the setup binary
internal/    the Go packages behind it
bundles/     the catalog: sources.yaml, sources.lock, security-allow.yaml, one folder per bundle
docs/        design.md (decisions and formats), guide.md (step-by-step usage)
scripts/     e2e.sh (full workflow check) and release.sh (reproducible release archives)
install.sh   installer for release archives
Makefile     build, test, lint, e2e and dist targets
go.mod       Go module github.com/cychiang/setup
```

## Status

This is an MVP. The following are not done yet:

- tracked forks of third-party skills
- a review step for third-party changes
- MCP servers and hooks in bundles
- update notifications
- an interactive bundle picker

Known gaps:

- The external scanners (Gitleaks, skill-scanner) are optional. They have not been run as part of a release yet.
- GitHub release catalogs (`--catalog github:owner/repo`) have been tested only against a local server.
- Archives store fixed file modes, so a script inside a skill loses its executable bit.

See [docs/design.md](docs/design.md) for the full design and the roadmap.
