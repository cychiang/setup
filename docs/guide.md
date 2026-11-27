# Using setup, step by step

This guide covers both halves of the workflow:

- Part 1 builds and releases bundles. You do this on your own machine, in this repo.
- Part 2 initializes a project from those bundles. You or an agent can do this in any project, locally or in a cloud sandbox.

Every command accepts `--json` where machine-readable output makes sense. Commands never prompt, so agents and CI can run them. Flags can come before or after positional arguments.

## Part 0. Install the binary

```sh
go install github.com/cychiang/setup/cmd/setup@latest
setup --version
```

You can also run `install.sh` from the repo root, which installs a verified release archive into `~/.local/bin`, or build from a clone with `make build` and use `bin/setup`. The README lists the options. In a clone, `make e2e` replays Parts 1 and 2 against the real catalog and fails on any deviation.

`setup` caches third-party sources under `~/.setup/sources`. To put the cache somewhere else, set `SETUP_HOME`.

## Part 1. Build bundles

Run these commands from the repo root. Build-mode commands read the catalog from `./bundles` by default. Pass `--catalog <dir>` to use a different folder.

### 1. Register and pin a third-party source

A source is a git repo that holds skills you want to reuse.

```sh
setup source add anthropic-kwp https://github.com/anthropics/knowledge-work-plugins
setup source sync anthropic-kwp
```

`source add` writes the URL to `bundles/sources.yaml`. `source sync` fetches the repo and pins its current commit in `bundles/sources.lock`. Every bundle that uses the source shares that one pin. Run `setup source sync` with no name to re-pin every source to its latest commit.

Check the source's license before you use its skills. Release archives include the upstream `LICENSE` file next to each third-party skill.

### 2. Create a bundle

```sh
setup bundle new healthcare --layer domain
```

This creates `bundles/healthcare/` with `bundle.yaml`, `skills/` and `instructions/`. The layer must be `base`, `domain` or `type`:

- **domain** is knowledge about a subject. You would still want it if you switched from a notebook to a service.
- **type** is a way of working. You would still want it if you switched subjects.
- **base** is what every project gets. The catalog has exactly one base bundle.

Open `bundles/healthcare/bundle.yaml` and fill in `description`. If another bundle pairs well with this one, list it under `suggests`. This is advice only, and nothing is installed automatically.

### 3. Add skills

You can add a skill you wrote yourself. Create `bundles/<bundle>/skills/<name>/SKILL.md` first, then register it:

```sh
setup bundle add healthcare --local hipaa-basics
```

You can also add a skill from a registered source. Give the source name followed by the path of the skill folder inside that repo:

```sh
setup bundle add finance anthropic-kwp/finance/skills/financial-statements
```

To remove a skill reference:

```sh
setup bundle remove healthcare hipaa-basics
```

`bundle remove` removes the reference only. A local skill folder stays on disk until you delete it.

Every SKILL.md needs YAML frontmatter that follows the Agent Skills spec:

- `name` must match the folder name. It may contain lowercase letters, digits and hyphens, up to 64 characters.
- `description` must be present and at most 1024 characters. It says what the skill does and when to use it.

Keep bundles small. One to three skills per bundle is a good size.

### 4. Add instructions (optional)

Put short Markdown snippets in `bundles/<bundle>/instructions/*.md`. Init places each bundle's snippets in a labelled block in the project's `AGENTS.md` and `CLAUDE.md`. Don't add `setup:begin` markers yourself, because the tool writes them.

### 5. Check the catalog

```sh
setup bundle check            # all bundles
setup bundle check finance    # one bundle
```

`bundle check` fails on any of the following:

- **Structure.** A skill name is used twice, a local skill is missing, a source isn't pinned, or a SKILL.md frontmatter is invalid.
- **Hidden Unicode.** The file contains invisible or bidirectional characters, tag characters, variation selectors, or a byte order mark (BOM) that isn't at the start of the file.
- **Dangerous patterns.** The file contains `curl ... | sh`, a long base64 blob, or a line starting with `` !` ``.

If Gitleaks or skill-scanner is on your PATH, `bundle check` runs it too. When one of them is missing, `check` prints a warning.

To suppress a reviewed finding, add an entry to `bundles/security-allow.yaml`:

```yaml
- skill: some-skill
  sha256: <the "skill sha256" printed with the finding>
  rule: base64-blob
  reason: Embedded test fixture, reviewed 2026-11-27.
