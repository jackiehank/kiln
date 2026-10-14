// Package cli wires the command surface: argument parsing, git access and text
// rendering. All output goes through the writers passed to Run, so the package
// stays testable.
package cli

import (
	"fmt"
	"io"
	"strings"

	"kiln/internal/engine"
	"kiln/internal/git"
	"kiln/internal/model"
	"kiln/internal/trailer"
)

const usage = `kiln — git-trailer 驱动的问题追踪与路径重建

用法:
  kiln do <problem> <action> [object] [flags]    生成规范提交信息（不写文件）
  kiln commit <problem> <action> [object] [flags] 用规范信息直接提交
  kiln path <problem>                            重建问题的研究路径
  kiln status                                    全库概览
  kiln check [--verbose]                         语义完整性校验
  kiln list [--domain=] [--tag=] [--action=]     按条件过滤问题
  kiln graph [--format=ascii]                    问题关系图

action: ask | hypothesize | collect | test | conclude | apply

do/commit 的 flags:
  --title  --object  --refs  --target  --scope  --domain  --tag
  --derived-from  --spawns  --relates-to  --blocks  --supersedes
`

// Run executes one command and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "do":
		return runWrite(rest, stdout, stderr, false)
	case "commit":
		return runWrite(rest, stdout, stderr, true)
	case "path":
		return runPath(rest, stdout, stderr)
	case "status":
		return runStatus(rest, stdout, stderr)
	case "check":
		return runCheck(rest, stdout, stderr)
	case "list":
		return runList(rest, stdout, stderr)
	case "graph":
		return runGraph(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "未知命令: %s\n\n%s", cmd, usage)
		return 2
	}
}

// flags holds repeatable --key value pairs parsed from a command's arguments.
type flags map[string][]string

