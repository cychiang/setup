# setup: design

`setup` is a tool for building reusable bundles of agent skills and for initializing projects from those bundles. A bundle is a curated set of skills and instructions for one concern. A project combines several bundles, for example `base + finance + pdf-reports`. Once a project is initialized, any coding agent that opens it, local or cloud, has everything it needs.

This document records the decisions from the design interview held on 2026-11-26/27. Each decision gives the choice and the reason for it.

## Goals

- Build bundles from existing skills: your own skills plus individual skills picked from third-party repos.
- Initialize any project from a set of bundles, in any agent, including cloud sandboxes.
- Keep projects pinned, isolated from each other, and easy to update.
- Support Claude Code, Codex and Cursor.

## Non-goals for the MVP

The following are deferred to later stages:

- Tracked forks with three-way merge.
- `source approve`, a step where you review new or changed third-party skills before bundles can use them.
- Agent-assisted research for new sources.
- MCP servers and hooks. The bundle format reserves `mcp/` and `hooks/` folders, but the MVP never populates or processes them.
- Session-start update notification and update PRs.
- Setup snippets for each agent's pre-session setup step.
- `agent` workspace bundles and repo creation.
- The interactive picker. The MVP accepts bundles through flags only.
- An LLM judge as part of the security scan.
- Any domain other than finance.
- Going public.

## Concepts

### Bundle

A bundle is a folder in the catalog. It describes exactly one concern and belongs to one layer:

| Layer | Meaning | Test |
|---|---|---|
| `base` | Included in every project. | Would you want it regardless of the subject or the kind of work? |
| `domain` | Knowledge about a subject, such as finance. | Would you still want it if you switched from a notebook to a service? |
| `type` | A way of working, such as producing PDF reports. | Would you still want it if you switched domains? |

A project takes `base` plus any number of domain bundles plus any number of type bundles.

A bundle contains:

- **Skills.** These are SKILL.md folders. A skill is either one you wrote (stored in the bundle) or a reference to a skill in a registered third-party source, pinned through `sources.lock`.
- **Instructions.** These are Markdown snippets that are compiled into AGENTS.md.
- **`mcp/` and `hooks/`.** These folders are reserved. The MVP doesn't use them.

A bundle never includes another bundle. A bundle can list `suggests`, which names other bundles that pair well with it (for example, `pdf-reports` suggests `diagrams-and-charts`). This is advice only: the workbench skill mentions the suggestion, and nothing is installed automatically.

### Catalog

The catalog is the folder of bundles plus `sources.yaml` (the registered sources) and `sources.lock` (one pinned commit for each source, shared by every bundle). Because bundles share one pin for each source, two bundles can never disagree about the version of a source.

The tool finds the catalog only through a setting (`--catalog` or a config file). It never uses a path relative to its own location. For now, the catalog lives in this repo under `bundles/`.

### Two modes

```
 BUILD MODE (your machine)                  INIT MODE (any agent, any sandbox)
 source add / sync                          setup init --bundles base,finance,...
 bundle new / add / remove                    -> download released archives
 bundle check (names, pins, rules, scan)      -> verify checksums
 bundle release ──── finance/v1.0.0 ────▶     -> merge, then write files per agent
                    (self-contained archive)  -> setup.lock
```

The released archive is the only interface between the two modes. Init mode has no knowledge of sources, forks, APM or scanners.

## Decisions

