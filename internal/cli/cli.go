// Package cli wires the command surface: argument parsing, git access and text
// rendering. All output goes through the writers passed to Run, so the package
// stays testable.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"kiln/internal/config"
	"kiln/internal/engine"
	"kiln/internal/model"
	"kiln/internal/trailer"
	"kiln/internal/vcs"
	"kiln/internal/vcs/git"
	"kiln/internal/vcs/jj"
)

const usageHeader = `kiln — git-trailer 驱动的问题追踪与路径重建

用法:
`

const usageFooter = `
action 词表默认: ask | hypothesize | collect | test | conclude | apply
worktree 默认: on（动作只在 worktree 里提交——开工前先 kiln worktree new <issue>）| off（允许在 main 提交）
（配置：仓库根的 .kilnconfig.jsonc，JSONC；用 kiln config path 打印路径）

do/commit 的 flags:
  --title  --refs  --target  --scope  --domain  --tag
  --derived-from  --spawns  --relates-to  --blocks  --supersedes

path 的 flags:  --stat（每动作附文件统计）  --shortstat（每动作一行总计）
show 的 flags:  --stat（只给该动作的文件统计）
status / list / check 的 flags:  --json（机器可读输出：stdout 只放一个 JSON 文档）
commit 额外 flag:  --force（一次性豁免：worktree=on 时放行当次在 main 提交）
worktree remove 额外 flag:  --force（连同未合并的分支 / 尚有改动的 worktree 一起删）
`

// retiredCommands maps a name kiln no longer answers to its current spelling, so
// a hard cut still tells the caller where the command went.
var retiredCommands = map[string]string{"workspace": "worktree"}

// usageCommands is the command surface rendered by usageText.
var usageCommands = [][2]string{
	{"kiln do <issue> <action> [flags]", "生成规范提交信息（不写文件）"},
	{"kiln commit <issue> <action> [flags]", "用规范信息直接提交"},
	{"kiln path <issue>", "重建问题的研究路径（可加 --stat / --shortstat）"},
	{"kiln show <issue> [n]", "展开某个动作的完整变更（可加 --stat）"},
	{"kiln status", "全库概览"},
	{"kiln check [--verbose]", "语义完整性校验"},
	{"kiln list [--domain=] [--tag=] [--action=]", "按条件过滤问题"},
	{"kiln graph [--format=ascii]", "问题关系图"},
	{"kiln worktree new <issue>", "建 worktree（main 只读，动作只在 worktree 提交）"},
	{"kiln worktree list", "列出全部 worktree（含 main）及各自状态"},
	{"kiln worktree remove <issue>", "删除 worktree（尚有工作的默认拒绝，--force 强删）"},
	{"kiln config path", "打印配置路径（缺失时写入一份带注释的默认配置）"},
}

// usageText renders the help text, aligning descriptions by terminal display
// width so full-width CJK text does not skew the columns.
func usageText() string {
	width := 0
	for _, c := range usageCommands {
		if w := displayWidth(c[0]); w > width {
			width = w
		}
	}
	var b strings.Builder
	b.WriteString(usageHeader)
	for _, c := range usageCommands {
		b.WriteString("  " + pad(c[0], width) + "  " + c[1] + "\n")
	}
	b.WriteString(usageFooter)
	return b.String()
}

// pad right-pads s with spaces until it spans at least width display columns.
func pad(s string, width int) string {
	if d := width - displayWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// displayWidth returns the number of terminal columns s occupies, counting East
// Asian wide characters as two (a practical subset of UAX #11).
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// isWide reports whether r spans two columns in the common CJK ranges.
func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r >= 0x2E80 && r <= 0x303E: // CJK radicals / Kangxi / CJK symbols
		return true
	case r >= 0x3041 && r <= 0x33FF: // kana, CJK compatibility
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK extension A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK unified ideographs
		return true
	case r >= 0xA000 && r <= 0xA4CF: // Yi
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK compatibility ideographs
		return true
	case r >= 0xFE10 && r <= 0xFE19: // vertical forms
		return true
	case r >= 0xFE30 && r <= 0xFE6F: // CJK compatibility forms
		return true
	case r >= 0xFF00 && r <= 0xFF60: // fullwidth forms
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // fullwidth signs
		return true
	}
	return false
}

