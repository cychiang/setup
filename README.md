# setup

`setup` builds reusable bundles of agent skills and installs them into projects for Claude Code, Codex and Cursor.

A bundle covers one concern, such as finance knowledge or producing PDF reports. A project combines bundles, for example `base + finance + pdf-reports`. After `setup init`, any coding agent opened in the project has the skills and instructions it needs. This works for local agents and for cloud agents that only see a clone of the repo.

## How it works

`setup` has two modes. A release archive connects them.

```
 BUILD MODE (your machine)                 INIT MODE (any agent, any sandbox)
 register sources, pick skills             setup init --bundles base,finance,...
 bundle check (names, pins, security)        -> download released archives
 bundle release ── finance-0.1.0.tar.gz ──▶  -> verify checksums
                                             -> write skills + AGENTS.md / CLAUDE.md
                                             -> setup.lock
```

- **Bundles** belong to one of three layers. `base` goes into every project. A **domain** bundle holds subject knowledge, such as `finance`. A **type** bundle covers a way of working, such as `pdf-reports`. A project uses `base` plus any number of domain bundles and any number of type bundles.
- **Skills** in a bundle are either your own or individual skills picked from registered third-party repos. Every third-party source is pinned to one commit, which all bundles share.
- **Releases** are reproducible `.tar.gz` archives with a checksum. A security gate must pass before a release is written.
- **Projects** get real copies of each skill in `.claude/skills/` (Claude Code) and `.agents/skills/` (Codex and Cursor). Instructions go into managed blocks in `AGENTS.md` and `CLAUDE.md`. Everything is pinned in `setup.lock`. Commit these files, so that a fresh clone works with no install step.
- **Nothing is written at user scope.** `setup` never touches `~/.claude`, `~/.agents` or `~/.cursor`, so projects cannot affect each other.

## Install

```sh
git clone git@github.com:cychiang/setup.git
cd setup/tool
go build -o ~/bin/setup ./cmd/setup    # any directory on your PATH
setup --help
```

This needs Go 1.27 or later. The binary has no runtime dependencies.

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

For the full walkthrough, which covers writing your own bundle, updates, CI and cloud agents, read [docs/guide.md](docs/guide.md).

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
tool/       Go module for the setup binary (cmd/setup, internal/...)
bundles/    the catalog: sources.yaml, sources.lock, security-allow.yaml, one folder per bundle
docs/       design.md (decisions and formats), guide.md (step-by-step usage)
```

## Status

This is an MVP. The repo is private, and the following are not done yet:

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