| # | Decision | Reason |
|---|---|---|
| 1 | Support Claude Code, Codex and Cursor. | You use all three. |
| 2 | Bundles mix your own skills with individual skills picked from third-party sources. | You need to reuse existing skills and fill the gaps with your own. |
| 3 | Tracked forks keep a record of their upstream (deferred). | You can see when you fall behind upstream without having to merge patches into prose. |
| 4 | Layers are base, domain and type. Bundles don't nest. | The layers are separate axes, and the design avoids a dependency graph between bundles. |
| 5 | Projects are pinned, and you are told about updates. | Results are reproducible, and you control when updates happen. |
| 6 | The tool is a CLI, plus a workbench skill that calls it. | The CLI is deterministic and can be tested. The skill makes it convenient to use from an agent. |
| 7 | APM is used in build mode only. | APM resolves sources and per-agent layouts. Keeping it out of init mode keeps init a single binary with no dependencies. |
| 8 | Bundles can contain skills, instructions, MCP and hooks. The MVP uses only skills and instructions. | A domain bundle needs more than skills, but the MVP stays minimal. |
| 9 | A project can use any number of domains and types. | Real projects span more than one domain or type. |
| 10 | Bundle selection works through flags, an inferring agent, and later a picker. All three produce explicit bundle names. | Nothing about the selection is hidden, and `setup.lock` shows what was chosen. |
| 11 | Each bundle has its own version, for example `finance/v1.2.0`. | Bundles are released on their own schedules. |
| 12 | The tool is separate from the catalog, and both are private for now. | Other people can use the tool with their own catalog later. |
| 13 | One repo for now. The Go module sits at the repo root next to `bundles/`. | Early on the tool and the bundles change together. |
| 14 | Sources are pinned once for the whole catalog. | This prevents version conflicts when a project combines bundles. |
| 15 | Search covers registered sources only. The agent proposes new sources (deferred). | Only trusted text can reach agents. |
| 16 | Forks are updated with a copier-style three-way merge (deferred). | Most upstream edits merge cleanly, and conflicts are visible. |
| 17 | Merge rules are fixed (see below). | Most conflicts are caught when the bundle is built rather than in a project. |
| 18 | Implementation language is Go. | It produces one clean binary that ships easily. |
| 19 | The tool never writes to user scope. The workbench skill lives in `base`. | Projects can't affect each other. |
| 20 | Initialization starts with `base`. The agent then infers further bundles from inside the project. | No user-level skill is needed before init. |
| 21 | Generated files are real copies and are committed. Secrets are referenced as `${ENV}`. | A clone works in any cloud sandbox. Cursor doesn't reliably follow symlinks. |
| 22 | Releases are self-contained archives with checksums. | Init in a sandbox is one download per bundle, and upstream changes can't break it. |
| 23 | `bundle release` runs a security gate. | Skills are text that steers agents, so they are checked before release. |
| 24 | Init should run before the session through each agent's setup step (deferred). Init can also run mid-session and print an activation note. | Instructions and MCP changes aren't picked up mid-session. |
| 25 | Dev mode links a project to a local source catalog. The link is stored only in the gitignored `setup.local.yaml`. | You can iterate on bundles without a release and version bump, and the public repo never receives a local path. |
| 26 | Dev bundles are built in memory through the build code and marked `source: dev` in `setup.lock`. `setup check` fails while any exist. | A dev-built install can't pass CI or be shared by accident, even on a clone without `setup.local.yaml`. |
| 27 | Dev builds run the release security gate. External scanners only warn. Third-party skills use the `sources.lock` pins. | Dev mode is not a way around the gate or the pins. |
| 28 | Bundle instructions live only in AGENTS.md. CLAUDE.md holds a plain `@AGENTS.md` line and no markers. | Claude Code loads CLAUDE.md in preference to AGENTS.md and expands `@path` imports, so writing blocks to both files made Claude read every instruction twice. A created CLAUDE.md is tracked whole in `setup.lock`. A CLAUDE.md the user wrote gets the line prepended once and stays untracked. One that already imports AGENTS.md is untouched. Older installs migrate on the next init or update. |
| 29 | `setup list` shows the bundles in the project's active catalog and what is installed. | The agent proposes bundles from live catalog data, not a table baked into a skill that goes stale. Released catalogs are read from the latest manifest, so the manifest carries the description. |

### Merge rules (init mode)