// env carries the resolved action vocabulary, the repository backend and the
// output streams. The backend is an interface so tests can substitute a fake
// instead of touching a real repository; it is opened lazily, so commands that
// need no repo (do) still work outside one.
type env struct {
	actions  *model.ActionSet
	dir      string
	vcsPref  string
	worktree string
	repo     vcs.Repo
	kind     vcs.Kind // the backend that was opened, recorded for backend-specific wording
	out      io.Writer
	errw     io.Writer
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
	return nil, "", fmt.Errorf("未知后端: %s", kind)
}

// Run executes one command and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText())
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usageText())
		return 0
	case "do", "commit", "path", "status", "check", "list", "graph", "show", "worktree", "config":
		// handled below, once the vocabulary is loaded
	default:
		if now, ok := retiredCommands[cmd]; ok {
			fmt.Fprintf(stderr, "未知命令: %s（已改名为 `%s`）\n\n%s", cmd, now, usageText())
			return 2
		}
		fmt.Fprintf(stderr, "未知命令: %s\n\n%s", cmd, usageText())
		return 2
	}

	cfg, err := config.Read(".")
	if err != nil {
		fmt.Fprintf(stderr, "配置错误: %v\n", err)
		return 1
	}
	e := &env{actions: cfg.Actions, dir: ".", vcsPref: cfg.VCS, worktree: cfg.Worktree, out: stdout, errw: stderr}
	switch cmd {
	case "do":
		return e.write(rest, false)
	case "commit":
		return e.write(rest, true)
	case "path":
		return e.path(rest)
	case "show":
		return e.show(rest)
	case "status":
		return e.status(rest)
	case "check":
		return e.check(rest)
	case "list":
		return e.list(rest)
	case "graph":
		return e.graph(rest)
	case "worktree":
		return e.workspace(rest)
	case "config":
		return e.config(rest)
	}
	return 2
}

// flags holds repeatable --key value pairs parsed from a command's arguments.
type flags map[string][]string

