// Package cli wires the command surface: argument parsing, git access and text
// rendering. All output goes through the writers passed to Run, so the package
// stays testable.
package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/config"
	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/trailer"
	"github.com/jackiehank/kiln/internal/vcs"
	"github.com/jackiehank/kiln/internal/vcs/git"
	"github.com/jackiehank/kiln/internal/vcs/jj"
)

// retiredCommands maps a name kiln no longer answers to its current spelling, so
// a hard cut still tells the caller where the command went.
var retiredCommands = map[string]string{"workspace": "worktree"}

// env carries the resolved action vocabulary, the repository backend and the
// output streams. The backend is an interface so tests can substitute a fake
// instead of touching a real repository; it is opened lazily, so commands that
// need no repo (do) still work outside one.
type env struct {
	actions      *model.ActionSet
	relations    *model.RelationSet
	dir          string
	vcsPref      string
	worktree     string
	configPath   string
	configSource string
	noCache      bool // --no-cache / KILN_NO_CACHE turns the record cache off
	repo         vcs.Repo
	kind         vcs.Kind // the backend that was opened, recorded for backend-specific wording
	out          io.Writer
	errw         io.Writer
}

// rel returns the relation vocabulary, falling back to the built-in default
// when the env was built without one (e.g. tests).
func (e *env) rel() *model.RelationSet {
	if e.relations == nil {
		return model.DefaultRelationSet()
	}
	return e.relations
}

// fixedFlags are the do/commit flags that are not relation names.
var fixedFlags = []string{"title", "refs", "target", "scope", "domain", "tag", "unrelate", "force"}

// relationsFromFlags reads relation declarations from the parsed flags, accepting
// canonical names and aliases (resolved to canonical).
func (e *env) relationsFromFlags(f flags) []model.Rel {
	rel := e.rel()
	var out []model.Rel
	add := func(key string) {
		n, ok := rel.Resolve(key)
		if !ok {
			return
		}
		for _, t := range f.list(key) {
			out = append(out, model.Rel{Name: n.Name, Target: t})
		}
	}
	for _, name := range rel.Names() {
		add(name)
	}
	for _, alias := range rel.Aliases() {
		add(alias)
	}
	return out
}

// doCommitFlags is the do/commit flag allowlist: fixed flags plus the relation
// vocabulary's names and aliases.
func (e *env) doCommitFlags() map[string]bool {
	allowed := allowedFlags(fixedFlags...)
	for _, name := range e.rel().Names() {
		allowed[name] = true
	}
	for _, alias := range e.rel().Aliases() {
		allowed[alias] = true
	}
	return allowed
}

// backend returns the repository backend, opening it from dir/config on first
// use and caching it. Tests inject one via repo.
func (e *env) backend() (vcs.Repo, error) {
	if e.repo != nil {
		return e.repo, nil
	}
	r, kind, err := openRepo(e.dir, e.vcsPref)
	if err != nil {
		return nil, err
	}
	e.repo, e.kind = r, kind
	return r, nil
}

// openRepo picks a backend for dir. In auto mode jj wins over git: a colocated
// workspace contains both .jj and .git, so reading through git would silently
// miss jj's view.
func openRepo(dir, prefer string) (vcs.Repo, vcs.Kind, error) {
	kind, err := vcs.Detect(dir, prefer)
	if err != nil {
		return nil, "", err
	}
	switch kind {
	case vcs.Git:
		return git.ExecRunner{Dir: dir}, kind, nil
	case vcs.JJ:
		return jj.ExecRunner{Dir: dir}, kind, nil
	}
	return nil, "", i18n.E("未知后端: %s", kind)
}

// checkTargets validates --target for `do`: every declared path must be among the
// working changes the next commit would record. Outside a repository there is
// nothing to compare against (the backend fails), so the preview still prints.
func (e *env) checkTargets(targets []string) int {
	backend, err := e.backend()
	if err != nil {
		return 0
	}
	files, cerr := backend.Changes()
	if cerr != nil {
		return 0
	}
	return reportMissingTargets(e.errw, targets, files)
}

// reportMissingTargets refuses a target the file set does not cover; it names the
// missing paths and returns 1, mirroring the --only path refusal.
func reportMissingTargets(errw io.Writer, targets, files []string) int {
	missing := targetsNotIn(targets, files)
	if len(missing) == 0 {
		return 0
	}
	fmt.Fprint(errw, i18n.T("以下 target 没有改动可记录: %s\n", strings.Join(missing, ", ")))
	return 1
}

// targetsNotIn reports the declared targets the file set does not cover. Paths are
// slash-normalised and compared exactly; a target covers a directory when it is a
// prefix of a changed path, and a changed "sub/" (an untracked directory git
// collapsed) covers the files under it.
func targetsNotIn(targets, files []string) []string {
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "/")
		return strings.TrimPrefix(s, "./")
	}
	set := make([]string, 0, len(files))
	for _, f := range files {
		if f = norm(f); f != "" {
			set = append(set, f)
		}
	}
	var missing []string
	for _, t := range targets {
		t = norm(t)
		if t == "" {
			continue
		}
		dir := strings.TrimSuffix(t, "/") + "/"
		covered := false
		for _, f := range set {
			if f == t || strings.HasPrefix(f, dir) || (strings.HasSuffix(f, "/") && strings.HasPrefix(t, f)) {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, t)
		}
	}
	return missing
}

// write implements both `do` (print only) and `commit` (print and commit).
func (e *env) write(args []string, commit bool) int {
	pos, f, missing := parse(args, "force")
	if len(pos) < 2 {
		fmt.Fprint(e.errw, i18n.T("用法: kiln %s <issue> <action> --title <文本> [flags]\n", modeName(commit)))
		return 2
	}
	if len(pos) > 2 {
		fmt.Fprint(e.errw, i18n.T("用法: kiln %s <issue> <action> --title <文本> [flags]：object 已退役，第三个参数已不再支持\n", modeName(commit)))
		return 2
	}
	if code := rejectSwitchValues(e.errw, f, "force"); code != 0 {
		return code
	}
	if code := rejectRepeats(e.errw, f, "title", "target", "scope", "domain"); code != 0 {
		return code
	}
	issue := e.normalizedIssue(pos[0])
	action := strings.ToLower(strings.TrimSpace(pos[1]))
	if !e.actions.IsValid(action) {
		fmt.Fprint(e.errw, i18n.T("非法 action: %q（可选: %s）\n", action, strings.Join(e.actions.Names(), ", ")))
		return 2
	}
	if !model.ValidIssueID(issue) {
		fmt.Fprint(e.errw, i18n.T("警告: issue ID %q 不符合单段 kebab-case slug 约定（小写字母 / 数字 / `-`，长度 ≤ 50）\n", issue))
	}
	if e.actions.IsStart(action) {
		if code := e.refuseDuplicateStart(issue); code != 0 {
			return code
		}
	}

	rels := e.relationsFromFlags(f)
	allowed := e.doCommitFlags()
	allowed["only"] = true
	if code := rejectMissingValues(e.errw, missing, allowed); code != 0 {
		return code
	}
	warnUnknownFlags(e.errw, f, allowed)
	if !commit && f.has("only") {
		fmt.Fprintln(e.errw, i18n.T("do 不接受 --only：它只打印 trailer，不提交；只提交哪些文件是 commit 的事。"))
		return 2
	}
	// Every action carries a title: it is git's subject line, and without it the
	// whole trailer block becomes the subject and the body is empty (see the
	// condition's collect). do and commit share this check — do only prints the
	// same message commit would write.
	title := strings.TrimSpace(f.get("title"))
	if title == "" {
		if f.has("title") {
			fmt.Fprint(e.errw, i18n.T("--title 不能为空（每个动作都要有标题）\n"))
		} else {
			fmt.Fprint(e.errw, i18n.T("%s 需要 --title（每个动作都要有标题）\n", modeName(commit)))
		}
		return 2
	}
	paths := f.list("only")
	targets := f.list("target")
	msg := trailer.Format(trailer.Fields{
		Issue:     issue,
		Action:    action,
		Title:     title,
		Refs:      f.list("refs"),
		Target:    f.get("target"),
		Scope:     f.get("scope"),
		Domain:    f.get("domain"),
		Tags:      f.list("tag"),
		Relations: rels,
		Unrelate:  f.list("unrelate"),
	}, e.rel())
	if !commit {
		// do is the same judgement without the commit: a target that is not among
		// the working changes would produce a message commit refuses. Outside a
		// repository there is nothing to compare against, so the check is skipped
		// and the preview still prints.
		if len(targets) > 0 {
			if code := e.checkTargets(targets); code != 0 {
				return code
			}
		}
		fmt.Fprintln(e.out, msg)
		if tip := e.worktreeTip(issue); tip != "" {
			fmt.Fprintln(e.errw, tip)
		}
		return 0
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	if !f.has("force") {
		if code := e.refuseInMain(backend, issue); code != 0 {
			return code
		}
	}
	// The action is about to land, and nothing can take it back afterwards. Two
	// readings come first so the caller sees what is about to happen while it can
	// still stop it: every --only path has a change behind it (jj would otherwise
	// accept a fileset that matches nothing and record an empty action in silence),
	// and the inventory of what will be recorded is announced on stdout, where a
	// reader — human or agent — actually sees it.
	files := paths
	filesKnown := true
	if len(paths) > 0 {
		bad, uerr := backend.Unchanged(paths)
		if uerr != nil {
			fmt.Fprint(e.errw, i18n.T("检查路径失败: %v\n", uerr))
			return 1
		}
		if len(bad) > 0 {
			fmt.Fprint(e.errw, i18n.T("以下路径没有改动可记录: %s\n", strings.Join(bad, ", ")))
			return 1
		}
	} else if got, cerr := backend.Changes(); cerr == nil {
		files = got
	} else {
		filesKnown = false
	}
	// Every declared target must be part of what this commit records: a target
	// that is not in the change set is a claim the commit itself disproves. This
	// is a write-side guard (it runs before anything lands), not a later audit of
	// history — check stays about the current structure only.
	if len(targets) > 0 && filesKnown {
		if code := reportMissingTargets(e.errw, targets, files); code != 0 {
			return code
		}
	}
	if len(files) > 0 {
		fmt.Fprint(e.out, i18n.T("将提交 %d 个文件: %s\n", len(files), inventory(files)))
	} else if len(paths) == 0 && filesKnown {
		// An empty action is legitimate, so this is a hint, not a refusal: say it
		// before the commit exists rather than after. Changes already read the
		// working tree; an empty inventory is the same reading Dirty would return,
		// so asking again would only spawn a second status.
		fmt.Fprintln(e.errw, i18n.T("提示: 当前工作区没有检测到改动，这次动作会记成空提交。"))
	}
	id, err := recordAction(backend, msg, paths)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("提交失败: %v\n", err))
		return 1
	}
	fmt.Fprintln(e.out, msg)
	// The confirmation names what was just written. Without it the caller only sees
	// the text it already had from `do`, and has to run a second command to learn
	// whether anything landed — which is exactly the round trip this line saves.
	if id == "" {
		fmt.Fprint(e.out, i18n.T("\n已提交 · %s · %s\n", action, issue))
	} else {
		fmt.Fprint(e.out, i18n.T("\n已提交 %s · %s · %s\n", shortID(id), action, issue))
	}
	return 0
}

