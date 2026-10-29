package cli

import (
	"strings"

	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

const usageHeader = `kiln — git-trailer 驱动的问题追踪与路径重建

用法:
`

const usagePrologue = `
action 词表默认: ask | hypothesize | collect | test | conclude | apply（生效词表见 kiln action list）
worktree 默认: off（允许在 main 提交）| on（动作只在 worktree 里提交——开工前先 kiln worktree new <issue>）
提交范围: 默认提交当前工作区的全部改动（快照）；--only <paths> 只提交列出的路径（两后端同义；空动作允许）
（配置：仓库根的 .kilnconfig.jsonc，JSONC；用 kiln config path 打印路径）
`

// usageOtherFlags are the per-command flags that are not the do/commit surface.
const usageOtherFlags = `
path 的 flags:  --stat（每动作附文件统计）  --shortstat（每动作一行总计）  --json（机器可读输出）
path 额外 flag:  --all（一次读全仓；只与 --json 同用，也可直接给多条问题）
show 的 flags:  --stat（只给该动作的文件统计）  --json（机器可读输出）
worktree list 的 flags:  --json（机器可读输出）
status / list / check 的 flags:  --json（机器可读输出：stdout 只放一个 JSON 文档）
list 的 --status:  open（未收尾）| done（末条动作是终点动作）
list 的 --tag:  单个标签，不切逗号（多标签过滤暂不支持）
list 的 --search:  空白分词、词间 AND、字面量、大小写不敏感（搜 id / 当前标题 / 动作 subject）
commit 额外 flag:  --force（worktree=on 时在 main 提交**必填**，作一次性豁免）  --only <paths>（只提交列出的路径）
worktree remove 额外 flag:  --force（连同未合并的分支 / 尚有改动的 worktree 一起删）
--lang <zh|en>:  全局语言开关（默认按 KILN_LANG / LC_ALL / LC_MESSAGES / LANG 探测，未设则中文）
`

// usageCommands is the command surface rendered by usageText.
var usageCommands = [][2]string{
	{"kiln do <issue> <action> --title <text> [flags]", "生成规范提交信息（不写文件）"},
	{"kiln commit <issue> <action> --title <text> [--only <paths>] [flags]", "用规范信息直接提交（--only 只提交列出的路径）"},
	{"kiln path <issue> [<issue>…] | --all", "重建问题的研究路径（--all 一次读全仓；可加 --stat / --shortstat / --json）"},
	{"kiln show <issue> [n|action]", "展开某个动作的完整变更（可加 --stat / --json）"},
	{"kiln status", "全库概览"},
	{"kiln check [--verbose]", "语义完整性校验"},
	{"kiln list [--domain=] [--tag=] [--action=] [--status=open|done] [--search=]", "按条件过滤问题"},
	{"kiln action list [--json]", "列出可用的 action 类型（词表）"},
	{"kiln action log [--jsonl] [--limit N]", "全局动作流（一条动作一行；--jsonl 给 agent 读；--limit 只看最近 N 条）"},
	{"kiln graph [--format=ascii|mermaid|json] [--mermaid] [--json] [--issue <id>] [--depth N]", "问题关系图（--mermaid 出流程图文本；--json 给 agent 读，--issue/--depth 收窄邻域）"},
	{"kiln worktree new <issue>", "建 worktree（main 只读，动作只在 worktree 提交）"},
	{"kiln worktree list", "列出全部 worktree（含 main）及各自状态（可加 --json）"},
	{"kiln cache gc", "按可达集清理提交记录缓存（读命令自己写、自己合并；只有强制回收需要这条）"},
	{"kiln cache clear", "清空提交记录缓存（缓存出错、升级 kiln 之后用；下一次读会用当前版本重建）"},
	{"kiln worktree remove <issue>", "删除 worktree（尚有工作的默认拒绝，--force 强删）"},
	{"kiln config path", "打印配置路径（缺失时写入一份带注释的默认配置）"},
	{"kiln --version", "打印版本、可执行文件路径与构建时间（-V / version 同义）"},
}

// flagHelp is one row of the do/commit flag help: the flag, its value shape
// ("<issue>[,<issue>]" for a repeatable comma-separated list), and a gloss.
type flagHelp struct{ flag, shape, gloss string }

// doCommitFlagGroups builds the do/commit flag help from a fixed metadata group
// and a relation group generated from the vocabulary.
func doCommitFlagGroups(rel *model.RelationSet) (meta, relations []flagHelp) {
	meta = []flagHelp{
		{"--title", i18n.T("<文本>"), i18n.T("动作标题（必填）")},
		{"--refs", i18n.T("<条目>[,<条目>]"), i18n.T("引用")},
		{"--target", i18n.T("<文件>[,<文件>]"), i18n.T("涉及的文件（必须在本次改动里）")},
		{"--scope", i18n.T("<文本>"), i18n.T("范围")},
		{"--domain", i18n.T("<名字>"), i18n.T("分类")},
		{"--tag", i18n.T("<标签>[,<标签>]"), i18n.T("标签")},
	}
	for _, name := range rel.Names() {
		relations = append(relations, flagHelp{"--" + name, "<issue>[,<issue>]", i18n.T(rel.Gloss(name))})
	}
	relations = append(relations, flagHelp{"--unrelate", "<issue>[,<issue>]", i18n.T("撤销与该 issue 的关系")})
	return meta, relations
}

// usageText renders the help text, aligning descriptions by terminal display
// width so full-width CJK text does not skew the columns. The do/commit flag
// section is generated from the relation vocabulary, so it never drifts from
// what the parser actually accepts.
func usageText(rel *model.RelationSet) string {
	width := 0
	for _, c := range usageCommands {
		if w := displayWidth(c[0]); w > width {
			width = w
		}
	}
	var b strings.Builder
	b.WriteString(i18n.T(usageHeader))
	for _, c := range usageCommands {
		b.WriteString("  " + pad(c[0], width) + "  " + i18n.T(c[1]) + "\n")
	}
	b.WriteString(i18n.T(usagePrologue))
	b.WriteString(doCommitFlagText(rel))
	b.WriteString(i18n.T(usageOtherFlags))
	return b.String()
}

// doCommitFlagText renders the do/commit flag groups, shared by the full help
// and by `kiln do -h` / `kiln commit -h`.
func doCommitFlagText(rel *model.RelationSet) string {
	meta, relations := doCommitFlagGroups(rel)
	flagWidth := 0
	for _, rows := range [][]flagHelp{meta, relations} {
		for _, r := range rows {
			if w := displayWidth(r.flag + " " + r.shape); w > flagWidth {
				flagWidth = w
			}
		}
	}
	var b strings.Builder
	writeGroup := func(title string, rows []flagHelp) {
		b.WriteString(title + "\n")
		for _, r := range rows {
			line := "    " + pad(r.flag+" "+r.shape, flagWidth)
			if r.gloss != "" {
				line += "  " + r.gloss
			}
			b.WriteString(strings.TrimRight(line, " ") + "\n")
		}
	}
	b.WriteString(i18n.T("\ndo/commit 的 flags:  （位置参数 <issue> <action> 与 --title 必填；其余 flag 选填）\n"))
	writeGroup(i18n.T("  元数据:"), meta)
	writeGroup(i18n.T("  关系:  （均可重复、逗号分隔）"), relations)
	b.WriteString(i18n.T("  说明: <> 是值占位；[,<…>] 表示可重复、逗号分隔；关系名亦可写别名 derived-from / spawns。\n  值以 -- 开头时写 --flag=值（空格形式会报缺值）。\n  终点动作（默认 conclude / apply）之后仍可继续接动作——问题不会被关闭。\n"))
	return b.String()
}

// commandUsageText renders `<cmd> -h`: the command's own usage rows plus the
// flag lines that name it, instead of the whole surface. ok is false for a name
// the command table does not know.
func commandUsageText(rel *model.RelationSet, cmd string) (string, bool) {
	var rows [][2]string
	for _, c := range usageCommands {
		rest, found := strings.CutPrefix(c[0], "kiln "+cmd)
		if found && (rest == "" || strings.HasPrefix(rest, " ")) {
			rows = append(rows, c)
		}
	}
	if len(rows) == 0 {
		return "", false
	}
	width := 0
	for _, c := range rows {
		if w := displayWidth(c[0]); w > width {
			width = w
		}
	}
	var b strings.Builder
	b.WriteString(i18n.T("用法:\n"))
	for _, c := range rows {
		b.WriteString("  " + pad(c[0], width) + "  " + i18n.T(c[1]) + "\n")
	}
	if cmd == "do" || cmd == "commit" {
		b.WriteString(doCommitFlagText(rel))
	}
	b.WriteString(flagLinesFor(cmd))
	return b.String(), true
}

// flagLinesFor keeps the usageOtherFlags lines that belong to cmd. The block is
// one translated string; each line starts with the command (or a
// "status / list / check" group) it documents, and --lang is global.
func flagLinesFor(cmd string) string {
	var b strings.Builder
	for _, line := range strings.Split(i18n.T(usageOtherFlags), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if lineNamesCommand(trimmed, cmd) {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// lineNamesCommand reports whether a usageOtherFlags line documents cmd. The
// English block prefixes some lines with "extra " ("extra list flag: …"), and
// the group line names several commands ("status / list / check … flags:").
func lineNamesCommand(line, cmd string) bool {
	trimmed := strings.TrimPrefix(strings.TrimSpace(line), "extra ")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "--lang" {
		return true
	}
	if i := strings.Index(trimmed, " flags"); i > 0 {
		for _, name := range strings.Split(trimmed[:i], " / ") {
			name = strings.TrimSuffix(strings.TrimSpace(name), " 的")
			if name == cmd {
				return true
			}
		}
		return false
	}
	return fields[0] == cmd
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
