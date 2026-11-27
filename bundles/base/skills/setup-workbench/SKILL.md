---
name: setup-workbench
description: "Use the setup CLI from inside a project to add, remove, update and check skill bundles (base, domain and type layers). Use when the project has a setup.yaml or setup.lock, when the user asks for new skills, instructions or capabilities for this project, when a task falls in a domain or way of working the project has no bundle for, or right after setup init or setup add ran in this session."
---

# Setup workbench

`setup` installs bundles of agent skills and instructions into a project. A project always has `base`, plus any number of `domain` bundles (a subject, such as finance) and `type` bundles (a way of working, such as PDF reports).

## Files

- `setup.yaml` lists the catalog and the selected bundles. It is the source of truth for what the project uses.
- `setup.lock` pins each bundle to a version, an archive sha256 and per-file hashes.
- `.claude/skills/<name>/` and `.agents/skills/<name>/` hold generated copies of each skill.
- `AGENTS.md` holds one generated block per bundle between `<!-- setup:begin <bundle> -->` markers. It is the single source of instructions for every agent.
- `CLAUDE.md` only imports it with an `@AGENTS.md` line. Put project rules in `AGENTS.md` outside the markers.

Never edit generated skill copies or text inside the markers. `setup check` fails on any drift. Change behavior by changing bundles, or write project rules outside the markers.

## Commands

Every command accepts `--json`. Pass `--yes` to run without prompts.

| Command | What it does |
|---|---|
| `setup init --bundles base,<more> [--catalog <repo-or-dir>] --yes` | Initialize the project from the named bundles. |
| `setup list --json` | List the bundles in the project's catalog with layer, latest version, description, skills and installed state. Works before init too. |
| `setup add <bundle>...` | Add bundles to an initialized project. |
| `setup remove <bundle>...` | Remove bundles and their files. |
| `setup outdated --json` | List bundles with newer versions. Exit code 1 means updates exist. |
| `setup update [<bundle>...]` | Update all bundles, or the named ones. |
| `setup check` | Exit non-zero if generated files differ from `setup.lock`. Run it in CI. |

Use `--json` when you parse the output. Commit `setup.yaml`, `setup.lock` and every generated file after a change.

## After init or add in a running session

The command prints an activation note listing the new SKILL.md paths and instruction files.

1. Read every listed SKILL.md now. The agent loaded its skill list at session start, so new skills are invisible until you read them.
2. Read the new instruction blocks in AGENTS.md now and follow them for the rest of the session.
3. Tell the user that instructions and MCP changes load automatically only in the next session.

## Proposing bundles

Never run `setup add` or `setup init` with bundles the user has not confirmed.

1. Run `setup list --json`. It lists every bundle in the catalog with `layer`, `description`, `skills`, `latest_version`, `installed` and `installed_version`. Propose only bundles from this output.
2. Look at the project and the request. Check file types, dependencies, README, data files and the words in the user's request.
3. Match the evidence to the descriptions and skills of bundles that are not installed. Pick a domain bundle for the subject and a type bundle for the kind of output.
4. Propose the bundles in one message. For each one, give its name, its layer and the evidence. Include `suggests` pairings, marked as optional.
5. Wait for the user to confirm. Then run `setup add <bundles> --yes` and follow the activation steps above.

If nothing in the list fits the request, say so and do the work without a bundle.

## Suggests

A bundle's `suggests` list names other bundles that pair well with it. It is advice only. Nothing installs automatically. Mention each suggestion when you propose or add the bundle, and add it only after the user agrees.

## Updates

Run `setup outdated --json` when the user asks about updates or before a large task. Show the user which bundles are behind and the versions involved. Run `setup update` only after the user agrees, then run `setup check` and commit the result.