// recordAction records the action. write has already validated every --only
// path with Unchanged, so a backend that offers CheckedAction is asked to skip
// the second read: without it, `--only` pays two status processes per path.
func recordAction(backend vcs.Repo, msg string, paths []string) (string, error) {
	if len(paths) > 0 {
		if checked, ok := backend.(vcs.CheckedAction); ok {
			return checked.ActionChecked(msg, paths)
		}
	}
	return backend.Action(msg, paths)
}

// inventory renders the file list an action will record: every path when the list
// is short, the first five and a count when it would drown the confirmation. The
// count is what carries the information in the long case; the paths are the
// paste-ready detail in the common case.
func inventory(files []string) string {
	const shown = 5
	if len(files) <= shown {
		return strings.Join(files, ", ")
	}
	return strings.Join(files[:shown], ", ") + i18n.T(" 等 %d 个", len(files))
}

// refuseInMain enforces kiln's unified layout: the main checkout is read/merge
// only, so an action is recorded in a dedicated area instead. The test is purely
// structural, so it needs no concurrency knowledge. Backends without workspace
// support are left unchecked; --force bypasses the guard.
//
// The refusal branches on whether main still holds uncommitted work, because in
// that case "先建区" on its own is wrong advice: a new area is a clean checkout,
// so the work would be left behind.
//
// It never suggests turning isolation off in the config: a runtime message that an
// agent reads while working must not invite editing the repository's shared
// settings — that decision belongs to whoever owns the repository, and the config
// file documents the option itself.
func (e *env) refuseInMain(backend vcs.Repo, issue string) int {
	if !e.isolated() {
		return 0 // worktree off (the default) — the repository records actions in main
	}
	ws, ok := backend.(vcs.Workspace)
	if !ok {
		return 0
	}
	dedicated, err := ws.IsDedicated()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("无法判定 worktree 类型: %v\n", err))
		return 1
	}
	if dedicated {
		return 0
	}
	fmt.Fprintln(e.errw, i18n.T("拒绝提交：当前是 main（只读/只合并），动作只在 worktree 里提交。"))
	if dirty, err := ws.Dirty(); err == nil && dirty {
		fmt.Fprintln(e.errw, i18n.T("你已有未提交改动——新区是干净检出，直接建区会把改动留在 main："))
		if recipe := e.carryRecipe(issue, ws); recipe != "" {
			fmt.Fprintln(e.errw, recipe)
		}
		// --force stays documented, but it must not read as the cheaper
		// alternative to the recipe: it only lets the guard through, it does not
		// move anything, so it is offered as a condition you have to *want* —
		// otherwise an agent optimizing for "make the command succeed" takes the
		// one flag and leaves the work sitting in main.
		fmt.Fprintln(e.errw, i18n.T("（--force 只放行护栏、不把改动搬进区——只有你确实要把它留在 main 时才用。）"))
		return 1
	}
	fmt.Fprint(e.errw, i18n.T("先 `kiln worktree new %s`，再 cd 到 worktree 提交。\n", issue))
	fmt.Fprintln(e.errw, i18n.T("确要在 main 提交时加 --force（一次性豁免）。"))
	return 1
}

// isolated reports whether the repository requires a dedicated area for actions.
// An unset value means off: isolation is opt-in, so the guard engages only when a
// repository asked for it by name.
func (e *env) isolated() bool { return e.worktree == config.WorktreeOn }

// worktreeTip returns the isolation hint `do` prints on stderr, or "" when there
// is nothing worth saying. It is best effort on purpose — `do` only prints a
// message, so a hint that cannot be computed must never become a failure — and it
// is deliberately kept off stdout, which has to stay the bare commit message.
func (e *env) worktreeTip(issue string) string {
	backend, err := e.backend()
	if err != nil {
		if errors.Is(err, vcs.ErrNoRepository) {
			// Being outside a repository is worth saying: there is no isolation
			// to ask for, and a caller who just formatted an action should know
			// it will not be recorded anywhere here.
			return i18n.T("提示: ") + err.Error()
		}
		return ""
	}
	if !e.isolated() {
		return ""
	}
	ws, ok := backend.(vcs.Workspace)
	if !ok {
		return ""
	}
	if here, err := ws.IsDedicated(); err != nil || here {
		return "" // already in a dedicated area, or undecidable: stay quiet
	}
	head := i18n.T("提示: 本仓要求动作在 worktree 里提交——开工前先 kiln worktree new ") + issue
	dirty, err := ws.Dirty()
	if err != nil || !dirty {
		return head
	}
	recipe := e.carryRecipe(issue, ws)
	if recipe == "" {
		return head
	}
	return i18n.T("提示: 本仓要求动作在 worktree 里提交；你已有未提交改动——") + recipe
}

// carryRecipe names the commands that move main's uncommitted changes into a new
// area, per backend. Nothing is carried by kiln itself (docs/worktree-adopt.md
// settled that): the recipe is the backend's own, and both avoid rewriting
// history, which is the trap on the jj side.
func (e *env) carryRecipe(issue string, ws vcs.Workspace) string {
	switch e.kind {
	case vcs.Git:
		// The stash is repository-wide, so it can be popped from inside the area.
		return i18n.T("git：git stash -u → kiln worktree new %s → 进区后 git stash pop", issue)
	case vcs.JJ:
		// jj keeps the change in the current @, so basing the new workspace on it
		// is what carries it; jj edit @- then leaves main clean without rewriting
		// a single commit. (jj restore would rewrite main's @ and drag the new
		// workspace along with it.) jj creates no intermediate directories, so the
		// area's parent has to exist first — the mkdir is exactly what kiln's own
		// PrepareAreas does before it calls workspace add.
		dir := vcs.StateDir + "/" + vcs.AreasDir
		mainRoot, err := mainRootOf(ws)
		if err != nil {
			return i18n.T("jj：先在仓库根 mkdir %s，再 jj workspace add --name %s <区路径> -r @（新区即带上它），最后在 main 跑 jj edit @-", dir, issue)
		}
		return i18n.T("jj：先在仓库根 mkdir %s，再 jj workspace add --name %s %s -r @（新区即带上它），最后在 main 跑 jj edit @-", dir, issue, vcs.AreaPath(mainRoot, issue))
	}
	return ""
}

// workspace implements `kiln worktree new <issue>` and `kiln worktree list`.
func (e *env) workspace(args []string) int {
	// Parse first so a flag may precede the subcommand (`worktree --json list`),
	// the same order `config` uses; the subcommand is then just the first
	// positional and the flags are already known.
	pos, f, _ := parse(args, "json", "force")
	if code := rejectSwitchValues(e.errw, f, "json", "force"); code != 0 {
		return code
	}
	sub := ""
	rest := pos
	if len(pos) > 0 {
		sub, rest = pos[0], pos[1:]
	}
	ws, code := e.workspaceBackend()
	if ws == nil {
		return code
	}
	switch sub {
	case "new":
		if code := rejectExtra(e.errw, rest, 1, "kiln worktree new <issue>"); code != 0 {
			return code
		}
		warnUnknownFlags(e.errw, f, allowedFlags())
		return e.workspaceNew(ws, rest)
	case "list":
		// The only flag is --json; a stray one is still a mistake, and a mistake
		// that changes nothing silently is the bug this surface fixed earlier.
		if code := rejectExtra(e.errw, rest, 0, "kiln worktree list [--json]"); code != 0 {
			return code
		}
		warnUnknownFlags(e.errw, f, allowedFlags("json"))
		return e.workspaceList(ws, f)
	case "remove":
		if code := rejectExtra(e.errw, rest, 1, "kiln worktree remove <issue> [--force]"); code != 0 {
			return code
		}
		warnUnknownFlags(e.errw, f, allowedFlags("force"))
		return e.workspaceRemove(ws, rest, f)
	default:
		fmt.Fprintln(e.errw, i18n.T("用法: kiln worktree new <issue> | kiln worktree list | kiln worktree remove <issue> [--force]"))
		return 2
	}
}