func (f flags) get(key string) string {
	if v := f[key]; len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

// has reports whether a (possibly value-less) flag was supplied at all.
func (f flags) has(key string) bool { return len(f[key]) > 0 }

// list returns the values for a list-valued key, flattening comma-separated
// strings: each collected value is split on commas, each item trimmed, and
// blank items dropped. This makes `--refs a,b` and `--refs a --refs b` produce
// the same canonical output.
func (f flags) list(key string) []string {
	var out []string
	for _, v := range f[key] {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// parse splits args into positionals and flags. `--key=value`, `--key value` and
// repeated `--key` all accumulate. A trailing flag with no value becomes "".
func parse(args []string) ([]string, flags) {
	pos := make([]string, 0, len(args))
	f := flags{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		body := a[2:]
		key, val := body, ""
		if j := strings.Index(body, "="); j >= 0 {
			key, val = body[:j], body[j+1:]
		} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			val = args[i+1]
			i++
		}
		f[key] = append(f[key], val)
	}
	return pos, f
}

// write implements both `do` (print only) and `commit` (print and commit).
func (e *env) write(args []string, commit bool) int {
	pos, f := parse(args)
	if len(pos) < 2 {
		fmt.Fprintf(e.errw, "用法: kiln %s <issue> <action> [flags]\n", modeName(commit))
		return 2
	}
	if len(pos) > 2 {
		fmt.Fprintf(e.errw, "用法: kiln %s <issue> <action> [flags]：object 已退役，第三个参数已不再支持\n", modeName(commit))
		return 2
	}
	issue := e.normalizedIssue(pos[0])
	action := strings.ToLower(strings.TrimSpace(pos[1]))
	if !e.actions.IsValid(action) {
		fmt.Fprintf(e.errw, "非法 action: %q（可选: %s）\n", action, strings.Join(e.actions.Names(), ", "))
		return 2
	}
	if !model.ValidIssueID(issue) {
		fmt.Fprintf(e.errw, "警告: issue ID %q 不符合单段 kebab-case slug 约定（小写字母 / 数字 / `-`，长度 ≤ 50）\n", issue)
	}
	if e.actions.IsStart(action) {
		if code := e.refuseDuplicateStart(issue); code != 0 {
			return code
		}
	}

	msg := trailer.Format(trailer.Fields{
		Issue:       issue,
		Action:      action,
		Title:       f.get("title"),
		Refs:        f.list("refs"),
		Target:      f.get("target"),
		Scope:       f.get("scope"),
		Domain:      f.get("domain"),
		Tags:        f.list("tag"),
		DerivedFrom: f.list("derived-from"),
		Spawns:      f.list("spawns"),
		RelatesTo:   f.list("relates-to"),
		Blocks:      f.list("blocks"),
		Supersedes:  f.list("supersedes"),
	})
	if !commit {
		fmt.Fprintln(e.out, msg)
		if tip := e.worktreeTip(issue); tip != "" {
			fmt.Fprintln(e.errw, tip)
		}
		return 0
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprintf(e.errw, "打开仓库失败: %v\n", err)
		return 1
	}
	if !f.has("force") {
		if code := e.refuseInMain(backend, issue); code != 0 {
			return code
		}
	}
	if err := backend.Action(msg); err != nil {
		fmt.Fprintf(e.errw, "提交失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(e.out, msg)
	return 0
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
		return 0 // worktree: "off" — the repository opted out of isolation
	}
	ws, ok := backend.(vcs.Workspace)
	if !ok {
		return 0
	}
	dedicated, err := ws.IsDedicated()
	if err != nil {
		fmt.Fprintf(e.errw, "无法判定 worktree 类型: %v\n", err)
		return 1
	}
	if dedicated {
		return 0
	}
	fmt.Fprintln(e.errw, "拒绝提交：当前是 main（只读/只合并），动作只在 worktree 里提交。")
	if dirty, err := ws.Dirty(); err == nil && dirty {
		fmt.Fprintln(e.errw, "你已有未提交改动——新区是干净检出，直接建区会把改动留在 main：")
		if recipe := e.carryRecipe(issue, ws); recipe != "" {
			fmt.Fprintln(e.errw, recipe)
		}
		// --force stays documented, but it must not read as the cheaper
		// alternative to the recipe: it only lets the guard through, it does not
		// move anything, so it is offered as a condition you have to *want* —
		// otherwise an agent optimizing for "make the command succeed" takes the
		// one flag and leaves the work sitting in main.
		fmt.Fprintln(e.errw, "（--force 只放行护栏、不把改动搬进区——只有你确实要把它留在 main 时才用。）")
		return 1
	}
	fmt.Fprintf(e.errw, "先 `kiln worktree new %s`，再 cd 到 worktree 提交。\n", issue)
	fmt.Fprintln(e.errw, "确要在 main 提交时加 --force（一次性豁免）。")
	return 1
}

// isolated reports whether the repository still requires a dedicated area for
// actions. An unset value means on: the guard holds unless a repository turned
// it off by name.
func (e *env) isolated() bool { return e.worktree != config.WorktreeOff }

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
			return "提示: " + err.Error()
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
	head := "提示: 本仓要求动作在 worktree 里提交——开工前先 kiln worktree new " + issue
	dirty, err := ws.Dirty()
	if err != nil || !dirty {
		return head
	}
	recipe := e.carryRecipe(issue, ws)
	if recipe == "" {
		return head
	}
	return "提示: 本仓要求动作在 worktree 里提交；你已有未提交改动——" + recipe
}

// carryRecipe names the commands that move main's uncommitted changes into a new
// area, per backend. Nothing is carried by kiln itself (docs/worktree-adopt.md
// settled that): the recipe is the backend's own, and both avoid rewriting
// history, which is the trap on the jj side.
func (e *env) carryRecipe(issue string, ws vcs.Workspace) string {
	switch e.kind {
	case vcs.Git:
		// The stash is repository-wide, so it can be popped from inside the area.
		return fmt.Sprintf("git：git stash -u → kiln worktree new %s → 进区后 git stash pop", issue)
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
			return fmt.Sprintf("jj：先在仓库根 mkdir %s，再 jj workspace add --name %s <区路径> -r @（新区即带上它），最后在 main 跑 jj edit @-", dir, issue)
		}
		return fmt.Sprintf("jj：先在仓库根 mkdir %s，再 jj workspace add --name %s %s -r @（新区即带上它），最后在 main 跑 jj edit @-",
			dir, issue, vcs.AreaPath(mainRoot, issue))
	}
	return ""
}

// workspace implements `kiln worktree new <issue>` and `kiln worktree list`.
func (e *env) workspace(args []string) int {
	sub := ""
	rest := args
	if len(args) > 0 {
		sub, rest = args[0], args[1:]
	}
	ws, code := e.workspaceBackend()
	if ws == nil {
		return code
	}
	switch sub {
	case "new":
		return e.workspaceNew(ws, rest)
	case "list":
		return e.workspaceList(ws)
	case "remove":
		return e.workspaceRemove(ws, rest)
	default:
		fmt.Fprintln(e.errw, "用法: kiln worktree new <issue> | kiln worktree list | kiln worktree remove <issue> [--force]")
		return 2
	}
}

// workspaceBackend opens the backend and requires workspace support.
func (e *env) workspaceBackend() (vcs.Workspace, int) {
	backend, err := e.backend()
	if err != nil {
		fmt.Fprintf(e.errw, "打开仓库失败: %v\n", err)
		return nil, 1
	}
	ws, ok := backend.(vcs.Workspace)
	if !ok {
		fmt.Fprintln(e.errw, "当前后端不支持 worktree")
		return nil, 1
	}
	return ws, 0
}

// workspaceNew creates (or reuses) the dedicated area bound to an issue and
// points the user at it. The area's name and branch are derived from the issue,
// so naming alone is the binding — no extra file keeps them in sync.
func (e *env) workspaceNew(ws vcs.Workspace, args []string) int {
	pos, _ := parse(args)
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, "用法: kiln worktree new <issue>")
		return 2
	}
	issue := e.normalizedIssue(pos[0])
	if !model.ValidIssueID(issue) {
		fmt.Fprintf(e.errw, "非法 issue ID: %q（须为小写 kebab-case slug）\n", issue)
		return 2
	}
	path, err := ws.NewWorkspace(issue)
	if err != nil {
		fmt.Fprintf(e.errw, "建立 worktree 失败: %v\n", err)
		return 1
	}
	fmt.Fprintf(e.out, "worktree: %s\n", path)
	e.seedArea(ws, path)
	fmt.Fprintf(e.out, "下一步: cd 到该目录，再用 `kiln commit %s <action>` 记录动作\n", issue)
	// Project-neutral: an area is a bare checkout, whatever the project builds
	// with. Making it usable is the project's own business, so the hint names no
	// language, no path and no tool — and it uses the user-facing noun (worktree),
	// not the internal shorthand.
	fmt.Fprintln(e.out, "提示: worktree 是一次性检出——被忽略的构建产物与依赖不在其中，请自行 build / 装依赖")
	if cfgPath, created, err := config.Ensure(e.dir); err != nil {
		fmt.Fprintf(e.errw, "提示: 未能写入默认配置: %v\n", err)
	} else if created {
		fmt.Fprintf(e.out, "已写入默认配置: %s\n", cfgPath)
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
		fmt.Fprintf(e.errw, "提示: 未能播种本地文件: %v\n", err)
		return
	}
	if len(copied) > 0 {
		fmt.Fprintf(e.out, "已播种本地文件 %d 个（%s）: %s\n",
			len(copied), vcs.IncludeFile, strings.Join(copied, ", "))
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
	return "", fmt.Errorf("后端未列出 main")
}

