package cli

import (
	"fmt"
	"strings"

	"github.com/jackiehank/kiln/internal/config"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/vcs"
)

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