// workspaceBackend opens the backend and requires workspace support.
func (e *env) workspaceBackend() (vcs.Workspace, int) {
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return nil, 1
	}
	ws, ok := backend.(vcs.Workspace)
	if !ok {
		fmt.Fprintln(e.errw, i18n.T("当前后端不支持 worktree"))
		return nil, 1
	}
	return ws, 0
}

// workspaceNew creates (or reuses) the dedicated area bound to an issue and
// points the user at it. The area's name and branch are derived from the issue,
// so naming alone is the binding — no extra file keeps them in sync.
func (e *env) workspaceNew(ws vcs.Workspace, pos []string) int {
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, i18n.T("用法: kiln worktree new <issue>"))
		return 2
	}
	issue := e.normalizedIssue(pos[0])
	if !model.ValidIssueID(issue) {
		fmt.Fprint(e.errw, i18n.T("非法 issue ID: %q（须为小写 kebab-case slug）\n", issue))
		return 2
	}
	path, err := ws.NewWorkspace(issue)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("建立 worktree 失败: %v\n", err))
		return 1
	}
	fmt.Fprintf(e.out, "worktree: %s\n", path)
	e.seedArea(ws, path)
	fmt.Fprint(e.out, i18n.T("下一步: cd 到该目录，再用 `kiln commit %s <action>` 记录动作\n", issue))
	// Project-neutral: an area is a bare checkout, whatever the project builds
	// with. Making it usable is the project's own business, so the hint names no
	// language, no path and no tool — and it uses the user-facing noun (worktree),
	// not the internal shorthand.
	fmt.Fprintln(e.out, i18n.T("提示: worktree 是一次性检出——被忽略的构建产物与依赖不在其中，请自行 build / 装依赖"))
	if cfgPath, created, err := config.Ensure(e.dir); err != nil {
		fmt.Fprint(e.errw, i18n.T("提示: 未能写入默认配置: %v\n", err))
	} else if created {
		fmt.Fprint(e.out, i18n.T("已写入默认配置: %s\n", cfgPath))
	}
	return 0
}

// seedArea carries the repository's declared local files (.worktreeinclude) into
// a new area, so it starts usable instead of as a bare checkout. Seeding is best
// effort: a failure is a hint, never a command failure, because the recording of
// an action does not depend on it.
func (e *env) seedArea(ws vcs.Workspace, areaRoot string) {
	mainRoot, err := mainRootOf(ws)
	if err != nil {
		return // the backend could not name the main checkout; stay quiet
	}
	copied, err := vcs.SeedArea(mainRoot, areaRoot)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("提示: 未能播种本地文件: %v\n", err))
		return
	}
	if len(copied) > 0 {
		fmt.Fprint(e.out, i18n.T("已播种本地文件 %d 个（%s）: %s\n", len(copied), vcs.IncludeFile, strings.Join(copied, ", ")))
	}
}

// mainRootOf returns the main checkout's root from a backend's area list, which
// lists the main area first.
func mainRootOf(ws vcs.Workspace) (string, error) {
	areas, err := ws.ListWorkspaces()
	if err != nil {
		return "", err
	}
	for _, a := range areas {
		if a.Main {
			return a.Path, nil
		}
	}
	return "", i18n.E("后端未列出 main")
}

// action implements the read-only `action` group. Only `list` exists today;
// `log` will join it later.
func (e *env) action(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			if text, ok := commandUsageText(e.rel(), "action"); ok {
				fmt.Fprint(e.out, text)
				return 0
			}
		}
	}
	// Parse first so flags may precede the subcommand (`action --json list`),
	// the same shape `worktree` uses.
	pos, f, missing := parse(args, "json", "jsonl")
	sub := ""
	rest := pos
	if len(pos) > 0 {
		sub, rest = pos[0], pos[1:]
	}
	switch sub {
	case "list":
		if code := rejectExtra(e.errw, rest, 0, "kiln action list [--json]"); code != 0 {
			return code
		}
		// The two subcommands share one flag namespace after parsing, so each
		// refuses the other's flags instead of dropping them on the floor.
		if f.has("jsonl") {
			fmt.Fprintln(e.errw, i18n.T("action list 用 --json，不用 --jsonl"))
			return 2
		}
		if f.has("limit") {
			fmt.Fprintln(e.errw, i18n.T("--limit 只服务 action log"))
			return 2
		}
		if code := rejectSwitchValues(e.errw, f, "json"); code != 0 {
			return code
		}
		warnUnknownFlags(e.errw, f, allowedFlags("json"))
		return e.actionList(f)
	case "log":
		if code := rejectExtra(e.errw, rest, 0, "kiln action log [--jsonl] [--limit N]"); code != 0 {
			return code
		}
		allowed := allowedFlags("jsonl", "limit")
		if code := rejectMissingValues(e.errw, missing, allowed); code != 0 {
			return code
		}
		if code := rejectSwitchValues(e.errw, f, "jsonl"); code != 0 {
			return code
		}
		if code := rejectRepeats(e.errw, f, "limit"); code != 0 {
			return code
		}
		if f.has("json") {
			fmt.Fprintln(e.errw, i18n.T("action log 用 --jsonl（一行一个动作），不用 --json"))
			return 2
		}
		warnUnknownFlags(e.errw, f, allowed)
		return e.actionLog(f)
	default:
		fmt.Fprintln(e.errw, i18n.T("用法: kiln action list [--json] | kiln action log [--jsonl] [--limit N]"))
		return 2
	}
}

// actionList prints the effective vocabulary, not the history: the text view is
// one action per line with optional start/terminal markers, and --json mirrors
// the config's actions block plus where it came from.
func (e *env) actionList(f flags) int {
	if f.has("json") {
		source := e.configSource
		if source == "" {
			source = config.SourceDefault
		}
		doc := actionListDoc{
			Actions: actionSetDoc{
				Names:    e.actions.Names(),
				Start:    e.actions.Start(),
				Terminal: terminalNames(e.actions),
			},
			Source: source,
		}
		if source == config.SourceFile && e.configPath != "" {
			doc.Config = e.configPath
		}
		return e.writeJSON(doc)
	}
	names := e.actions.Names()
	width := 0
	for _, name := range names {
		if w := displayWidth(name); w > width {
			width = w
		}
	}
	for _, name := range names {
		line := pad(name, width)
		var markers []string
		if e.actions.IsStart(name) {
			markers = append(markers, "start")
		}
		if e.actions.IsTerminal(name) {
			markers = append(markers, "terminal")
		}
		if len(markers) > 0 {
			line += "  " + strings.Join(markers, ", ")
		}
		fmt.Fprintln(e.out, strings.TrimRight(line, " "))
	}
	return 0
}

// actionLog renders the global action stream: one row per commit that carries
// both `issue:` and `action:`, newest author date first. It never builds the
// issue model — one pass, one row at a time — so the first line arrives with the
// first commit instead of after the whole history, and memory stays flat however
// large the repository is.
//
// --limit and a consumer that went away both stop the scan through the sink's
// false return; the backend kills its child at that point.
func (e *env) actionLog(f flags) int {
	limit := 0
	if f.has("limit") {
		n, err := strconv.Atoi(strings.TrimSpace(f.get("limit")))
		if err != nil || n < 1 {
			fmt.Fprint(e.errw, i18n.T("--limit 需要 ≥ 1 的整数（当前: %q）\n", f.get("limit")))
			return 2
		}
		limit = n
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	stream, ok := backend.(vcs.LogEach)
	if !ok {
		fmt.Fprintln(e.errw, i18n.T("当前后端不支持 action log 的流式读取"))
		return 1
	}
	jsonl := f.has("jsonl")
	var scratch bytes.Buffer
	enc := json.NewEncoder(&scratch)
	enc.SetEscapeHTML(false)
	var writeErr error
	emitted := 0
	// emit writes one row and reports whether the commit was an action at all.
	emit := func(c vcs.Commit) bool {
		fields, ok := trailer.Parse(c.Message, e.rel())
		if !ok {
			return false
		}
		if err := e.writeActionLogRow(c.ID, c.Date, fields, engine.Subject(c.Message), jsonl, &scratch, enc); err != nil {
			writeErr = err
		}
		return true
	}
	// No --limit means the whole history is going to be printed: on a warm cache
	// that is an enumeration plus lookups, without printing 35 MB of messages.
	if limit == 0 {
		if n, handled, cerr := e.actionLogCached(backend, jsonl, &scratch, enc); handled {
			if cerr != nil {
				if isBrokenPipe(cerr) {
					return 0
				}
				fmt.Fprint(e.errw, i18n.T("输出失败: %v\n", cerr))
				return 1
			}
			if n == 0 {
				fmt.Fprint(e.errw, i18n.T("仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）\n"))
			}
			return 0
		}
	}
	err = stream.LogEach(func(c vcs.Commit) bool {
		kept := emit(c)
		if writeErr != nil {
			return false
		}
		if kept {
			emitted++
		}
		return limit == 0 || emitted < limit
	})
	if writeErr != nil {
		if isBrokenPipe(writeErr) {
			return 0 // the consumer closed the pipe: not a kiln failure
		}
		fmt.Fprint(e.errw, i18n.T("输出失败: %v\n", writeErr))
		return 1
	}
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取历史失败: %v\n", err))
		return 1
	}
	if emitted == 0 {
		fmt.Fprint(e.errw, i18n.T("仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）\n"))
	}
	return 0
}