// config implements `kiln config path`: it prints where the repository config
// lives, seeding it when missing so there is always something to edit.
func (e *env) config(args []string) int {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	if sub != "path" {
		fmt.Fprintln(e.errw, "用法: kiln config path")
		return 2
	}
	path, created, err := config.Ensure(e.dir)
	if err != nil {
		fmt.Fprintf(e.errw, "定位配置失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(e.out, path)
	if created {
		fmt.Fprintln(e.out, "（原本不存在，已写入一份带注释的默认配置）")
	}
	return 0
}

// workspaceList prints every working area, main first, aligned by path and role
// width, each row ending in what that area still holds. The state is asked of
// the backend for every area: the column exists to be trusted, so it is never
// left to a flag the caller might forget.
func (e *env) workspaceList(ws vcs.Workspace) int {
	areas, err := ws.ListWorkspaces()
	if err != nil {
		fmt.Fprintf(e.errw, "读取 worktree 失败: %v\n", err)
		return 1
	}
	states := make([]vcs.AreaState, len(areas))
	for i, a := range areas {
		state, err := ws.AreaState(a)
		if err != nil {
			fmt.Fprintf(e.errw, "读取 worktree 状态失败: %v\n", err)
			return 1
		}
		states[i] = state
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
		parts = append(parts, "目录已删")
	}
	if s.Dirty {
		parts = append(parts, "有改动")
	}
	if s.Unmerged {
		parts = append(parts, "未合并")
	}
	if len(parts) == 0 {
		return "干净"
	}
	return strings.Join(parts, "，")
}

// workspaceRemove deletes the dedicated area bound to an issue. The name is the
// binding, so the issue alone identifies the area; an area that still holds work
// is refused by the backend unless --force says to lose it.
func (e *env) workspaceRemove(ws vcs.Workspace, args []string) int {
	pos, f := parse(args)
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, "用法: kiln worktree remove <issue> [--force]")
		return 2
	}
	issue := pos[0]
	if err := ws.RemoveWorkspace(issue, f.has("force")); err != nil {
		fmt.Fprintf(e.errw, "删除 worktree 失败: %v\n", err)
		return 1
	}
	fmt.Fprintf(e.out, "已删除 worktree: %s\n", issue)
	return 0
}