```

A suppression is tied to the skill's content hash. If the skill changes, the finding comes back.

### 6. Release a bundle

Bump `version` in `bundle.yaml` whenever the bundle's content changes. Then release:

```sh
setup bundle release finance --out dist
```

This writes `dist/finance-0.1.0.tar.gz` and `dist/finance-0.1.0.tar.gz.sha256`. The same input always produces the same bytes. The release step runs the full check first and writes nothing if the check fails.

- If an external scanner is missing, the release fails. Pass `--allow-missing-scanners` to release anyway.
- Pass `--publish` to also create a GitHub release tagged `finance/v0.1.0`, using `gh`. Projects can then use `--catalog github:cychiang/setup`.

### 7. Commit the catalog

Commit `bundles/` so that the source registry, the pins and the bundle definitions are versioned. Don't commit `dist/`.

## Part 2. Initialize a project

### 1. Choose bundles

Every project gets `base`. Add the domain and type bundles the project needs, for example:

- a finance report project: `base,finance,concise-explaining,diagrams-and-charts,pdf-reports`
- a project that only needs clear explanations: `base,concise-explaining`

### 2. Run init

Run init from the project root:

```sh
# from local release archives
setup init --bundles base,finance,pdf-reports --catalog ~/Projects/github.com/cychiang/setup/dist

# from GitHub releases (needs a token while the repo is private)
export GITHUB_TOKEN=<read-only token with access to cychiang/setup>
setup init --bundles base,finance,pdf-reports --catalog github:cychiang/setup
```

Init writes the following files:

| File | Purpose |
|---|---|
| `setup.yaml` | The catalog and the bundles you chose. |
| `setup.lock` | The version of each bundle, its archive checksum and a hash of every generated file. |
| `.claude/skills/<name>/` | Skill copies for Claude Code. |
| `.agents/skills/<name>/` | Skill copies for Codex and Cursor. |
| `AGENTS.md`, `CLAUDE.md` | One managed block per bundle, between `<!-- setup:begin <bundle> -->` and `<!-- setup:end <bundle> -->`. Text outside the blocks is never changed. |

Running init again with the same bundles changes nothing.

### 3. Commit the result

```sh
git add setup.yaml setup.lock .claude .agents AGENTS.md CLAUDE.md
git commit -m "Set up agent bundles"
```

Committing the generated files is what makes the project work in a fresh clone or a cloud sandbox with no install step.

### 4. Open your agent

Start Claude Code, Codex or Cursor in the project. The skills are loaded at session start.

If an agent ran `setup init` partway through a session, init prints an activation note that lists the new files. Claude Code and Codex usually pick up new skills live. Cursor fixes its skill list when a chat starts, and none of the three reloads instruction files mid-session. So either read the listed files in that session, or start a new session or chat.

### 5. Let the agent manage bundles

The `base` bundle installs the `setup-workbench` skill. You can ask the agent things like "this project also needs PDF reports". The agent proposes the bundles that fit, waits for you to confirm, and then runs `setup add`.

## Part 3. Keep projects current

```sh
setup outdated          # lists newer bundle versions; exit code 1 if any exist
setup update            # update all bundles
setup update finance    # update one bundle
setup add diagrams-and-charts
setup remove pdf-reports
```

Before `update`, `add` or `remove` writes anything, it compares every generated file with `setup.lock`. If you edited a generated file by hand, the command stops and names the file. Choose one of these:

- To keep your change, move it into the bundle and release a new version, or put it outside the managed markers.
- To discard your change, rerun with `--force`.

## Part 4. Guard projects in CI

Add this step to the project's CI:

```sh
setup check
```

It exits 1 and names the file when any generated file differs from `setup.lock`. That catches hand edits and partial updates before they merge.

## Part 5. Cloud agents

A cloud agent clones the repo into a fresh sandbox. You can set up a project for one in either of two ways.

- **Commit the generated files (recommended).** Run init locally, as in Part 2, and commit. The cloud agent sees everything in the clone. It needs no binary, no token and no network access.
- **Let the agent initialize the project.** Tell the agent which bundles to use. The agent then installs the `setup` binary in the sandbox and runs `setup init --bundles ... --catalog github:cychiang/setup --json`. While the repo is private, add a read-only `GITHUB_TOKEN` to the cloud environment's secrets. Prefer the environment's setup step (setup script, install script or `environment.json`) over running init in the chat, because instructions only load at session start.

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `drift: <file>: modified` | A generated file was edited. Run `setup update --force` to restore it, or move the change into the bundle. |
| `conflict (hand-edited): <file>` | `update`, `add` or `remove` refused to overwrite your edit. The fix is the same as for drift. |
| `warning: external scanner ... not found on PATH` | Install Gitleaks or skill-scanner, or pass `--allow-missing-scanners` when you release. |
| A skill doesn't appear in the agent | Start a new session or chat. In Claude Code, `/reload-skills` also works. |
| A skill name collision during `bundle check` or `init` | Two bundles ship a skill with the same name. Rename one of them. Names must be unique across the catalog. |