func (f flags) get(key string) string {
	if v := f[key]; len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
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

// runWrite implements both `do` (print only) and `commit` (print then commit).
func runWrite(args []string, stdout, stderr io.Writer, commit bool) int {
	pos, f := parse(args)
	if len(pos) < 2 {
		fmt.Fprintf(stderr, "用法: kiln %s <problem> <action> [object] [flags]\n", modeName(commit))
		return 2
	}
	problem, action := pos[0], pos[1]
	object := f.get("object")
	if object == "" && len(pos) > 2 {
		object = pos[2]
	}
	if !model.IsValidAction(action) {
		fmt.Fprintf(stderr, "非法 action: %q（可选: %s）\n", action, strings.Join(model.ValidActions, ", "))
		return 2
	}
	if !model.ValidProblemID(problem) {
		fmt.Fprintf(stderr, "警告: problem ID %q 不符合 <namespace>/<slug> 约定\n", problem)
	}

	msg := trailer.Format(trailer.Fields{
		Problem:     problem,
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
		fmt.Fprintln(stdout, msg)
		return 0
	}
	if err := (git.ExecRunner{Dir: "."}).Commit(msg); err != nil {
		fmt.Fprintf(stderr, "提交失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, msg)
	return 0
}

func (f flags) list(key string) []string { return f[key] }

func runPath(args []string, stdout, stderr io.Writer) int {
	pos, _ := parse(args)
	if len(pos) < 1 {
		fmt.Fprintln(stderr, "用法: kiln path <problem>")
		return 2
	}
	repo, code := loadRepo(stderr)
	if repo == nil {
		return code
	}
	p, ok := repo.Problems[pos[0]]
	if !ok {
		fmt.Fprintf(stderr, "未找到问题: %s\n", pos[0])
		return 1
	}
	fmt.Fprintf(stdout, "%s  %s（当前 %s）\n", p.ID, progressWord(p), stateWord(p))
	for i, a := range p.Actions {
		fmt.Fprintf(stdout, "  %d. %-12s %-6s %-8s %s\n", i+1, a.Kind, a.Object, shortHash(a.Hash), a.Subject)
	}
	if rel := relationSummary(p); rel != "" {
		fmt.Fprintf(stdout, "  关系: %s\n", rel)
	}
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	repo, code := loadRepo(stderr)
	if repo == nil {
		return code
	}
	s := engine.Summarize(repo)
	fmt.Fprintf(stdout, "问题总数: %d（完成 %d / 进行中 %d）\n", s.Total, s.Complete, s.InProgress)
	fmt.Fprintf(stdout, "kiln 动作数: %d\n", s.Actions)
	if s.Total == 0 {
		fmt.Fprintln(stdout, "提示: 用 `kiln do <problem> <action> [object]` 生成规范提交信息")
		return 0
	}
	for _, id := range engine.IDs(repo) {
		p := repo.Problems[id]
		mark := " "
		if last, ok := p.Last(); ok && model.IsTerminal(last.Kind) {
			mark = "✓"
		}
		fmt.Fprintf(stdout, "  %s %-30s %s\n", mark, p.ID, stateWord(p))
	}
	return 0
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	_, f := parse(args)
	verbose := len(f["verbose"]) > 0
	repo, code := loadRepo(stderr)
	if repo == nil {
		return code
	}
	diags := engine.Check(repo)
	if len(repo.Problems) == 0 {
		fmt.Fprintln(stdout, "没有 kiln 动作（检查提交信息是否同时带了 problem: 与 action:）")
		return 0
	}
	failed := false
	for _, d := range diags {
		fmt.Fprintf(stdout, "%s %-30s %s\n", d.Symbol(), d.Problem, d.Message)
		if d.Level == engine.LevelErr {
			failed = true
		}
		if verbose {
			if p, ok := repo.Problems[d.Problem]; ok {
				for _, a := range p.Actions {
					fmt.Fprintf(stdout, "      %-12s %-6s %s\n", a.Kind, a.Object, shortHash(a.Hash))
				}
			}
		}
	}
	if failed {
		return 1
	}
	return 0
}

func runList(args []string, stdout, stderr io.Writer) int {
	_, f := parse(args)
	repo, code := loadRepo(stderr)
	if repo == nil {
		return code
	}
	ids := engine.Filter(repo, engine.FilterOptions{
		Domain: f.get("domain"),
		Tag:    f.get("tag"),
		Action: f.get("action"),
	})
	for _, id := range ids {
		p := repo.Problems[id]
		fmt.Fprintf(stdout, "%-30s %-12s %s\n", p.ID, stateWord(p), tagsSuffix(p))
	}
	return 0
}

func runGraph(args []string, stdout, stderr io.Writer) int {
	_, f := parse(args)
	format := f.get("format")
	if format == "" {
		format = "ascii"
	}
	if format != "ascii" {
		fmt.Fprintf(stderr, "MVP 仅支持 --format=ascii（当前: %s）\n", format)
		return 2
	}
	repo, code := loadRepo(stderr)
	if repo == nil {
		return code
	}
	if len(repo.Problems) == 0 {
		fmt.Fprintln(stdout, "（空）")
		return 0
	}
	fmt.Fprintln(stdout, engine.Graph(repo))
	return 0
}

// loadRepo reads history and aggregates it; returns (nil, exitCode) on failure.
func loadRepo(stderr io.Writer) (*model.Repo, int) {
	commits, err := (git.ExecRunner{Dir: "."}).Log()
	if err != nil {
		fmt.Fprintf(stderr, "读取 git 历史失败: %v\n", err)
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

func stateWord(p *model.Problem) string {
	if s := p.State(); s != "" {
		return s
	}
	return "未知"
}

func progressWord(p *model.Problem) string {
	if last, ok := p.Last(); ok && model.IsTerminal(last.Kind) {
		return "已完成"
	}
	return "进行中"
}

func relationSummary(p *model.Problem) string {
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

func tagsSuffix(p *model.Problem) string {
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