// areaRole labels an area in `kiln workspace list`.
func areaRole(a vcs.WorkspaceInfo) string {
	if a.Main {
		return "main（只读/只合并）"
	}
	if a.Issue == "" {
		return "worktree"
	}
	return "worktree " + a.Issue
}

func (e *env) path(args []string) int {
	pos, f := parse(args)
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, "用法: kiln path <issue> [--stat|--shortstat]")
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	p, ok := repo.Issues[pos[0]]
	if !ok {
		fmt.Fprintf(e.errw, "未找到问题: %s\n", pos[0])
		return 1
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprintf(e.errw, "打开仓库失败: %v\n", err)
		return 1
	}
	mode := statMode(f)
	fmt.Fprintf(e.out, "%s  %s（当前 %s）\n", p.ID, e.progressWord(p), stateWord(p))
	for i, a := range p.Actions {
		line := fmt.Sprintf("  %d. %s %s %s", i+1, pad(a.Kind, 12), pad(shortID(a.ID), 8), a.Subject)
		if mode == statShort {
			stat, err := backend.ShowStat(a.ID, true)
			if err != nil {
				fmt.Fprintf(e.errw, "读取变更失败: %v\n", err)
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
				fmt.Fprintf(e.errw, "读取变更失败: %v\n", err)
				return 1
			}
			fmt.Fprint(e.out, indentStat(stat))
		}
	}
	if rel := relationSummary(p); rel != "" {
		fmt.Fprintf(e.out, "  关系: %s\n", rel)
	}
	return 0
}

