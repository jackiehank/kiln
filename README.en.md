# kiln

**Issue tracking and research-path reconstruction, driven by git trailers.**

In one sentence: **kiln is a filter, not a registry.** It creates no database, no index, no record files for you; it does exactly two things:

- **Write**: encode one research action (ask / hypothesize / collect / test / conclude / apply) as a **canonical commit message with semantic trailers**;
- **Read**: *filter* those actions back out of the Git history by that convention, and rebuild an issue's **research path**, relation graph and completeness.

All state lives in **commit messages** — wherever `git clone` takes the repository, the research history goes with it. No server, no storage, no implicit state. Underneath is the `git` or `jj` you (and your agent) already know; kiln only adds structural semantics on top.

Language: [中文](README.md) · this English edition translates **this README only**; the design records under [`docs/`](docs/) stay in Chinese.

## Design: two atoms + emergence

kiln breaks both things it models into **atoms**, then lets complexity grow out of **combination** — instead of imposing a pre-designed process:

| What is split | How | Combined by |
| --- | --- | --- |
| **Atomic issues** | A complex problem is **not** subdivided into sub-threads inside one issue; it becomes **several independent issues** (each with its own `ask` and `domain`, passing `check` on its own) | Typed **relations**: `blocks` / `part-of` / `relates-to` / `caused-by` / `supersedes` (directed ones have inverse names) |
| **Atomic actions** | An issue's workflow has **no fixed stages**; it is made of **atomic actions** (ask / hypothesize / collect / test / conclude / apply), and every action is one commit | Free **combination**: any order, repeatable, vocabulary configurable |

So **complexity is emergent, not designed**:

- As fine-grained as you need: six `test` actions on one issue are normal;
- As decomposed as you need: a parent issue does not "get bigger", it just **gets more edges**;
- New processes need no new features: **new words** in the `.kilnconfig.jsonc` action vocabulary are enough.

The two axes are complementary: **issues combine through relations; inside an issue, actions combine**. An action is a **step in a process**, not an entity anyone references — which is why the old `object:` tag for single artifacts was retired.

> Lineage: **Unix philosophy** (small tools, single responsibility, composition to express complexity) and a deeper conviction — **complexity and good products are emergent, not designed**. See [docs/object-semantics.md](docs/object-semantics.md) (atomic issues) and [docs/evidence-fluidity.md](docs/evidence-fluidity.md) (the three principles in the original words).

## Install

Pick one of three ways.

**1. `go install` (needs Go 1.27+, see [go.mod](go.mod))**

```sh
go install github.com/jackiehank/kiln/cmd/kiln@latest
```

The binary lands in `$GOBIN` (default `$(go env GOPATH)/bin`, on Windows `%USERPROFILE%\go\bin`); make sure it is on `PATH`. Pin a version with `@v0.1.0` if you like.

**2. Build it yourself (single binary, no external dependencies)**

```sh
git clone https://github.com/jackiehank/kiln.git
cd kiln
go build -o kiln ./cmd/kiln
```

**3. Download a prebuilt binary**

