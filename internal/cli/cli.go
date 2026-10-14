// Package cli wires the command surface: argument parsing, git access and text
// rendering. All output goes through the writers passed to Run, so the package
// stays testable.
package cli

import (
	"fmt"
	"io"
	"strings"

	"kiln/internal/config"
	"kiln/internal/engine"
	"kiln/internal/git"
	"kiln/internal/model"
	"kiln/internal/trailer"
)

const usageHeader = `kiln — git-trailer 驱动的问题追踪与路径重建

用法:
`

const usageFooter = `
action 词表默认: ask | hypothesize | collect | test | conclude | apply
（可用当前目录的 .kiln.json 覆盖，见 docs/mvp-plan.md §2.3）

do/commit 的 flags:
  --title  --object  --refs  --target  --scope  --domain  --tag
  --derived-from  --spawns  --relates-to  --blocks  --supersedes
`

// usageCommands is the command surface rendered by usageText.
var usageCommands = [][2]string{
	{"kiln do <issue> <action> [object] [flags]", "生成规范提交信息（不写文件）"},
	{"kiln commit <issue> <action> [object] [flags]", "用规范信息直接提交"},
	{"kiln path <issue>", "重建问题的研究路径"},
	{"kiln status", "全库概览"},
	{"kiln check [--verbose]", "语义完整性校验"},
	{"kiln list [--domain=] [--tag=] [--action=]", "按条件过滤问题"},
	{"kiln graph [--format=ascii]", "问题关系图"},
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

// env carries the resolved action vocabulary and the output streams.
type env struct {
	actions *model.ActionSet
	out     io.Writer
	errw    io.Writer
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
	case "do", "commit", "path", "status", "check", "list", "graph":
		// handled below, once the vocabulary is loaded
	default:
		fmt.Fprintf(stderr, "未知命令: %s\n\n%s", cmd, usageText())
		return 2
	}

	actions, err := config.Load(".")
	if err != nil {
		fmt.Fprintf(stderr, "配置错误: %v\n", err)
		return 1
	}
	e := &env{actions: actions, out: stdout, errw: stderr}
	switch cmd {
	case "do":
		return e.write(rest, false)
	case "commit":
		return e.write(rest, true)
	case "path":
		return e.path(rest)
	case "status":
		return e.status(rest)
	case "check":
		return e.check(rest)
	case "list":
		return e.list(rest)
	case "graph":
		return e.graph(rest)
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

func (f flags) list(key string) []string { return f[key] }

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
		fmt.Fprintf(e.errw, "用法: kiln %s <issue> <action> [object] [flags]\n", modeName(commit))
		return 2
	}
	issue := pos[0]
	action := strings.ToLower(strings.TrimSpace(pos[1]))
	object := f.get("object")
	if object == "" && len(pos) > 2 {
		object = pos[2]
	}
	if !e.actions.IsValid(action) {
		fmt.Fprintf(e.errw, "非法 action: %q（可选: %s）\n", action, strings.Join(e.actions.Names(), ", "))
		return 2
	}
	if !model.ValidIssueID(issue) {
		fmt.Fprintf(e.errw, "警告: issue ID %q 不符合 <namespace>/<slug> 约定\n", issue)
	}

	msg := trailer.Format(trailer.Fields{
		Issue:       issue,
		Action:      action,
		Object:      object,
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
		return 0
	}
	if err := (git.ExecRunner{Dir: "."}).Commit(msg); err != nil {
		fmt.Fprintf(e.errw, "提交失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(e.out, msg)
	return 0
}

func (e *env) path(args []string) int {
	pos, _ := parse(args)
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, "用法: kiln path <issue>")
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
	fmt.Fprintf(e.out, "%s  %s（当前 %s）\n", p.ID, e.progressWord(p), stateWord(p))
	for i, a := range p.Actions {
		fmt.Fprintf(e.out, "  %d. %s %s %s %s\n", i+1, pad(a.Kind, 12), pad(a.Object, 6), pad(shortHash(a.Hash), 8), a.Subject)
	}
	if rel := relationSummary(p); rel != "" {
		fmt.Fprintf(e.out, "  关系: %s\n", rel)
	}
	return 0
}

func (e *env) status(args []string) int {
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	s := engine.Summarize(repo, e.actions)
	fmt.Fprintf(e.out, "问题总数: %d（完成 %d / 进行中 %d）\n", s.Total, s.Complete, s.InProgress)
	fmt.Fprintf(e.out, "kiln 动作数: %d\n", s.Actions)
	if s.Total == 0 {
		fmt.Fprintln(e.out, "提示: 用 `kiln do <issue> <action> [object]` 生成规范提交信息")
		return 0
	}
	for _, id := range engine.IDs(repo) {
		p := repo.Issues[id]
		mark := " "
		if last, ok := p.Last(); ok && e.actions.IsTerminal(last.Kind) {
			mark = "✓"
		}
		fmt.Fprintf(e.out, "  %s %s %s\n", mark, pad(p.ID, 30), stateWord(p))
	}
	return 0
}

func (e *env) check(args []string) int {
	_, f := parse(args)
	verbose := len(f["verbose"]) > 0
	repo, code := e.loadRepo()
	if repo == nil {
		return code
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
					fmt.Fprintf(e.out, "      %s %s %s\n", pad(a.Kind, 12), pad(a.Object, 6), shortHash(a.Hash))
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
	ids := engine.Filter(repo, engine.FilterOptions{
		Domain: f.get("domain"),
		Tag:    f.get("tag"),
		Action: f.get("action"),
	})
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
	commits, err := (git.ExecRunner{Dir: "."}).Log()
	if err != nil {
		fmt.Fprintf(e.errw, "读取 git 历史失败: %v\n", err)
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

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