// writeActionLogRow renders one action's row. Both read paths — the stream and the
// cache — call it, so their bytes cannot drift apart.
func (e *env) writeActionLogRow(id string, when time.Time, f trailer.Fields, subject string, jsonl bool, scratch *bytes.Buffer, enc *json.Encoder) error {
	label := f.Title
	if label == "" {
		label = subject
	}
	if jsonl {
		scratch.Reset()
		if err := enc.Encode(actionLogDoc{
			Issue:     f.Issue,
			Action:    f.Action,
			Commit:    id,
			Date:      when.Format(time.RFC3339),
			Title:     label,
			Domain:    f.Domain,
			Tags:      f.Tags,
			Refs:      dedupeStrings(f.Refs),
			Target:    dedupeStrings(trailer.SplitList(f.Target)),
			Scope:     f.Scope,
			Relations: actionLogRelations(e.rel(), f.Relations),
			Unrelate:  dedupeStrings(f.Unrelate),
		}); err != nil {
			return err
		}
		_, err := e.out.Write(scratch.Bytes())
		return err
	}
	line := fmt.Sprintf("%s  %s  %s  %s  %s",
		when.Local().Format("2006-01-02 15:04"),
		pad(f.Issue, 24),
		pad(f.Action, 12),
		shortID(id),
		label)
	_, err := fmt.Fprintln(e.out, strings.TrimRight(line, " "))
	return err
}

// actionLogRelations lays an action's declarations out in a fixed order: the
// vocabulary's own order first (so the same repository always prints the same
// bytes), then each name's targets in the order the action wrote them, with
// exact repeats dropped. Parse has already canonicalized aliases, and names
// outside the vocabulary never reach Fields, but the sorted fallback keeps the
// output total (and therefore deterministic) if that ever changes.
func actionLogRelations(rel *model.RelationSet, decls []model.Rel) []actionLogRelation {
	if len(decls) == 0 {
		return nil
	}
	byName := make(map[string][]string, len(decls))
	for _, d := range decls {
		byName[d.Name] = append(byName[d.Name], d.Target)
	}
	for name, targets := range byName {
		byName[name] = dedupeStrings(targets)
	}
	out := make([]actionLogRelation, 0, len(decls))
	emit := func(name string) {
		for _, t := range byName[name] {
			out = append(out, actionLogRelation{Name: name, Target: t})
		}
		delete(byName, name)
	}
	for _, name := range rel.Names() {
		emit(name)
	}
	extra := make([]string, 0, len(byName))
	for name := range byName {
		extra = append(extra, name)
	}
	sort.Strings(extra)
	for _, name := range extra {
		emit(name)
	}
	return out
}

// terminalNames lists the terminal actions in declaration order. ActionSet has
// no direct accessor, and deriving keeps the order the config declares.
func terminalNames(set *model.ActionSet) []string {
	names := set.Names()
	terminal := make([]string, 0, len(names))
	for _, name := range names {
		if set.IsTerminal(name) {
			terminal = append(terminal, name)
		}
	}
	return terminal
}

// workspaceList prints every working area, main first, aligned by path and role
// width, each row ending in what that area still holds. The state is asked of
// the backend for every area: the column exists to be trusted, so it is never
// left to a flag the caller might forget.
func (e *env) workspaceList(ws vcs.Workspace, f flags) int {
	areas, err := ws.ListWorkspaces()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取 worktree 失败: %v\n", err))
		return 1
	}
	states := make([]vcs.AreaState, len(areas))
	for i, a := range areas {
		state, err := ws.AreaState(a)
		if err != nil {
			fmt.Fprint(e.errw, i18n.T("读取 worktree 状态失败: %v\n", err))
			return 1
		}
		states[i] = state
	}
	if f.has("json") {
		doc := worktreeListDoc{Backend: string(e.kind), Worktrees: make([]worktreeEntry, 0, len(areas))}
		for i, a := range areas {
			doc.Worktrees = append(doc.Worktrees, worktreeEntry{
				Path:     a.Path,
				Main:     a.Main,
				Issue:    a.Issue,
				Gone:     states[i].Gone,
				Dirty:    states[i].Dirty,
				Unmerged: states[i].Unmerged,
			})
		}
		return e.writeJSON(doc)
	}
	pathWidth, roleWidth := 0, 0
	for _, a := range areas {
		if w := displayWidth(a.Path); w > pathWidth {
			pathWidth = w
		}
		if w := displayWidth(areaRole(a)); w > roleWidth {
			roleWidth = w
		}
	}
	for i, a := range areas {
		fmt.Fprintf(e.out, "%s  %s  %s\n",
			pad(a.Path, pathWidth), pad(areaRole(a), roleWidth), areaStateWord(a, states[i]))
	}
	return 0
}

// areaStateWord renders what an area still holds, in the words `remove` refuses
// in. main is not a dedicated area, so it reports nothing: it shows a dash
// rather than claiming to be clean.
func areaStateWord(a vcs.WorkspaceInfo, s vcs.AreaState) string {
	if a.Main {
		return "—"
	}
	var parts []string
	if s.Gone {
		parts = append(parts, i18n.T("目录已删"))
	}
	if s.Dirty {
		parts = append(parts, i18n.T("有改动"))
	}
	if s.Unmerged {
		parts = append(parts, i18n.T("未合并"))
	}
	if len(parts) == 0 {
		return i18n.T("干净")
	}
	return strings.Join(parts, i18n.T("，"))
}

// workspaceRemove deletes the dedicated area bound to an issue. The name is the
// binding, so the issue alone identifies the area; an area that still holds work
// is refused by the backend unless --force says to lose it.
func (e *env) workspaceRemove(ws vcs.Workspace, pos []string, f flags) int {
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, i18n.T("用法: kiln worktree remove <issue> [--force]"))
		return 2
	}
	issue := pos[0]
	if err := ws.RemoveWorkspace(issue, f.has("force")); err != nil {
		fmt.Fprint(e.errw, i18n.T("删除 worktree 失败: %v\n", err))
		return 1
	}
	fmt.Fprint(e.out, i18n.T("已删除 worktree: %s\n", issue))
	return 0
}

// areaRole labels an area in `kiln workspace list`.
func areaRole(a vcs.WorkspaceInfo) string {
	if a.Main {
		return i18n.T("main（只读/只合并）")
	}
	if a.Issue == "" {
		return "worktree"
	}
	return "worktree " + a.Issue
}

