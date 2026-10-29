package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackiehank/kiln/internal/config"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/trailer"
	"github.com/jackiehank/kiln/internal/vcs"
)

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
// history means "no opinion", never an error. It answers from the record layer
// when the cache can serve it: the duplicate-start check used to read the whole
// history (every message, via `git log`) on every attempt, which is exactly the
// work the cache exists to skip. The cold fallback goes through the shared
// loadUncached, so even the first answer seeds the cache the next check uses.
func (e *env) findIssue(issue string) (*model.Issue, bool) {
	backend, err := e.backend()
	if err != nil {
		return nil, false
	}
	if repo, ok := e.loadCached(backend); ok {
		p, found := repo.Issues[issue]
		return p, found
	}
	repo, err := e.loadUncached(backend)
	if err != nil {
		return nil, false
	}
	p, ok := repo.Issues[issue]
	return p, ok
}