Take the artifact for your platform from [Releases](https://github.com/jackiehank/kiln/releases) and put it on `PATH` (Windows: `kiln.exe`). Every `v*` tag is built by GitHub Actions into five artifacts — Windows amd64, macOS arm64/amd64, Linux amd64/arm64 — plus `checksums.txt`; Termux and other arm64 Linux environments take `kiln_linux_arm64`. The tag must be `v0.x` / `v1.x`: the module path has no `/v2`, so Go's version stamping ignores a higher major. Prebuilt artifacts ship with the first release; until then use one of the two methods above.

A single-binary CLI, with no `require` in `go.mod`. `kiln --version` tells you which build you run (release artifacts print the tag, e.g. `v0.1.0`).

## Five-minute tour

```sh
# 1) Open the issue: ask is the start action
kiln commit session-expiry ask --title "X concurrency may be unsafe" --domain auth

# 2) Middle actions: order is not a constraint, write what you use
kiln commit session-expiry hypothesize --title "Assume lock contention" --refs q1
kiln commit session-expiry collect     --title "Read two implementations" --refs "doc-a, doc-b"
kiln commit session-expiry test        --title "Reproduce under load" --target internal/session/lock.go

# 3) Close it: a terminal action (conclude / apply)
kiln commit session-expiry conclude --title "Conclusion: tune lock granularity"

# 4) Read the research path
kiln path session-expiry
```

```
$ kiln path session-expiry
session-expiry  done (currently conclude)
  1. ask          a1b2c3d  X concurrency may be unsafe
  2. hypothesize  e4f5g6h  Assume lock contention
  3. collect      k1l2m3n  Read two implementations
  4. test         i7j8k9l  Reproduce under load
  5. conclude     o4p5q6r  Conclusion: tune lock granularity
```

> Output snippets in this guide are translated; the CLI itself prints Chinese.

**Want the message without letting kiln commit?** Use `kiln do` — it **only prints, never writes a file**:

```sh
git commit -m "$(kiln do session-expiry test --title 'Reproduce under load')"
# or, on the jj backend
jj commit -m "$(kiln do session-expiry test --title 'Reproduce under load')"
```

> `kiln commit` handles staging itself: by default it takes **the whole working tree** — a snapshot of all changes (on git, kiln runs a full `add`). To commit only the listed paths, use `--only <paths>` (the same on both backends: git adds only those paths, jj uses a native fileset). Before committing, stdout prints `将提交 N 个文件: …`; empty actions are allowed and produce a single stderr hint.

## Command reference

| Command | What it does |
| --- | --- |
| `kiln do <issue> <action> [flags]` | Print the canonical commit message (**print only, no commit**) |
| `kiln commit <issue> <action> [--only <paths>] [flags]` | Commit with that message; by default the whole working tree — a snapshot of all changes; `--only` commits just the listed paths; stdout first prints `将提交 N 个文件: …`, then the canonical message, and finally `已提交 <hash> · <action> · <issue>` |
| `kiln path <issue>` | Rebuild the issue's research path; `--stat` adds a per-action file summary, `--shortstat` puts one total per action; `--json` gives one document, and `--all --json` (or several issues) reads every path in one process |
| `kiln show <issue> [n\|action]` | Show one action's full change (default: the last one; `n` is the number from `path`, **or give the action name** — that takes the **last** such action, hinting on stderr when there are several, and listing the valid numbers when no such action exists); `--stat` gives the file summary only |
| `kiln status [--json]` | Whole-repository overview: a directory view grouped by domain |
| `kiln check [--verbose] [--json]` | Semantic completeness check |
| `kiln list [--domain=] [--tag=] [--action=] [--status=open\|done] [--search=] [--json]` | Filter issues by field; `--status=open` keeps only the ones not wrapped up (last action is not terminal); `--search` splits on whitespace, ANDs the terms, matches literally and case-insensitively over id / title / action subject |
| `kiln graph [--format=ascii\|mermaid\|json] [--mermaid] [--json] [--issue <id>] [--depth N]` | Relation graph (`--mermaid` gives flowchart text; `--json` gives one document for agents, `--issue`/`--depth` narrow it to an undirected neighborhood; default `ascii`) |
| `kiln worktree new <issue>` | Create a dedicated worktree (see "Worktree mode") |
| `kiln worktree list` | List every worktree and its state |
| `kiln worktree remove <issue> [--force]` | Remove a worktree (refused while it still has work; `--force` discards it) |
| `kiln config path` | Print the config path (writing a commented default config when missing) |
| `kiln --version` | Print the version, executable path, build time and go platform (`-V` / `version` are the same command) |
| `kiln help` / `kiln -h` / `kiln <cmd> -h` | Help (`<cmd> -h` shows that command's usage and flags only) |

`do` / `commit` flags: `--title --refs --target --scope --domain --tag`; relations: `--blocks --blocked-by --part-of --has-part --relates-to --caused-by --causes --supersedes --superseded-by --unrelate` (all repeatable, comma-separated; the aliases `--derived-from` / `--spawns` are accepted too).
Also: `commit --force` (one-off waiver, see "Worktree mode"), `commit --only <paths>` (commit just the listed paths; comma-separated or repeated).
`--json`: supported by `status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` (machine-readable output, see "Machine-readable output"); `path` also takes `--all` to read the whole repository at once (or several issues directly), which works with `--json` only.

## Action vocabulary

Defaults: `ask` | `hypothesize` | `collect` | `test` | `conclude` | `apply`
Default start: `ask`; default terminals: `conclude`, `apply`.

**Every `action:` is one atomic action** — one question, one collection, one test, one conclusion — not a stage on an assembly line. Hence:

- **Only the two ends are structure**: every issue has **at least one start action** (default `ask`), and its **last action must be a terminal action** (default `conclude` / `apply`).
- **The middle is completely free**: any combination, any order, **repeatable** (six `test` actions are fine). There is no "hypothesize before test" pipeline rule.
- **You can keep committing after a terminal action**: kiln has no "closed" state — **"done" is a function of "the last action is terminal"** (the `✓` in `status` and the second rule of `check` come from the same computation). So after `ask → conclude → test` the issue **returns to in progress**, and `check` reports a missing terminal action: that is a **judgment, not an error**; add another terminal action to close it again.
- **The `1.` `2.` `3.` in `path` are the positions actions appeared in** (by time) — an **index**, not a stage number — so `1. collect 2. ask 3. conclude` is legal too (a start action only has to *exist*, not come first).

`apply` versus `conclude` is a semantic distinction: `conclude` = the conclusion holds, `apply` = the conclusion **landed in code**.

The vocabulary **may differ per repository** — it lives in the config (next section).

## Commit format (trailers)

**The recognition rule (the only signature): a commit is a kiln action if and only if it carries both `issue:` and `action:`.** If it carries only one of them, the commit is ignored (a normal commit that happens to say `issue: performance` is not picked up).

**Required**

| Trailer | Meaning |
| --- | --- |
| `issue:` | Issue ID (a single-segment kebab-case slug such as `session-expiry`); **the write side normalizes it** (case / `_` / repeated `-`), and a second `ask` on an issue that already has a start action is refused (see [docs/mvp-plan.md](docs/mvp-plan.md) §2.4) |
| `action:` | Action type (a member of the vocabulary above) |

**Optional (description)**: `title:` human-readable title | `refs:` referenced objects | `target:` files involved | `scope:` descriptive scope | `domain:` classification | `tags:` tags

**Optional (relations)**

| Trailer | Meaning | Direction |
| --- | --- | --- |
| `blocks:` | what I block | blocker → blocked |
| `blocked-by:` | who blocks me | blocked → blocker |
| `part-of:` | what I am part of | part → whole |
| `has-part:` | what belongs to me | whole → part |
| `relates-to:` | related | undirected |
| `caused-by:` | who caused me | result → cause |
| `causes:` | what I caused | cause → result |
| `supersedes:` | what I supersede | new → old |
| `superseded-by:` | who supersedes me | old → new |
| `unrelate:` | drop every relation to that issue (a declaration in the same action wins) | — |

The aliases `derived-from` / `spawns` remain readable and writable, and land on `caused-by` / `causes`.

**Keep one blank line between the title and the trailers**, otherwise `git log --oneline` and PR titles squeeze the whole trailer block onto one line.

Both ends of a relation key are **always issue names** (there is no "points at an action" key); canonical and inverse names are both writable — `blocks:` and `blocked-by:` are two views of the same edge, and the direction comes from the name itself.

`kiln do` lays all of this out for you, so you rarely write it by hand:

```sh
kiln do session-expiry test --title "Reproduce under load" --target internal/session/lock.go --blocks cli-necessity
```

## Configuration

The config file is `.kilnconfig.jsonc` at the **repository root** (JSONC: JSON + `//` and `/* */` comments, **no trailing commas**):

```jsonc
{
  // action vocabulary: every project may differ — this is kiln's customization point
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  // VCS backend: auto (.jj first, otherwise .git) / git / jj
  "vcs": "auto",
  // worktree mode: off (default, commits on main allowed) | on (actions may be committed only in a worktree — run kiln worktree new <issue> first)
  "worktree": "off"
}
```

- **Location**: `kiln config path` prints it; when the file is missing kiln **seeds a commented default config** (idempotent, never overwriting). Discovery walks **up from the current directory to the repository root**, so running from a subdirectory works the same.
- **Strict validation**: an empty vocabulary / `start`, `terminal` not in `names` / a **misspelled key** → error at startup, never silently ignored.
- **Minimal config**: `{"actions": {"names": ["ask", "probe", "done"]}}` → `start=ask`, `terminal=done`.

## Worktree mode (isolation)

Agents working in parallel pollute each other: in one working copy, two agents' uncommitted changes get mixed into the same commit and the attribution breaks. kiln's answer is **structural** — **main is read-only / merge-only, and actions are committed only in a dedicated worktree**:

```sh
kiln worktree new cli-necessity      # create the worktree (<repo root>/.kiln/ws/cli-necessity)
cd .kiln/ws/cli-necessity            # work in it
kiln commit cli-necessity test ...   # commit here
```

- **The `worktree` key defaults to `off`**: out of the box, `kiln commit` on main is allowed. For isolation write `"worktree": "on"` in the config: then a direct `kiln commit` on main is **structurally refused** with a hint to create a worktree first; to commit on main "just this once", add `--force` (a **one-off waiver** that changes no config and leaves no state).
- **You are warned before you act**: `kiln do` first sees where you stand — outside a repository, with isolation off, or **already inside a worktree** it stays quiet; on main it tells you to create a worktree first, and **when main also has uncommitted changes it hands you the exact commands to move them into the worktree** (one for git, one for jj). Hints go to stderr only, **never dirty the commit message**, and never change the exit code.
- **State**: the `kiln status` header reports the isolation mode and open count (`worktree on（已开 N 个）`); rows of issues with a worktree end in `■`; when any worktree is open the last line explains the mark and tells you **which worktree you are standing in**.
- **When it blocks you it offers a way out, not a wall**: the refusal branches on main being **dirty or clean** — clean, it only says "create a worktree first"; dirty, it first warns that a new worktree is a clean checkout and creating one now leaves the changes on main, then gives the backend recipe.
- **`kiln worktree list`** lists every worktree with a **state**:

  ```
  <repo>              main (read-only / merge-only)  —
  <repo>/.kiln/ws/t1  worktree t1                    clean
  <repo>/.kiln/ws/t2  worktree t2                    changes, unmerged
  ```

  There are four states: `clean` / `changes` (uncommitted) / `unmerged` (its commits are not on main yet) / `directory deleted` (the directory is gone but still registered). **The main row reports `—`** — it is not a dedicated worktree and has no work to judge. This state is computed the same way as the refusal criteria for `worktree remove`: what `list` reports is what `remove` refuses on.
- **`kiln worktree remove <issue>`** removes the worktree **and its branch**; while the worktree still has work (dirty / unmerged) it **refuses by default** — `--force` discards it.
- **A worktree is a one-off checkout**: a new worktree contains only **version-controlled** files — gitignored build artifacts and dependencies **are not there**; build / install them yourself first.
- **Seeding**: if the repository root has a **`.worktreeinclude`** file (one pattern per line, `.gitignore` syntax, `#` comments), creating a worktree copies **small local files that match and are ignored** (such as `.env`, local overrides) into the new worktree. Existing files are skipped, copying is one-way, and `.git`/`.jj`/`.kiln` metadata is never moved. Dependencies and build artifacts are **out of scope** — that is rebuilding, not copying.

## Backends: git and jj

kiln has a pluggable backend layer and **both git and jj work**. With `vcs: auto` (the default) **`.jj` wins over `.git`** — in a colocated repository both exist, and reading through git would lose jj's view.

| | git | jj |
| --- | --- | --- |
| Recording an action | by default kiln runs a full `add` and commits a snapshot; `--only` adds just the listed paths | by default the whole working copy is snapshotted into the commit; `--only` uses a native fileset |
| Empty action | allowed (`--allow-empty`) | allowed |
| Action history | counts **committed** actions only | excludes the working-copy commit (`@`), so again "committed only" |
| Backend name for the isolation unit | `worktree` | `workspace` (kiln's command surface calls both `kiln worktree`) |
| Dedicated-worktree test | is the current checkout a linked worktree | is the current workspace not `default` |

**Notes for colocated repositories (both `.jj` and `.git` present)**:

- jj exports its commits into git's refs automatically, so the `git log` and `jj log` views do not diverge.
- jj ignores itself via `.jj/.gitignore` (contents `/*`) so git does not report `.jj` as untracked; but `git clean -xdf` means "clean **ignored files too**" — **run it bare and it wipes `.jj/` as well**. Clean with `git clean -xdf -e .jj -e .kiln`, or only specific paths.
- This repository (kiln's own) is colocated, so the point above applies here too.

## Validation: `kiln check`

**Every issue must be opened by a start action and closed by a terminal action** (middle actions have no order constraint). The current rules:

1. Start: at least one start action (default `ask`) — missing reports `start-action` (✗)
2. Terminal: the last action is a terminal action (default `conclude` / `apply`) — otherwise `terminal-action` (⚠, warning only, non-blocking)
3. Relations: every declared relation target must exist — dangling reports `relation-target` (⚠, with the relation name and target issue)
4. Unknown keys: unrecognized trailer keys are named — `unknown-trailer` (⚠)
5. All of the above pass: `complete` (✓)

```
$ kiln check
✓ session-expiry   complete (ask, hypothesize, test, conclude)
⚠ deep-dive        missing a terminal action (currently test)
⚠ broken-ref       relation target missing: caused-by points at missing-parent
```

The convention: **only `✗` (errors) make `check` exit non-zero**; `⚠` merely informs — so "an issue in progress" never wedges CI.

## Machine-readable output (`--json`)

`status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` all emit machine-readable JSON (add `--json`) for agents and scripts. **Without the flag, the output does not change by a single byte.**

```sh
kiln status --json
kiln list --json --domain=design
kiln list --json --search=worktree
kiln check --json
kiln path --all --json
```

```json
{
  "passed": true,
  "findings": { "ok": 21, "warn": 9, "error": 0 },
  "results": [
    { "issue": "action-config", "level": "ok", "rule": "complete", "actions": ["ask", "apply"] }
  ]
}
```

Rules (full schema in [docs/machine-readable-output.md](docs/machine-readable-output.md)):

- **The top level is always one object**, never a bare array; `issues` / `results` are always arrays — an empty repository gives `[]`, not `null`.
- **`status` is one flat layer of rows**: `id` / `domain` / `action` / `done` / `title`, plus `has-worktree`. **No grouping, no nesting** — the text view marks a domain with one **block header** line, while JSON carries `domain` on every row, so **the relative order of issue rows matches** between the two (block-header lines don't count).
- **`check` speaks in rule codes**: `start-action` / `terminal-action` / `relation-target` / `unknown-trailer` / `complete` — no Chinese verdict strings; `passed` **means the same as the exit code** (`false` / exit 1 only when there is an `✗`), and `findings` counts **diagnostics**, not issues.
- **Absent means omitted**, **booleans are always written** (`false` is a verdict, not "missing").
- **stdout carries one JSON document only**: hints, legends and errors all go to stderr, and the exit code matches the text view.
- **`path --all --json` reads every path in one process**: `{"issues":[<one path document>…]}`, each entry byte-identical to `kiln path <id> --json`. Named ids keep their given order, `--all` sorts by id; one bad id does not sink the document — it lands in `errors` (`unknown-issue` / `stat-failed`), the rest still print, and the exit code is 1. The batch is `--json`-only (the text view expands one issue at a time).

## Working with agents

kiln's model suits agents naturally:

- **One convention is enough**: use `kiln commit` (or `git commit -m "$(kiln do …)"`) for research actions. That one sentence is all an agent needs.
- **Open the issue before writing code**: put only the issue doc in the `ask` action, commit it, then write the implementation. In that order nothing extra is needed; when you really must go the other way, `kiln commit … --only docs/<issue>.md` commits just the doc (identical on both backends) instead of moving the code aside. See [docs/commit-order.md](docs/commit-order.md), [docs/commit-scope.md](docs/commit-scope.md).
- **Structured output**: `status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` with `--json` are **truly** machine-readable, stdout carrying one JSON document; `path --all --json` returns every research path in one process (saving N startups), `graph` also has `--mermaid` (flowchart text), and `graph --json --issue <id> --depth N` emits only that issue's undirected neighborhood. See "Machine-readable output".
- **Honesty**: `check` points straight at structural problems — a missing terminal action, a missing relation target, an unknown trailer key (under `--json` it gives rule codes, not Chinese text).
- **Isolation**: parallel agents each take a worktree via `kiln worktree new`, removing cross-contamination **structurally**.

## What it does not do

- It creates no files (records like `q1.md` are written by a human or agent); it reads no diffs and does not classify changes;
- It has no central registry / index / database — all state is in commit messages;
- It has no mode switch (no "changes-only mode vs record mode"; everything is trailer convention);
- It does not orchestrate your workflow (no builds, no dependency installs, no pinned cwd);
- It does not rewrite history — an action *is* a commit; undo / revert / reuse across branches are git's own tools.

## Known boundaries

- `kiln graph` draws by **declaring-side arrows**: each relation appears only in the **declaring** issue's block (no inverse renaming, no duplication in the target block), symmetric relations (`relates-to`) carry no arrow, directed ones are `──name──▶`; when the same `(unordered pair, slot)` is declared from both sides **the later write wins**, and the whole graph shows a single shape — see [docs/graph-normalize.md](docs/graph-normalize.md).
- `worktree remove` cannot clear a worktree whose **directory is already deleted** (prune it first with `git worktree prune`).
- Uncommitted changes are **not** carried into a new worktree — **deliberate, not a TODO** (kiln does not manage local state, the same boundary as `.worktreeinclude` only handling ignored files; see [docs/worktree-adopt.md](docs/worktree-adopt.md)). To carry them by hand: on git `stash -u` → create the worktree → `pop` inside it; on jj `workspace add -r @` plus `jj edit @-` on main (**not** `jj restore`, which cascades a rebase that drags the new worktree in with it).
- **Machine-readable output covers seven commands**: `status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` all take `--json`; `--toon` is not evaluated yet — see [docs/machine-readable-output.md](docs/machine-readable-output.md), [docs/read-json.md](docs/read-json.md).

## Further reading

- [docs/mvp-plan.md](docs/mvp-plan.md) — **the specification**: core conventions, trailer rules, command surface, `check` rules, design principles. Read it before changing kiln.
- [docs/](docs/) — one record per design decision (`worktree-mode.md` / `worktree-include.md` / `config-home.md` …): why it looks like this, what was tried, what was dropped.
- Module map: `cmd/kiln` (entry) | `internal/model` (domain types) | `internal/config` (config) | `internal/trailer` (message parsing) | `internal/vcs` (backend interface + `git` / `jj` adapters + seeding) | `internal/engine` (aggregation: path / check / status / list / graph) | `internal/cli` (subcommands and rendering). Layering: `cli → engine → (vcs, model)`; the engine never execs directly.

## License

MIT, see [LICENSE](LICENSE).