| Content | Rule |
|---|---|
| Skills | Every skill from every selected bundle is installed. Skill names must be unique across the catalog. `bundle check` enforces this, and init also rejects collisions. |
| Instructions | Snippets are joined in layer order (base, then domains, then types, then the project's own). Each bundle's snippet goes in its own labelled block of AGENTS.md. When instructions contradict, precedence is project, then domain, then type, then base. |
| Project-level | `setup.yaml` can disable any bundle item by name, for example `disable: [finance/skills/x]`. |

### Security gate (build mode, MVP)

`bundle check` and `bundle release` run these checks, all locally:

1. Built-in Go checks:
   - hidden Unicode (Cf, tag characters U+E0000–E007F, bidi overrides and isolates, variation selectors, mid-file BOM)
   - SKILL.md frontmatter validity (`name` matches the folder, `description` is present)
   - dangerous patterns (`curl … | sh`, long base64 blobs, `` !`cmd` `` lines)
   - skill links (medium, never blocking): a relative Markdown link that leaves the skill folder or points at a missing file
2. Gitleaks (secrets), if it is installed.
3. Cisco skill-scanner (`--fail-on-severity high`, with no cloud options), if it is installed.

A high-severity finding blocks the release. To suppress a finding, add an entry to `security-allow.yaml` with a written reason. The entry is tied to the skill and to its content hash, so it stops applying when the content changes. If an external scanner is missing, `check` warns, and `release` fails unless `--allow-missing-scanners` is passed.

## Files

### Repository

```
go.mod, go.sum        # module github.com/cychiang/setup
cmd/setup/            # main package
internal/             # archive, build, catalog, cli, fetch, project, scan, store, version, ...
bundles/              # the catalog (below)
scripts/              # e2e.sh, release.sh, tarball.go
install.sh            # installs a release archive after verifying its checksum
Makefile              # build, run, test, lint, vuln, e2e, dist, install
```

Tool releases are tagged `vX.Y.Z` and publish `setup_<version>_<os>_<arch>.tar.gz` archives plus `checksums.txt`. Bundle releases are tagged `<bundle>/vX.Y.Z`. CI (`.github/workflows/ci.yml`) runs lint, test, vuln and e2e on every pull request. Pushing a `vX.Y.Z` tag runs `.github/workflows/release.yml`, or run `make release VERSION=vX.Y.Z` locally. The release workflow runs only on tags that start with `v`, so a bundle tag never triggers it.

### Catalog (`bundles/`)

```
bundles/
  sources.yaml            # registered sources: name -> git URL
  sources.lock            # name -> pinned commit
  security-allow.yaml     # suppressions (skill, hash, finding, reason)
  base/
    bundle.yaml
    skills/<name>/SKILL.md
    instructions/*.md
  finance/ ...
```

`bundle.yaml`:

```yaml
name: finance
layer: domain            # base | domain | type
version: 1.0.0
description: Finance domain — money precision, statements, terminology.
suggests: []
skills:
  - local: money-precision                 # bundles/finance/skills/money-precision
  - source: anthropic-fs                   # from sources.yaml, pinned by sources.lock
    path: plugins/financial-analysis/skills/comps
```

### Release archive

The archive is named `<bundle>-<version>.tar.gz` and comes with a `.sha256` file. It is published as a GitHub Release tagged `<bundle>/v<version>`. Contents:

```
manifest.json        # name, layer, version, description, suggests, skills[] with content hashes
skills/<name>/...    # fully resolved: own skills and third-party skills, ready to copy
instructions/*.md
```

### Project

```
setup.yaml       # catalog location + selected bundles (+ disable list)
setup.lock       # bundle -> version + archive sha256 + per-file hashes (dev bundles: source dev, no sha256)
setup.local.yaml # dev_catalog: absolute path of a linked source catalog; gitignored, never committed
.claude/skills/<name>/...     # copies, for Claude Code
.agents/skills/<name>/...     # copies, for Codex and Cursor
AGENTS.md                     # one managed block per bundle, between <!-- setup:begin <bundle> --> markers
CLAUDE.md                     # `@AGENTS.md` import; whole file tracked only when setup created it
```

Instructions are written into managed blocks of AGENTS.md, so text outside the markers is never modified. CLAUDE.md is only the line `@AGENTS.md`. `setup check` fails if any generated file differs from what `setup.lock` describes.

## CLI (MVP)

Build mode:

```
setup source add <name> <git-url>
setup source sync [<name>]            # fetch and re-pin in sources.lock
setup skills [--source <n>] [--query <t>] [--json] [--markdown]   # list pickable skills with their refs
setup bundle list [--json]
setup bundle new <name> --layer <l>
setup bundle add <bundle> <source>/<path> | --local <name>
setup bundle remove <bundle> <skill>
setup bundle check [<bundle>]
setup bundle release <bundle> [--publish]
```

Init mode:

```
setup init --bundles base,finance[,…] [--catalog <repo|dir>] [--yes] [--json]
setup add <bundle>… | setup remove <bundle>…
setup list [--json]           # bundles in the active catalog: layer, latest version, description, skills, installed
setup outdated [--json]       # exit code 1 if updates are available
setup update [<bundle>…]
setup check                   # CI drift gate; also fails while dev bundles are in the lock
setup link <dir>              # dev mode: build bundles from a local source catalog (init --catalog <dir> does the same)
setup unlink [--catalog <c>]  # leave dev mode, reinstall released bundles
```

Every command is non-interactive when `--yes` is passed or when stdin is not a TTY, and every command supports `--json`. When `init` runs inside an agent session, it prints an activation note that lists the new skill and instruction paths for the agent to read now.

## MVP content

Each bundle has 1–3 skills.

| Bundle | Layer | Purpose |
|---|---|---|
| `base` | base | The workbench skill (how to use `setup` from inside an agent), plus general conventions. |
| `finance` | domain | Money precision, financial statement basics, and terminology. |
| `concise-explaining` | type | Explain in less: lead with the answer, use plain words, cut filler. |
| `diagrams-and-charts` | type | Choose and draw the right visual for architecture, flows and numbers. |
| `pdf-reports` | type | Produce a structured PDF report that cites its sources, embeds charts, and can be regenerated. Suggests `diagrams-and-charts`. |

The first test project is `stock-analysis`, initialized as `base + finance + concise-explaining + diagrams-and-charts + pdf-reports`, followed by asking an agent for a report.

## Tests to run first

1. Whether APM resolves a dependency-only bundle into each agent's layout. If it can't, build mode resolves sources directly with git, which the design allows.
2. Whether Cursor shows skills twice when they are written to both `.claude/skills` and `.agents/skills`.
3. Whether a Claude Code cloud session can download from the private `setup` releases.

## Catalog resolution

A project's catalog is resolved in this order: the dev link in `setup.local.yaml`, then `--catalog`, then `catalog:` in `setup.yaml`, then the `SETUP_CATALOG` environment variable, then the built-in default `github:cychiang/setup`. Init records the released catalog it used in `setup.yaml`, so a project keeps its catalog when the default changes.