// show implements `kiln show <issue> [n] [--stat]`: it expands a single action.
// The action is addressed by its 1-based ordinal in `path` (default: the last
// one), so the selector stays put even when commit hashes are rewritten by a
// rebase. Rendering is delegated to git.
func (e *env) show(args []string) int {
	pos, f := parse(args)
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, "用法: kiln show <issue> [n] [--stat]")
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	p, ok := repo.Issues[pos[0]]
	if !ok {
		fmt.Fprintf(e.errw, "未找到问题: %s\n", pos[0])
		return 1
	}
	if len(p.Actions) == 0 {
		fmt.Fprintf(e.errw, "问题没有动作: %s\n", pos[0])
		return 1
	}
	ordinal := len(p.Actions)
	if len(pos) >= 2 {
		n, err := strconv.Atoi(pos[1])
		if err != nil || n < 1 || n > len(p.Actions) {
			fmt.Fprintf(e.errw, "动作序号超出范围: %q（可选 1..%d）\n", pos[1], len(p.Actions))
			return 2
		}
		ordinal = n
	}
	a := p.Actions[ordinal-1]
	fmt.Fprintf(e.out, "%s · %d/%d %s · %s · %s\n\n",
		p.ID, ordinal, len(p.Actions), a.Kind, shortID(a.ID), a.Subject)

	backend, err := e.backend()
	if err != nil {
		fmt.Fprintf(e.errw, "打开仓库失败: %v\n", err)
		return 1
	}
	var body string
	if f.has("stat") {
		body, err = backend.ShowStat(a.ID, false)
	} else {
		body, err = backend.Show(a.ID)
	}
	if err != nil {
		fmt.Fprintf(e.errw, "读取变更失败: %v\n", err)
		return 1
	}
	fmt.Fprint(e.out, trimNewlines(body))
	fmt.Fprintln(e.out)
	return 0
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
	_, f := parse(args)
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
		fmt.Fprintf(e.out, "问题总数: 0（完成 0 / 进行中 0）｜%s\n", worktreeSegment(e.worktree, areasOpen(areas)))
		fmt.Fprintf(e.out, "kiln 动作数: %d\n", s.Actions)
		fmt.Fprintln(e.out, "提示: 用 `kiln do <issue> <action>` 生成规范提交信息")
		e.renderWorktreeLegend(areas)
		return 0
	}
	groups := engine.GroupByDomain(repo)
	fmt.Fprintf(e.out, "问题总数: %d（完成 %d / 进行中 %d）｜domain %d 个｜%s\n",
		s.Total, s.Complete, s.InProgress, engine.CountDomains(groups), worktreeSegment(e.worktree, areasOpen(areas)))
	fmt.Fprintf(e.out, "kiln 动作数: %d\n\n", s.Actions)
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
			fmt.Fprintln(e.out, r.head)
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
func (e *env) knownAreas() []vcs.WorkspaceInfo {
	backend, err := e.backend()
	if err != nil {
		return nil
	}
	ws, ok := backend.(vcs.Workspace)
	if !ok {
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
	return fmt.Sprintf("worktree %s（已开 %d 个）", worktreeWord(mode), areas)
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
func (e *env) renderWorktreeLegend(areas []vcs.WorkspaceInfo) {
	if len(areas) == 0 {
		return
	}
	line := AreaMark + " = 已开 worktree——各自状态见 kiln worktree list"
	if here, ok := vcs.AreaAt(areas, "."); ok && !here.Main {
		line += "｜当前：worktree " + here.Issue
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
		fmt.Fprintf(e.errw, "提示: issue ID 已归一化：%q → %q\n", raw, issue)
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
	fmt.Fprintf(e.errw, "拒绝: issue %q 已经有起点动作 %q（共 %d 个动作）——同名即同一条 issue，不需要再新建\n",
		issue, e.actions.Start(), len(p.Actions))
	fmt.Fprintf(e.errw, "提示: 要继续它，用 kiln commit %s <action>（可选: %s）\n", issue, strings.Join(rest, ", "))
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
	p, ok := engine.Build(engine.ActionsFromCommits(commits)).Issues[issue]
	return p, ok
}

func (e *env) isDone(p *model.Issue) bool {
	last, ok := p.Last()
	return ok && e.actions.IsTerminal(last.Kind)
}

func (e *env) check(args []string) int {
	_, f := parse(args)
	verbose := len(f["verbose"]) > 0
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	if f.has("json") {
		return e.checkJSON(repo)
	}
	if len(repo.Issues) == 0 {
		fmt.Fprintln(e.out, "没有 kiln 动作（检查提交信息是否同时带了 issue: 与 action:）")
		return 0
	}
	failed := false
	for _, d := range engine.Check(repo, e.actions) {
		fmt.Fprintf(e.out, "%s %s %s\n", d.Symbol(), pad(d.Issue, 30), d.Message)
		if d.Level == engine.LevelErr {
			failed = true
		}
		if verbose {
			if p, ok := repo.Issues[d.Issue]; ok {
				for _, a := range p.Actions {
					fmt.Fprintf(e.out, "      %s %s\n", pad(a.Kind, 12), shortID(a.ID))
				}
			}
		}
	}
	if failed {
		return 1
	}
	return 0
}

func (e *env) list(args []string) int {
	_, f := parse(args)
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	opts := engine.FilterOptions{
		Domain: f.get("domain"),
		Tag:    f.get("tag"),
		Action: f.get("action"),
	}
	ids := engine.Filter(repo, opts)
	if f.has("json") {
		doc := listDoc{
			Filter: filterQuery{Domain: opts.Domain, Tag: opts.Tag, Action: opts.Action},
			Count:  len(ids),
			Issues: make([]listIssue, 0, len(ids)),
		}
		for _, id := range ids {
			p := repo.Issues[id]
			doc.Issues = append(doc.Issues, listIssue{
				issueCore: issueCore{ID: p.ID, Domain: p.Domain, Action: p.State(), Done: e.isDone(p)},
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

func (e *env) graph(args []string) int {
	_, f := parse(args)
	format := f.get("format")
	if format == "" {
		format = "ascii"
	}
	if format != "ascii" {
		fmt.Fprintf(e.errw, "MVP 仅支持 --format=ascii（当前: %s）\n", format)
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	if len(repo.Issues) == 0 {
		fmt.Fprintln(e.out, "（空）")
		return 0
	}
	fmt.Fprintln(e.out, engine.Graph(repo))
	return 0
}

// loadRepo reads history and aggregates it; returns (nil, exitCode) on failure.
func (e *env) loadRepo() (*model.Repo, int) {
	backend, err := e.backend()
	if err != nil {
		fmt.Fprintf(e.errw, "打开仓库失败: %v\n", err)
		return nil, 1
	}
	commits, err := backend.Log()
	if err != nil {
		fmt.Fprintf(e.errw, "读取历史失败: %v\n", err)
		return nil, 1
	}
	return engine.Build(engine.ActionsFromCommits(commits)), 0
}

func modeName(commit bool) string {
	if commit {
		return "commit"
	}
	return "do"
}

func stateWord(p *model.Issue) string {
	if s := p.State(); s != "" {
		return s
	}
	return "未知"
}

func (e *env) progressWord(p *model.Issue) string {
	if last, ok := p.Last(); ok && e.actions.IsTerminal(last.Kind) {
		return "已完成"
	}
	return "进行中"
}

func relationSummary(p *model.Issue) string {
	var parts []string
	add := func(label string, vals []string) {
		if len(vals) > 0 {
			parts = append(parts, label+" "+strings.Join(vals, ", "))
		}
	}
	add("derived-from", p.DerivedFrom)
	add("spawns", p.Spawns)
	add("relates-to", p.RelatesTo)
	add("blocks", p.Blocks)
	add("supersedes", p.Supersedes)
	return strings.Join(parts, "; ")
}

func tagsSuffix(p *model.Issue) string {
	var parts []string
	if p.Domain != "" {
		parts = append(parts, "domain="+p.Domain)
	}
	if len(p.Tags) > 0 {
		parts = append(parts, "tags="+strings.Join(p.Tags, ","))
	}
	return strings.Join(parts, " ")
}

func shortID(id string) string {
	if len(id) > 7 {
		return id[:7]
	}
	return id
}

// ---------------------------------------------------------------------------
// Machine-readable output (`--json`)
//
// The text views are the default; `--json` is a side road for agents and
// scripts, whose schema lives in docs/machine-readable-output.md. Four rules
// hold for all of it. (1) The document is always one object, never a bare
// array, so a field can be added later without changing the type a caller
// decodes. (2) Keys reuse the words kiln already speaks — trailer names where
// the thing is the trailer's, view words (`parent`, `has-worktree`) where the
// concept belongs to the view. (3) Nothing addressed to a human gets in: no
// hints, no legends, no prose — a caller that wants those reads the text view.
// (4) An absent key means "none", never `null`; booleans are the exception, since
// `false` is a finding rather than an absence and is always written.
// ---------------------------------------------------------------------------

// writeJSON emits one document on stdout, which is the whole machine-readable
// contract: stdout carries the document and nothing else, every failure stays on
// stderr, and the exit code says what the text view says.
func (e *env) writeJSON(v any) int {
	enc := json.NewEncoder(e.out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(e.errw, "输出 JSON 失败: %v\n", err)
		return 1
	}
	return 0
}

// issueCore is the row `status` and `list` share, so one decoder covers the same
// four fields in both and each command only adds what belongs to it.
type issueCore struct {
	ID     string `json:"id"`
	Domain string `json:"domain,omitempty"`
	Action string `json:"action"`
	Done   bool   `json:"done"`
}

// statusDoc is the `kiln status --json` document: the three things the text view
// states — repository counters, the isolation state, and the issues.
type statusDoc struct {
	Summary  statusSummary `json:"summary"`
	Worktree worktreeState `json:"worktree"`
	Issues   []statusIssue `json:"issues"`
}

type statusSummary struct {
	Total      int `json:"total"`
	Complete   int `json:"complete"`
	InProgress int `json:"in-progress"`
	Actions    int `json:"actions"`
	Domains    int `json:"domains"`
}

type worktreeState struct {
	Mode    string `json:"mode"`
	Open    int    `json:"open"`
	Current string `json:"current,omitempty"`
}

// statusIssue is one flat row of the domain view. It carries no action list —
// that is `path`'s job — which keeps the document O(issues), not O(commits).
type statusIssue struct {
	issueCore
	HasWorktree bool `json:"has-worktree"`
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
}

type listIssue struct {
	issueCore
	Tags []string `json:"tags,omitempty"`
}

// checkDoc is the `kiln check --json` document. OK mirrors the exit code — false
// exactly when an error was found — so a caller that ignores exit codes still
// gets the answer CI cares about.
type checkDoc struct {
	OK      bool          `json:"ok"`
	Summary checkSummary  `json:"summary"`
	Results []checkResult `json:"results"`
}

// checkSummary counts findings, not issues: one issue can fail twice.
type checkSummary struct {
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Err  int `json:"error"`
}

// checkResult is one finding, named by rule code rather than described in
// prose. The sentence stays where it is read by a human.
type checkResult struct {
	Issue   string   `json:"issue"`
	Level   string   `json:"level"`
	Rule    string   `json:"rule"`
	Action  string   `json:"action,omitempty"`
	Ref     string   `json:"ref,omitempty"`
	Actions []string `json:"actions"`
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
			Total:      s.Total,
			Complete:   s.Complete,
			InProgress: s.InProgress,
			Actions:    s.Actions,
			Domains:    engine.CountDomains(groups),
		},
		Worktree: e.worktreeState(areas),
		Issues:   make([]statusIssue, 0, s.Total),
	}
	for _, r := range e.statusRows(groups, areaIssues(areas)) {
		doc.Issues = append(doc.Issues, statusIssue{
			issueCore: issueCore{
				ID:     r.issue.ID,
				Domain: r.issue.Domain,
				Action: r.issue.State(),
				Done:   e.isDone(r.issue),
			},
			HasWorktree: r.area,
		})
	}
	return e.writeJSON(doc)
}

// worktreeState states the repository-level isolation mode and how many areas are
// open, plus the area the caller stands in when there is one.
func (e *env) worktreeState(areas []vcs.WorkspaceInfo) worktreeState {
	ws := worktreeState{Mode: worktreeWord(e.worktree), Open: areasOpen(areas)}
	if here, ok := vcs.AreaAt(areas, "."); ok && !here.Main {
		ws.Current = here.Issue
	}
	return ws
}

// checkJSON renders every finding with its rule code. The verbose flag changes
// nothing here: the action list is always reported, so the document keeps one
// shape and a caller needs one decoder whatever flags were passed.
//
// The document is written first and the exit code decided after: leaving with a
// non-zero code must not cost the caller the report they ran the command for.
func (e *env) checkJSON(repo *model.Repo) int {
	diags := engine.Check(repo, e.actions)
	doc := checkDoc{OK: true, Results: make([]checkResult, 0, len(diags))}
	for _, d := range diags {
		switch d.Level {
		case engine.LevelWarn:
			doc.Summary.Warn++
		case engine.LevelErr:
			doc.Summary.Err++
			doc.OK = false
		default:
			doc.Summary.OK++
		}
		doc.Results = append(doc.Results, checkResult{
			Issue:   d.Issue,
			Level:   levelWord(d.Level),
			Rule:    d.Rule,
			Action:  d.Action,
			Ref:     d.Ref,
			Actions: kindsOf(repo.Issues[d.Issue]),
		})
	}
	if code := e.writeJSON(doc); code != 0 {
		return code
	}
	if !doc.OK {
		return 1
	}
	return 0
}

// levelWord renders a diagnostic level as the word the machine view reports. The
// internal ordering (LevelOK < LevelWarn < LevelErr) is an implementation
// detail and must not leak into the contract.
func levelWord(l engine.Level) string {
	switch l {
	case engine.LevelWarn:
		return "warn"
	case engine.LevelErr:
		return "error"
	default:
		return "ok"
	}
}

// kindsOf lists an issue's action kinds in path order.
func kindsOf(p *model.Issue) []string {
	out := make([]string, 0, len(p.Actions))
	for _, a := range p.Actions {
		out = append(out, a.Kind)
	}
	return out
}