func (e *env) path(args []string) int {
	pos, f, _ := parse(args, "all", "json", "stat", "shortstat")
	warnUnknownFlags(e.errw, f, allowedFlags("stat", "shortstat", "json", "all"))
	// A switch never takes a value; saying --json=x is a mistake worth naming
	// instead of dropping x (or swallowing the positional that follows).
	if code := rejectSwitchValues(e.errw, f, "all", "json", "stat", "shortstat"); code != 0 {
		return code
	}
	// Batch is decided by what the caller asked for, before repeats are folded
	// away: `path a a --json` is still a batch request, it just names one issue.
	batch := len(pos) > 1
	pos = dedupeStrings(pos)
	all := f.has("all")
	if len(pos) == 0 && !all {
		fmt.Fprintln(e.errw, i18n.T("用法: kiln path <issue> [<issue>…] | --all [--stat|--shortstat|--json]"))
		return 2
	}
	if all && len(pos) > 0 {
		fmt.Fprintln(e.errw, i18n.T("--all 与问题参数不能同用"))
		return 2
	}
	// Batch is JSON-only: the text view is a reading layout (one path, stats
	// nested under each action), while the batch exists to feed a consumer.
	// Refusing beats silently rendering only the first argument, which is what
	// this command used to do.
	if (all || batch) && !f.has("json") {
		fmt.Fprintln(e.errw, i18n.T("批量取路径只支持 --json（--all 或多条问题）"))
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	mode := statMode(f)
	if all || batch {
		ids := pos
		if all {
			ids = engine.IDs(repo)
		}
		return e.pathBatch(repo, ids, mode)
	}
	p, ok := repo.Issues[pos[0]]
	if !ok {
		fmt.Fprint(e.errw, i18n.T("未找到问题: %s\n", pos[0]))
		return 1
	}
	if f.has("json") {
		return e.pathJSON(repo, p, mode)
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	fmt.Fprint(e.out, i18n.T("%s  %s（当前 %s）\n", p.ID, e.progressWord(p), stateWord(p)))
	for i, a := range p.Actions {
		line := fmt.Sprintf("  %d. %s %s %s", i+1, pad(a.Kind, 12), pad(shortID(a.ID), 8), a.Subject)
		if mode == statShort {
			stat, err := backend.ShowStat(a.ID, true)
			if err != nil {
				fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
				return 1
			}
			if s := strings.TrimSpace(stat); s != "" {
				line += "  (" + s + ")"
			}
		}
		fmt.Fprintln(e.out, line)
		if mode == statBlock {
			stat, err := backend.ShowStat(a.ID, false)
			if err != nil {
				fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
				return 1
			}
			fmt.Fprint(e.out, indentStat(stat))
		}
	}
	if lines := engine.PathLines(repo, p, e.rel()); len(lines) > 0 {
		fmt.Fprintln(e.out, i18n.T("  关系:"))
		for _, l := range lines {
			fmt.Fprintln(e.out, "    "+l)
		}
	}
	return 0
}

// pathDoc is `kiln path --json` for one issue: the whole research path as one
// object. Actions keep their 1-based ordinal and full commit hash, so a caller
// can address `kiln show` without parsing the text view. A batch wraps these in
// pathBatchDoc unchanged, so one path reads the same either way.
type pathDoc struct {
	Issue     string       `json:"issue"`
	Action    string       `json:"action"`
	Done      bool         `json:"done"`
	Actions   []pathAction `json:"actions"`
	Relations []graphEdge  `json:"relations"`
}

// pathBatchDoc is `kiln path --all --json` / `kiln path <id> <id> --json`: one
// process, every requested path. issues is always an array (empty, never null);
// a request that could not be answered is named in errors instead of aborting
// the whole document.
type pathBatchDoc struct {
	Issues []pathDoc        `json:"issues"`
	Errors []pathBatchError `json:"errors,omitempty"`
}

// pathBatchError is one id that did not make it into issues. The code is a
// language-neutral token, not a translated sentence: JSON is data, so the same
// repository must produce the same bytes whatever --lang says.
type pathBatchError struct {
	Issue string `json:"issue"`
	Error string `json:"error"`
}

// pathBatchError codes. unknown-issue means the id was never opened; stat-failed
// means the backend could not produce that action's diffstat.
const (
	errUnknownIssue = "unknown-issue"
	errStatFailed   = "stat-failed"
)

// pathAction is one action in the path: what it did, when, under which hash,
// and — with --stat / --shortstat — the diffstat that belongs to it. Tags are
// the action's own `tags:` trailer (commit order, duplicates kept); they are not
// the issue-level union, which is what `graph --json` and `list --json` carry.
type pathAction struct {
	Ordinal int      `json:"ordinal"`
	Action  string   `json:"action"`
	Commit  string   `json:"commit"`
	Date    string   `json:"date"`
	Title   string   `json:"title,omitempty"`
	Refs    []string `json:"refs,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Stat    string   `json:"stat,omitempty"`
}

// worktreeListDoc is `kiln worktree list --json`: every area with the three
// judgments `remove` refuses on, so a caller decides without parsing words.
// backend names the VCS the list came from; it is not the isolation policy
// (`status.worktree.policy`), so the key says which of the two it is.
type worktreeListDoc struct {
	Backend   string          `json:"backend,omitempty"`
	Worktrees []worktreeEntry `json:"worktrees"`
}

// worktreeEntry is one area. main carries no issue; the booleans are always
// written because they are verdicts, not missing data.
type worktreeEntry struct {
	Path     string `json:"path"`
	Main     bool   `json:"main"`
	Issue    string `json:"issue,omitempty"`
	Gone     bool   `json:"gone"`
	Dirty    bool   `json:"dirty"`
	Unmerged bool   `json:"unmerged"`
}

// pathJSON renders the path as one document. Relations reuse the graph's edge
// shape, restricted to the edges touching this issue, so a caller that already
// understands `graph --json` needs nothing new. With --stat / --shortstat the
// diffstat travels in each action's stat field instead of replacing the view.
func (e *env) pathJSON(repo *model.Repo, p *model.Issue, mode statDisplay) int {
	backend, code := e.statBackend(mode)
	if code != 0 {
		return code
	}
	doc, err := e.pathDocOf(p, mode, backend, engine.FoldEdges(repo, e.rel()), nil)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
		return 1
	}
	return e.writeJSON(doc)
}

// pathBatch renders several paths as one document. --all follows engine.IDs
// (sorted by id); explicitly named ids keep the order they were given. A single
// bad id does not sink the document — it lands in errors, the rest still print,
// and the exit code stays non-zero so a script can notice.
//
// With --stat / --shortstat it fetches every action's diffstat in one backend
// pass when the backend offers StatLog (see batchStats), so the batch does not
// pay one process per action. A failure there is not fatal: the per-action
// fallback in pathDocOf keeps today's behavior exactly.
func (e *env) pathBatch(repo *model.Repo, ids []string, mode statDisplay) int {
	backend, code := e.statBackend(mode)
	if code != 0 {
		return code
	}
	stats := e.batchStats(repo, backend, ids, mode)
	edges := engine.FoldEdges(repo, e.rel())
	w := newPathBatchWriter(e.out)
	var errs []pathBatchError
	for _, id := range ids {
		p, ok := repo.Issues[id]
		if !ok {
			errs = append(errs, pathBatchError{Issue: id, Error: errUnknownIssue})
			continue
		}
		d, err := e.pathDocOf(p, mode, backend, edges, stats)
		if err != nil {
			errs = append(errs, pathBatchError{Issue: id, Error: errStatFailed})
			continue
		}
		w.issue(d)
		if w.err != nil {
			break
		}
	}
	if err := w.finish(errs); err != nil {
		fmt.Fprint(e.errw, i18n.T("输出 JSON 失败: %v\n", err))
		return 1
	}
	if len(errs) > 0 {
		return 1
	}
	return 0
}

// pathBatchWriter streams the batch document one issue at a time, so a large
// `path --all` never holds the whole document — at 100k actions it is 35 MB of
// JSON and one pathDoc per action — before the first byte leaves. The bytes are
// exactly what writeJSON emits for pathBatchDoc: same key order, 2-space indent,
// no HTML escaping, trailing newline.
type pathBatchWriter struct {
	buf     *bufio.Writer
	enc     *json.Encoder
	scratch bytes.Buffer
	first   bool
	err     error
}

func newPathBatchWriter(out io.Writer) *pathBatchWriter {
	w := &pathBatchWriter{buf: bufio.NewWriterSize(out, 64*1024), first: true}
	w.enc = json.NewEncoder(&w.scratch)
	w.enc.SetEscapeHTML(false)
	w.enc.SetIndent("    ", "  ")
	_, w.err = fmt.Fprint(w.buf, "{\n  \"issues\": [")
	return w
}

// encode renders v at the current nesting level. One encoder and one scratch
// buffer are reused for every issue: 100k allocations of both is measurable.
func (w *pathBatchWriter) encode(v any) ([]byte, error) {
	w.scratch.Reset()
	if err := w.enc.Encode(v); err != nil {
		return nil, err
	}
	body := bytes.TrimSuffix(w.scratch.Bytes(), []byte("\n"))
	out := make([]byte, 0, 4+len(body))
	out = append(out, "    "...)
	out = append(out, body...)
	return out, nil
}

// issue writes one element of the issues array.
func (w *pathBatchWriter) issue(doc pathDoc) {
	if w.err != nil {
		return
	}
	body, err := w.encode(doc)
	if err != nil {
		w.err = err
		return
	}
	if w.first {
		w.first = false
		_, w.err = fmt.Fprint(w.buf, "\n")
	} else {
		_, w.err = fmt.Fprint(w.buf, ",\n")
	}
	if w.err == nil {
		_, w.err = w.buf.Write(body)
	}
}

// finish closes the issues array, adds the errors array when there is one, and
// closes the document.
func (w *pathBatchWriter) finish(errs []pathBatchError) error {
	if w.err != nil {
		return w.err
	}
	if w.first {
		if _, err := fmt.Fprint(w.buf, "]"); err != nil {
			return err
		}
	} else if _, err := fmt.Fprint(w.buf, "\n  ]"); err != nil {
		return err
	}
	if len(errs) > 0 {
		if _, err := fmt.Fprint(w.buf, ",\n  \"errors\": ["); err != nil {
			return err
		}
		for i, e := range errs {
			body, err := w.encode(e)
			if err != nil {
				return err
			}
			if i == 0 {
				_, err = fmt.Fprint(w.buf, "\n")
			} else {
				_, err = fmt.Fprint(w.buf, ",\n")
			}
			if err != nil {
				return err
			}
			if _, err := w.buf.Write(body); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w.buf, "\n  ]"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w.buf, "\n}\n"); err != nil {
		return err
	}
	return w.buf.Flush()
}

// batchStats fetches the diffstats of every action in the batch in one call. It
// returns nil when no stats were asked for, when the backend has no batch form
// (jj), or when the batch call failed — nil means "let pathDocOf ask per
// action", which is the pre-batch behavior and the safety net for both cases.
func (e *env) batchStats(repo *model.Repo, backend vcs.Repo, ids []string, mode statDisplay) map[string]string {
	if mode == statNone || backend == nil {
		return nil
	}
	sl, ok := backend.(vcs.StatLog)
	if !ok {
		return nil
	}
	want := make(map[string]bool)
	for _, id := range ids {
		p, ok := repo.Issues[id]
		if !ok {
			continue
		}
		for _, a := range p.Actions {
			want[a.ID] = true
		}
	}
	if len(want) == 0 {
		return nil
	}
	stats, err := sl.LogStats(mode == statShort, func(id string) bool { return want[id] })
	if err != nil {
		return nil
	}
	return stats
}

// pathDocOf fills one path document without writing it. The folded edges are
// passed in so a batch folds them once instead of once per issue; backend may be
// nil when no stats were asked for. stats is the batch lookup from batchStats:
// a hit renders without another backend call, a miss (a merge commit, or no
// batch at all) still asks ShowStat, so no action can silently lose its stat.
func (e *env) pathDocOf(p *model.Issue, mode statDisplay, backend vcs.Repo, edges []engine.Edge, stats map[string]string) (pathDoc, error) {
	doc := pathDoc{
		Issue:     p.ID,
		Action:    p.State(),
		Done:      e.isDone(p),
		Actions:   make([]pathAction, 0, len(p.Actions)),
		Relations: incidentEdges(edges, e.rel(), p.ID),
	}
	for i, a := range p.Actions {
		item := pathAction{
			Ordinal: i + 1,
			Action:  a.Kind,
			Commit:  a.ID,
			Date:    a.Date.Format(time.RFC3339),
			Title:   a.Title,
			Refs:    a.Refs,
			Tags:    a.Tags,
		}
		if mode != statNone {
			stat, ok := stats[a.ID]
			if !ok {
				var err error
				stat, err = backend.ShowStat(a.ID, mode == statShort)
				if err != nil {
					return pathDoc{}, err
				}
			}
			item.Stat = strings.TrimSpace(stat)
		}
		doc.Actions = append(doc.Actions, item)
	}
	return doc, nil
}

// statBackend opens the backend a stat-carrying view needs. A view without stats
// works straight from the aggregated history and opens nothing.
func (e *env) statBackend(mode statDisplay) (vcs.Repo, int) {
	if mode == statNone {
		return nil, 0
	}
	b, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return nil, 1
	}
	return b, 0
}

// incidentEdges returns the folded edges touching one issue, declared arrow and
// ordering included — the same shape `graph --json` writes.
func incidentEdges(edges []engine.Edge, rel *model.RelationSet, id string) []graphEdge {
	out := make([]graphEdge, 0, 2)
	for _, ed := range edges {
		if ed.From != id && ed.To != id {
			continue
		}
		out = append(out, graphEdge{
			From:     ed.From,
			To:       ed.To,
			Name:     ed.Name,
			Directed: rel.Inverse(ed.Name) != "",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// statDisplay selects how `path` annotates each action with its diffstat.
type statDisplay int

const (
	statNone statDisplay = iota
	statBlock
	statShort
)

// statMode reads the stat flags; the histogram (--stat) wins over --shortstat.
func statMode(f flags) statDisplay {
	switch {
	case f.has("stat"):
		return statBlock
	case f.has("shortstat"):
		return statShort
	}
	return statNone
}

// statIndent nests a diffstat block under the action line it belongs to.
const statIndent = "        "

// indentStat indents every line of a git diffstat block. Empty input yields
// nothing, so an empty diff never leaves a stray indented blank line.
func indentStat(stat string) string {
	stat = trimNewlines(stat)
	if stat == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(stat, "\n") {
		b.WriteString(statIndent)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// trimNewlines drops leading and trailing blank lines while preserving the
// interior layout of a rendered block.
func trimNewlines(s string) string { return strings.Trim(s, "\n") }

func (e *env) status(args []string) int {
	pos, f, _ := parse(args, "json")
	warnUnknownFlags(e.errw, f, allowedFlags("json"))
	if code := rejectSwitchValues(e.errw, f, "json"); code != 0 {
		return code
	}
	if code := rejectExtra(e.errw, pos, 0, "kiln status [--json]"); code != 0 {
		return code
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	areas := e.knownAreas()
	s := engine.Summarize(repo, e.actions)
	if f.has("json") {
		return e.statusJSON(repo, s, areas)
	}
	if s.Total == 0 {
		fmt.Fprint(e.out, i18n.T("问题总数: 0（完成 0 / 进行中 0）｜%s\n", worktreeSegment(e.worktree, areasOpen(areas))))
		fmt.Fprint(e.out, i18n.T("kiln 动作数: %d\n", s.Actions))
		fmt.Fprintln(e.out, i18n.T("提示: 用 `kiln do <issue> <action> --title <文本>` 生成规范提交信息"))
		e.renderWorktreeLegend(areas)
		return 0
	}
	groups := engine.GroupByDomain(repo)
	fmt.Fprint(e.out, i18n.T("问题总数: %d（完成 %d / 进行中 %d）｜domain %d 个｜%s\n", s.Total, s.Complete, s.InProgress, engine.CountDomains(groups), worktreeSegment(e.worktree, areasOpen(areas))))
	fmt.Fprint(e.out, i18n.T("kiln 动作数: %d\n\n", s.Actions))
	e.renderStatus(e.statusRows(groups, areaIssues(areas)))
	e.renderWorktreeLegend(areas)
	return 0
}

// statusRow is one issue in the domain view, in exactly the order the view
// prints it. Both renderings — the text rows and the JSON array — come out of
// this one walk, so their issue rows can never disagree about order, which is
// what lets a test compare them row for row.
type statusRow struct {
	issue *model.Issue
	head  string // the block header to print above this row; "" inside a block
	area  bool   // this issue has a worktree open
}

// statusRows lays the domain blocks out as a flat, ordered list of rows: one row
// per issue, the first row of each block carrying that block's header. The header
// is text only — the JSON rows carry their own `domain` — but it comes out of this
// same walk, so neither view can disagree about where a block starts.
func (e *env) statusRows(groups []engine.DomainGroup, areas map[string]bool) []statusRow {
	var rows []statusRow
	for _, g := range groups {
		for i, p := range g.Issues {
			head := ""
			if i == 0 {
				head = g.Domain
			}
			rows = append(rows, statusRow{issue: p, head: head, area: areas[p.ID]})
		}
	}
	return rows
}

// renderStatus prints the rows aligned by display width, with a blank line
// renderStatus prints the domain blocks: a header line naming the block, then one
// row per issue, the rows aligned by display width across every block. Headers
// stay out of that width calculation — one long domain name must not push the
// whole table right — and a header opens its block, so it takes the place of the
// blank line that used to sit between blocks.
func (e *env) renderStatus(rows []statusRow) {
	labels := make([]string, len(rows))
	width := 0
	for i, r := range rows {
		labels[i] = "  " + r.issue.ID
		if w := displayWidth(labels[i]); w > width {
			width = w
		}
	}
	for i, r := range rows {
		if r.head != "" {
			if i > 0 {
				fmt.Fprintln(e.out)
			}
			fmt.Fprintln(e.out, i18n.T(r.head))
		}
		mark := " "
		if e.isDone(r.issue) {
			mark = "✓"
		}
		fmt.Fprintf(e.out, "%s %s %s%s\n", pad(labels[i], width), mark, stateWord(r.issue), areaMark(r.area))
	}
}

// AreaMark is the suffix that flags an issue as having a worktree open. U+25A0
// (■) is the pick out of the geometric shapes: it is present in every font tested
// except one vendored subset, and — unlike the ✓ this view already prints — it
// survives in the very fonts where that ✓ is missing, so the new mark never
// becomes the weakest link.
const AreaMark = "■"

// areaMark returns the marker cell for one row; rows without an area stay a
// character shorter rather than carrying an empty column.
func areaMark(hasArea bool) string {
	if hasArea {
		return "  " + AreaMark
	}
	return ""
}

// areaIssues lists the issues that currently have a worktree open, which is what
// the marker in the domain view marks.
func areaIssues(areas []vcs.WorkspaceInfo) map[string]bool {
	set := make(map[string]bool, len(areas))
	for _, a := range areas {
		if a.Issue != "" {
			set[a.Issue] = true
		}
	}
	return set
}

// areasOpen counts the dedicated areas in a listing; the main checkout is not one
// of them, so it never counts as an open worktree.
func areasOpen(areas []vcs.WorkspaceInfo) int {
	n := 0
	for _, a := range areas {
		if !a.Main {
			n++
		}
	}
	return n
}

// knownAreas lists the repository's working areas. It is best effort on purpose:
// status is a read view, and an area list that cannot be read must not fail it —
// the view then simply says zero, which is also what an empty repository has.
// A backend that can prove it has no linked areas (AreaHint) skips the listing
// process entirely; the hint may only ever say "none", so it cannot hide one.
func (e *env) knownAreas() []vcs.WorkspaceInfo {
	backend, err := e.backend()
	if err != nil {
		return nil
	}
	ws, ok := backend.(vcs.Workspace)
	if !ok {
		return nil
	}
	if hint, ok := backend.(vcs.AreaHint); ok && !hint.MaybeHasAreas() {
		return nil
	}
	areas, err := ws.ListWorkspaces()
	if err != nil {
		return nil
	}
	return areas
}

// worktreeSegment reports the isolation mode and how many areas are open — the
// repository-level half of the worktree state, which is why it is shown even when
// the mode is off and the count is zero: it answers "did this repository ask for
// isolation", and a segment whose shape never changes reads better than one that
// comes and goes.
func worktreeSegment(mode string, areas int) string {
	return i18n.T("worktree %s（已开 %d 个）", worktreeWord(mode), areas)
}

// worktreeWord is the effective isolation mode: an unset value means on, which is
// the repository default. Both views state the mode from this one place, so the
// text header and the JSON document cannot disagree about it.
func worktreeWord(mode string) string {
	if mode == config.WorktreeOff {
		return "off"
	}
	return "on"
}

// renderWorktreeLegend explains the marker — and names the area the caller is
// standing in — but only when the repository has one open. A legend for a mark
// that never appears would just be noise.
// renderWorktreeLegend explains the ■ mark. It is printed only when a dedicated
// area is open: the main checkout is always listed, and a legend for zero areas
// is noise (the same contract TestStatusStatesWorktreeOffWithNoAreas states).
func (e *env) renderWorktreeLegend(areas []vcs.WorkspaceInfo) {
	if areasOpen(areas) == 0 {
		return
	}
	line := AreaMark + i18n.T(" = 已开 worktree——各自状态见 kiln worktree list")
	if here, ok := vcs.AreaAt(areas, "."); ok && !here.Main {
		line += i18n.T("｜当前：worktree ") + here.Issue
	}
	fmt.Fprintln(e.out)
	fmt.Fprintln(e.out, line)
}

// normalizedIssue folds the spellings that mean the same issue and says so when it
// did. Normalizing on the **write** side — and only there — is what keeps one
// issue from being split in two by case, '_' or a doubled '-': the trailer value,
// the worktree name and its branch all come out of this one call.
func (e *env) normalizedIssue(raw string) string {
	issue := model.NormalizeIssueID(raw)
	if issue != raw {
		fmt.Fprint(e.errw, i18n.T("提示: issue ID 已归一化：%q → %q\n", raw, issue))
	}
	return issue
}

// refuseDuplicateStart stops a second start action on an issue that already has
// one: with a flat ID space and one person per repository, the same name is the
// same issue, so there is nothing to open — the honest answer is "continue it".
//
// An issue that exists **without** a start action still goes through: adding the
// missing start to such an issue is a repair (check already flags it as an error),
// not a new issue.
func (e *env) refuseDuplicateStart(issue string) int {
	p, ok := e.findIssue(issue)
	if !ok || !e.hasStart(p) {
		return 0
	}
	rest := make([]string, 0, len(e.actions.Names()))
	for _, name := range e.actions.Names() {
		if !e.actions.IsStart(name) {
			rest = append(rest, name)
		}
	}
	fmt.Fprint(e.errw, i18n.T("拒绝: 问题 %q 已经有起点动作 %q（共 %d 个动作）——同名即同一条问题，不需要再新建\n", issue, e.actions.Start(), len(p.Actions)))
	fmt.Fprint(e.errw, i18n.T("提示: 要继续它，用 kiln commit %s <action> --title <文本>（可选: %s）\n", issue, strings.Join(rest, ", ")))
	return 1
}

// hasStart reports whether the issue has been opened already.
func (e *env) hasStart(p *model.Issue) bool {
	for _, a := range p.Actions {
		if e.actions.IsStart(a.Kind) {
			return true
		}
	}
	return false
}

// findIssue reads one issue out of the history, best effort on purpose: `do` only
// prints a message and must keep working outside a repository, so an unreadable
// history means "no opinion", never an error.
func (e *env) findIssue(issue string) (*model.Issue, bool) {
	backend, err := e.backend()
	if err != nil {
		return nil, false
	}
	commits, err := backend.Log()
	if err != nil {
		return nil, false
	}
	p, ok := engine.Build(engine.ActionsFromCommits(commits, e.rel()), e.rel()).Issues[issue]
	return p, ok
}

func (e *env) isDone(p *model.Issue) bool {
	last, ok := p.Last()
	return ok && e.actions.IsTerminal(last.Kind)
}

// filterStatus keeps the issues whose current step is (`done`) or is not
// (`open`) a terminal action. It reads the same predicate as the row's `done`,
// so a filter and the rows it returns can never disagree.
func (e *env) filterStatus(repo *model.Repo, ids []string, done bool) []string {
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		if e.isDone(repo.Issues[id]) == done {
			kept = append(kept, id)
		}
	}
	return kept
}

// filterSearch keeps the issues whose id, current title or any action subject
// contains every term, case-insensitively. The query is an AND of literal
// fragments, not a regular expression: that is the convention for issue/record
// search (gh, git-bug), while regex stays opt-in elsewhere.
func (e *env) filterSearch(repo *model.Repo, ids []string, terms []string) []string {
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		if issueMatches(repo.Issues[id], terms) {
			kept = append(kept, id)
		}
	}
	return kept
}

// issueMatches joins the searchable text of one issue — id, current title and
// every action subject, the same words the views show — and asks whether every
// term occurs somewhere in it. A separator keeps matching from crossing fields.
func issueMatches(p *model.Issue, terms []string) bool {
	fields := make([]string, 0, len(p.Actions)+2)
	fields = append(fields, p.ID, issueTitle(p))
	for _, a := range p.Actions {
		fields = append(fields, a.Subject)
	}
	return containsAll(strings.Join(fields, "\x00"), terms)
}

// searchHit names the action that justifies a hit the row does not already
// show. When id + title contain every term the row explains itself and hit is
// nil; otherwise the earliest action subject holding all terms is returned. If
// the terms are spread over several actions, no single action explains the
// query and hit is nil too — better silent than pointing at the wrong line.
func searchHit(p *model.Issue, terms []string) *listHit {
	if len(terms) == 0 || containsAll(p.ID+"\x00"+issueTitle(p), terms) {
		return nil
	}
	for i, a := range p.Actions {
		if containsAll(a.Subject, terms) {
			return &listHit{
				Ordinal: i + 1,
				Action:  a.Kind,
				Commit:  a.ID,
				Subject: a.Subject,
				Total:   len(p.Actions),
				Date:    a.Date.Format(time.RFC3339),
			}
		}
	}
	return nil
}

// searchRank buckets a match the way the caller orders search results: 0 is an
// id that starts with the first term (the other terms may live in the title or
// an action), 1 an id holding every term, 2 the current title, 3 one action
// subject, and 4 a match spread over several fields. It assumes the issue
// already passed filterSearch; rank 4 is a fallback, not a second filter.
func searchRank(p *model.Issue, terms []string) int {
	if strings.HasPrefix(strings.ToLower(p.ID), strings.ToLower(terms[0])) {
		return 0
	}
	if containsAll(p.ID, terms) {
		return 1
	}
	if containsAll(issueTitle(p), terms) {
		return 2
	}
	for _, a := range p.Actions {
		if containsAll(a.Subject, terms) {
			return 3
		}
	}
	return 4
}

// searchRow is one issue in a search result: the issue, its rank, and the hit
// that explains the match. Both the JSON and the text view walk one slice of
// these, so the two orders cannot drift apart.
type searchRow struct {
	p    *model.Issue
	rank int
	hit  *listHit
}

// lastDate is the issue's most recent action date, the search tie-break key.
func lastDate(p *model.Issue) time.Time {
	if a, ok := p.Last(); ok {
		return a.Date
	}
	return time.Time{}
}

// lessSearchRow orders search results by rank, then newest last action first,
// then id. Dates compare as times, not strings; a missing date sorts after every
// known one so the order stays total.
func lessSearchRow(a, b searchRow) bool {
	if a.rank != b.rank {
		return a.rank < b.rank
	}
	ad, bd := lastDate(a.p), lastDate(b.p)
	switch {
	case ad.IsZero() != bd.IsZero():
		return bd.IsZero()
	case !ad.Equal(bd):
		return ad.After(bd)
	default:
		return a.p.ID < b.p.ID
	}
}

// containsAll reports whether every term occurs in haystack, case-insensitively.
func containsAll(haystack string, terms []string) bool {
	h := strings.ToLower(haystack)
	for _, term := range terms {
		if !strings.Contains(h, strings.ToLower(term)) {
			return false
		}
	}
	return true
}

func (e *env) list(args []string) int {
	pos, f, missing := parse(args, "json")
	allowed := allowedFlags("json", "domain", "tag", "action", "status", "search")
	if code := rejectMissingValues(e.errw, missing, allowed); code != 0 {
		return code
	}
	warnUnknownFlags(e.errw, f, allowed)
	if code := rejectSwitchValues(e.errw, f, "json"); code != 0 {
		return code
	}
	if code := rejectRepeats(e.errw, f, "domain", "tag", "action", "status", "search"); code != 0 {
		return code
	}
	if code := rejectExtra(e.errw, pos, 0, "kiln list [--domain=] [--tag=] [--action=] [--status=open|done] [--search=] [--json]"); code != 0 {
		return code
	}
	status := f.get("status")
	if f.has("status") && status != "open" && status != "done" {
		fmt.Fprint(e.errw, i18n.T("--status 只接受 open 或 done（当前: %s）\n", status))
		return 2
	}
	// The action vocabulary is the same closed set `do` validates against, so a
	// typo here is a refusal, not an empty result.
	action := strings.ToLower(strings.TrimSpace(f.get("action")))
	if action != "" && !e.actions.IsValid(action) {
		fmt.Fprint(e.errw, i18n.T("非法 action: %q（可选: %s）\n", action, strings.Join(e.actions.Names(), ", ")))
		return 2
	}
	// A tag is one literal value here; `do` splits commas into several tags, and
	// silently treating "a,b" as one tag returns zero rows that look like a miss.
	tag := f.get("tag")
	if strings.Contains(tag, ",") {
		fmt.Fprint(e.errw, i18n.T("--tag 只接受一个标签（当前: %q）；多个标签暂不支持\n", tag))
		return 2
	}
	query := f.get("search")
	terms := strings.Fields(query)
	if f.has("search") && len(terms) == 0 {
		fmt.Fprintln(e.errw, i18n.T("--search 需要一个非空查询串"))
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	opts := engine.FilterOptions{
		Domain: f.get("domain"),
		Tag:    tag,
		Action: action,
	}
	ids := engine.Filter(repo, opts)
	if status != "" {
		ids = e.filterStatus(repo, ids, status == "done")
	}
	if len(terms) > 0 {
		ids = e.filterSearch(repo, ids, terms)
	}
	// An empty result is a valid answer (exit 0), but silence reads like a broken
	// command. The hint goes to stderr in both views, so a JSON consumer still
	// gets exactly one document on stdout.
	if len(ids) == 0 {
		switch {
		case len(terms) > 0:
			fmt.Fprint(e.errw, i18n.T("没有匹配 %q 的问题\n", query))
		case len(repo.Issues) == 0:
			fmt.Fprint(e.errw, i18n.T("仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）\n"))
		default:
			fmt.Fprint(e.errw, i18n.T("没有匹配 %s 的问题\n", filterSummary(opts, status)))
		}
	}
	filter := filterQuery{Domain: opts.Domain, Tag: opts.Tag, Action: opts.Action, Status: status, Search: query}
	// Search is the only view that reorders: rank first, then the newest last
	// action, then id. Rows are built once so the text and JSON views below walk
	// the same order and share the same hit.
	if len(terms) > 0 {
		rows := make([]searchRow, 0, len(ids))
		for _, id := range ids {
			p := repo.Issues[id]
			rows = append(rows, searchRow{p: p, rank: searchRank(p, terms), hit: searchHit(p, terms)})
		}
		sort.Slice(rows, func(i, j int) bool { return lessSearchRow(rows[i], rows[j]) })
		if f.has("json") {
			doc := listDoc{Filter: filter, Count: len(rows), Issues: make([]listIssue, 0, len(rows))}
			for _, r := range rows {
				rank := r.rank
				doc.Issues = append(doc.Issues, listIssue{
					issueCore: issueCore{ID: r.p.ID, Domain: r.p.Domain, Action: r.p.State(), Done: e.isDone(r.p)},
					Rank:      &rank,
					Title:     issueTitle(r.p),
					Commit:    issueCommit(r.p),
					Hit:       r.hit,
					Tags:      r.p.Tags,
				})
			}
			return e.writeJSON(doc)
		}
		for _, r := range rows {
			line := fmt.Sprintf("%s %s %s", pad(r.p.ID, 30), pad(stateWord(r.p), 12), issueTitle(r.p))
			if r.hit != nil {
				line += "  ← " + r.hit.Subject
			}
			fmt.Fprintln(e.out, line)
		}
		return 0
	}
	if f.has("json") {
		doc := listDoc{
			Filter: filter,
			Count:  len(ids),
			Issues: make([]listIssue, 0, len(ids)),
		}
		for _, id := range ids {
			p := repo.Issues[id]
			doc.Issues = append(doc.Issues, listIssue{
				issueCore: issueCore{ID: p.ID, Domain: p.Domain, Action: p.State(), Done: e.isDone(p)},
				Title:     issueTitle(p),
				Commit:    issueCommit(p),
				Tags:      p.Tags,
			})
		}
		return e.writeJSON(doc)
	}
	for _, id := range ids {
		p := repo.Issues[id]
		fmt.Fprintf(e.out, "%s %s %s\n", pad(p.ID, 30), pad(stateWord(p), 12), tagsSuffix(p))
	}
	return 0
}

// filterSummary names the conditions actually in force, in the help's order, so
// an empty result says what matched nothing instead of nothing at all.
func filterSummary(o engine.FilterOptions, status string) string {
	var parts []string
	if o.Domain != "" {
		parts = append(parts, "domain="+o.Domain)
	}
	if o.Tag != "" {
		parts = append(parts, "tag="+o.Tag)
	}
	if o.Action != "" {
		parts = append(parts, "action="+o.Action)
	}
	if status != "" {
		parts = append(parts, "status="+status)
	}
	return strings.Join(parts, " ")
}

// loadRepo reads history and aggregates it; returns (nil, exitCode) on failure.
func (e *env) loadRepo() (*model.Repo, int) {
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return nil, 1
	}
	if repo, ok := e.loadCached(backend); ok {
		return repo, 0
	}
	commits, err := backend.Log()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取历史失败: %v\n", err))
		return nil, 1
	}
	actions, records := engine.ActionsAndRecordsFromCommits(commits, e.rel())
	if len(commits) > 0 {
		tip, _ := decodeOID(commits[0].ID)
		e.persistRecords(backend, records, tip)
		// The scan already holds every commit in git's order: seed the enumeration
		// snapshot so the next read does not have to ask git for the same list.
		e.storeEnumFromCommits(commits)
	}
	return engine.Build(actions, e.rel()), 0
}

// statusDoc is the `kiln status --json` document: the three things the text view
// states — repository counters, the isolation state, and the issues.
type statusDoc struct {
	Summary  statusSummary `json:"summary"`
	Worktree worktreeState `json:"worktree"`
	Issues   []statusIssue `json:"issues"`
}

type statusSummary struct {
	Total   int `json:"total"`
	Done    int `json:"done"`
	Open    int `json:"open"`
	Actions int `json:"actions"`
	Domains int `json:"domains"`
}

type worktreeState struct {
	Policy  string `json:"policy"`
	Open    int    `json:"open"`
	Current string `json:"current,omitempty"`
}

// statusIssue is one flat row of the domain view. It carries no action list —
// that is `path`'s job — which keeps the document O(issues), not O(commits).
type statusIssue struct {
	issueCore
	Title       string `json:"title,omitempty"`
	HasWorktree bool   `json:"has-worktree"`
}

// listDoc is the `kiln list --json` document.
type listDoc struct {
	Filter filterQuery `json:"filter"`
	Count  int         `json:"count"`
	Issues []listIssue `json:"issues"`
}

// filterQuery echoes the conditions that were asked for; an absent key means
// that condition was not applied.
type filterQuery struct {
	Domain string `json:"domain,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Action string `json:"action,omitempty"`
	Status string `json:"status,omitempty"`
	Search string `json:"search,omitempty"`
}

type listIssue struct {
	issueCore
	Rank   *int     `json:"rank,omitempty"`
	Title  string   `json:"title,omitempty"`
	Commit string   `json:"commit,omitempty"`
	Hit    *listHit `json:"hit,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// listHit is the action a search matched when the row's id and title cannot
// explain it: the ordinal addresses it in `kiln show`, the commit addresses it
// in git. It is a pointer so an unexplained search omits the key entirely.
type listHit struct {
	Ordinal int    `json:"ordinal"`
	Action  string `json:"action"`
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
	Total   int    `json:"total"`
	Date    string `json:"date"`
}

// actionListDoc is `kiln action list --json`: the effective vocabulary, mirroring
// the config's actions block, plus where it came from. Config is omitted when
// the defaults are in force; Source is always present.
type actionListDoc struct {
	Actions actionSetDoc `json:"actions"`
	Source  string       `json:"source"`
	Config  string       `json:"config,omitempty"`
}

type actionSetDoc struct {
	Names    []string `json:"names"`
	Start    string   `json:"start"`
	Terminal []string `json:"terminal"`
}

// actionLogDoc is one NDJSON record of `kiln action log --jsonl`. There is no
// envelope because every line is the same kind of record; the four identity keys
// are always written, and the descriptive ones are omitted when absent — an
// absent key means "none", never an empty string. date keeps the commit's own
// offset so a consumer never has to guess the timezone.
//
// Relations are the action's own declarations, not the folded graph: `has-part:
// X` stays {name: has-part, target: X} instead of being flipped into an edge.
// Folding is `graph` / `path`'s job, and doing it here would need the vocabulary
// at emit time and would silently drop the "this action said so" half. Names are
// canonical (aliases resolved at parse time) and unknown keys never reach this
// field — they are `check`'s unknown-trailer finding.
type actionLogDoc struct {
	Issue     string              `json:"issue"`
	Action    string              `json:"action"`
	Commit    string              `json:"commit"`
	Date      string              `json:"date"`
	Title     string              `json:"title,omitempty"`
	Domain    string              `json:"domain,omitempty"`
	Tags      []string            `json:"tags,omitempty"`
	Refs      []string            `json:"refs,omitempty"`
	Target    []string            `json:"target,omitempty"`
	Scope     string              `json:"scope,omitempty"`
	Relations []actionLogRelation `json:"relations,omitempty"`
	Unrelate  []string            `json:"unrelate,omitempty"`
}

// actionLogRelation is one declared relation: the canonical name and its target
// issue, exactly as this action wrote them.
type actionLogRelation struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// statusJSON renders the domain view as a flat issue list, in the very order the
// text view prints its rows — both renderings come from one walk, so their issue
// rows line up one for one. The text view's block headers have no counterpart
// here: every row already carries its `domain`, and a heading would be a second
// way to say the same thing.
func (e *env) statusJSON(repo *model.Repo, s engine.Stats, areas []vcs.WorkspaceInfo) int {
	groups := engine.GroupByDomain(repo)
	doc := statusDoc{
		Summary: statusSummary{
			Total:   s.Total,
			Done:    s.Complete,
			Open:    s.InProgress,
			Actions: s.Actions,
			Domains: engine.CountDomains(groups),
		},
		Worktree: e.worktreeState(areas),
		Issues:   make([]statusIssue, 0, s.Total),
	}
	for _, r := range e.statusRows(groups, areaIssues(areas)) {
		title := ""
		if last, ok := r.issue.Last(); ok {
			title = last.Title
		}
		doc.Issues = append(doc.Issues, statusIssue{
			issueCore: issueCore{
				ID:     r.issue.ID,
				Domain: r.issue.Domain,
				Action: r.issue.State(),
				Done:   e.isDone(r.issue),
			},
			Title:       title,
			HasWorktree: r.area,
		})
	}
	return e.writeJSON(doc)
}

// worktreeState states the repository-level isolation mode and how many areas are
// open, plus the area the caller stands in when there is one.
func (e *env) worktreeState(areas []vcs.WorkspaceInfo) worktreeState {
	ws := worktreeState{Policy: worktreeWord(e.worktree), Open: areasOpen(areas)}
	if here, ok := vcs.AreaAt(areas, "."); ok && !here.Main {
		ws.Current = here.Issue
	}
	return ws
}
