package i18n

// enCatalog maps every Chinese source string to its English rendering. Keys are
// the messages themselves, not ids: a string added after this catalog still
// prints its Chinese source instead of a meaningless symbol, and a test pins the
// English help/output to "no CJK" so a missing entry cannot slip through.
//
// Formatting verbs are part of the contract: a value must use the same verbs in
// the same order as its key, because T hands the arguments straight to Sprintf
// (or Errorf, for E).
var enCatalog = map[string]string{
	"<文本>":        "<text>",
	"动作标题":        "action title",
	"<条目>[,<条目>]": "<entry>[,<entry>]",
	"引用":          "references",
	"<文件>":        "<file>",
	"涉及的文件（单个）":   "file touched (single)",
	"<文件>[,<文件>]": "<file>[,<file>]",
	"涉及的文件（必须在本次改动里）": "files touched (must be part of this change)",
	"范围":             "scope",
	"<名字>":           "<name>",
	"分类":             "domain",
	"<标签>[,<标签>]":    "<tag>[,<tag>]",
	"标签":             "tags",
	"撤销与该 issue 的关系": "drop the relation to this issue",

	"\ndo/commit 的 flags:  （位置参数 <issue> <action> 必填；以下 flag 均选填）\n": "\ndo/commit flags:  (positional <issue> <action> required; every flag below is optional)\n",
	"  元数据:": "  metadata:",
	"  关系:  （均可重复、逗号分隔）": "  relations:  (repeatable, comma-separated)",
	"  说明: <> 是值占位；[,<…>] 表示可重复、逗号分隔；关系名亦可写别名 derived-from / spawns。\n  终点动作（默认 conclude / apply）之后仍可继续接动作——问题不会被关闭。\n": "  notes: <> is a value placeholder; [,<…>] means repeatable, comma-separated; relation names also accept the aliases derived-from / spawns.\n  a terminal action (conclude / apply by default) does not close the issue: more actions may follow.\n",
	"动作标题（必填）": "action title (required)",
	"\ndo/commit 的 flags:  （位置参数 <issue> <action> 与 --title 必填；其余 flag 选填）\n": "\ndo/commit flags:  (positional <issue> <action> and --title are required; every other flag is optional)\n",
	"  说明: <> 是值占位；[,<…>] 表示可重复、逗号分隔；关系名亦可写别名 derived-from / spawns。\n  值以 -- 开头时写 --flag=值（空格形式会报缺值）。\n  终点动作（默认 conclude / apply）之后仍可继续接动作——问题不会被关闭。\n": "  notes: <> is a value placeholder; [,<…>] means repeatable, comma-separated; relation names also accept the aliases derived-from / spawns.\n  a value that starts with -- is written --flag=value (the space form is refused as a missing value).\n  a terminal action (conclude / apply by default) does not close the issue: more actions may follow.\n",

	"警告: 未知 flag --%s（可用 flag 见 kiln -h）\n": "warning: unknown flag --%s (see kiln -h for the flags it takes)\n",
	"未知后端: %s":                               "unknown backend: %s",
	"--lang 需要一个值（zh 或 en）\n":                "--lang needs a value (zh or en)\n",
	"--lang 只接受 zh 或 en（当前: %s）\n":           "--lang only accepts zh or en (got: %s)\n",
	"未知命令: %s（已改名为 `%s`）\n\n%s":              "unknown command: %s (renamed to `%s`)\n\n%s",
	"未知命令: %s\n\n%s":                         "unknown command: %s\n\n%s",
	"配置错误: %v\n":                             "config error: %v\n",
	"可执行文件: %s\n":                            "executable: %s\n",
	"构建时间: %s\n":                             "built: %s\n",
	"用法: kiln %s <issue> <action> [flags]\n": "usage: kiln %s <issue> <action> [flags]\n",
	"用法: kiln %s <issue> <action> [flags]：object 已退役，第三个参数已不再支持\n":              "usage: kiln %s <issue> <action> [flags]: object was retired; the third argument is no longer accepted\n",
	"用法: kiln %s <issue> <action> --title <文本> [flags]\n":                       "usage: kiln %s <issue> <action> --title <text> [flags]\n",
	"用法: kiln %s <issue> <action> --title <文本> [flags]：object 已退役，第三个参数已不再支持\n": "usage: kiln %s <issue> <action> --title <text> [flags]: object was retired; the third argument is no longer accepted\n",
	"非法 action: %q（可选: %s）\n":                                                   "invalid action: %q (allowed: %s)\n",
	"警告: issue ID %q 不符合单段 kebab-case slug 约定（小写字母 / 数字 / `-`，长度 ≤ 50）\n":       "warning: issue ID %q breaks the single-segment kebab-case slug rule (lowercase letters / digits / `-`, length ≤ 50)\n",
	"do 不接受 --only：它只打印 trailer，不提交；只提交哪些文件是 commit 的事。":                        "do does not take --only: it only prints the trailer and commits nothing; which files to commit is commit's business.",
	"打开仓库失败: %v\n":            "cannot open the repository: %v\n",
	"检查路径失败: %v\n":            "cannot inspect the path: %v\n",
	"以下路径没有改动可记录: %s\n":       "these paths have no change to record: %s\n",
	"以下 target 没有改动可记录: %s\n": "these targets have no change to record: %s\n",
	"将提交 %d 个文件: %s\n":        "about to commit %d file(s): %s\n",
	"提示: 当前工作区没有检测到改动，这次动作会记成空提交。": "hint: the working area shows no change; this action will be recorded as an empty commit.",
	"提交失败: %v\n":             "commit failed: %v\n",
	"\n已提交 · %s · %s\n":      "\ncommitted · %s · %s\n",
	"\n已提交 %s · %s · %s\n":   "\ncommitted %s · %s · %s\n",
	" 等 %d 个":                " and %d more",
	"无法判定 worktree 类型: %v\n": "cannot tell which kind of worktree this is: %v\n",
	"拒绝提交：当前是 main（只读/只合并），动作只在 worktree 里提交。":       "refusing to commit: this is main (read-only / merge-only); actions are committed in a worktree.",
	"你已有未提交改动——新区是干净检出，直接建区会把改动留在 main：":             "you already have uncommitted changes — a new area is a clean checkout, so creating one now leaves those changes on main:",
	"（--force 只放行护栏、不把改动搬进区——只有你确实要把它留在 main 时才用。）":  "(--force only lifts the guard, it does not move the changes into the area — use it only when you really want them on main.)",
	"先 `kiln worktree new %s`，再 cd 到 worktree 提交。\n": "run `kiln worktree new %s` first, then cd into the worktree and commit there.\n",
	"确要在 main 提交时加 --force（一次性豁免）。":                  "to commit on main anyway, add --force (a one-off waiver).",
	"提示: ": "hint: ",
	"提示: 本仓要求动作在 worktree 里提交——开工前先 kiln worktree new ":                                           "hint: this repository requires actions to be committed in a worktree — run kiln worktree new before starting ",
	"提示: 本仓要求动作在 worktree 里提交；你已有未提交改动——":                                                         "hint: this repository requires actions to be committed in a worktree; you already have uncommitted changes — ",
	"git：git stash -u → kiln worktree new %s → 进区后 git stash pop":                                 "git: git stash -u → kiln worktree new %s → git stash pop inside the area",
	"jj：先在仓库根 mkdir %s，再 jj workspace add --name %s <区路径> -r @（新区即带上它），最后在 main 跑 jj edit @-":     "jj: mkdir %s at the repository root, then jj workspace add --name %s <area path> -r @ (the new area carries it), and finally run jj edit @- on main",
	"jj：先在仓库根 mkdir %s，再 jj workspace add --name %s %s -r @（新区即带上它），最后在 main 跑 jj edit @-":        "jj: mkdir %s at the repository root, then jj workspace add --name %s %s -r @ (the new area carries it), and finally run jj edit @- on main",
	"用法: kiln worktree new <issue> | kiln worktree list | kiln worktree remove <issue> [--force]": "usage: kiln worktree new <issue> | kiln worktree list | kiln worktree remove <issue> [--force]",
	"当前后端不支持 worktree":                                 "the current backend does not support worktrees",
	"用法: kiln worktree new <issue>":                    "usage: kiln worktree new <issue>",
	"非法 issue ID: %q（须为小写 kebab-case slug）\n":          "invalid issue ID: %q (must be a lowercase kebab-case slug)\n",
	"建立 worktree 失败: %v\n":                             "cannot create the worktree: %v\n",
	"下一步: cd 到该目录，再用 `kiln commit %s <action>` 记录动作\n": "next: cd into that directory, then record the action with `kiln commit %s <action>`\n",
	"提示: worktree 是一次性检出——被忽略的构建产物与依赖不在其中，请自行 build / 装依赖": "hint: a worktree is a one-off checkout — ignored build artifacts and dependencies are not in it; build / install them yourself",
	"提示: 未能写入默认配置: %v\n":                         "hint: could not write the default config: %v\n",
	"已写入默认配置: %s\n":                              "wrote the default config: %s\n",
	"提示: 未能播种本地文件: %v\n":                         "hint: could not seed the local files: %v\n",
	"已播种本地文件 %d 个（%s）: %s\n":                     "seeded %d local file(s) (%s): %s\n",
	"后端未列出 main":                                 "the backend did not list main",
	"用法: kiln config path":                       "usage: kiln config path",
	"定位配置失败: %v\n":                               "cannot locate the config: %v\n",
	"（原本不存在，已写入一份带注释的默认配置）":                      "(it did not exist; a commented default config was written)",
	"读取 worktree 失败: %v\n":                       "cannot read the worktrees: %v\n",
	"读取 worktree 状态失败: %v\n":                     "cannot read the worktree state: %v\n",
	"目录已删":                                       "directory deleted",
	"有改动":                                        "changes",
	"未合并":                                        "unmerged",
	"干净":                                         "clean",
	"，":                                          ", ",
	"用法: kiln worktree remove <issue> [--force]": "usage: kiln worktree remove <issue> [--force]",
	"删除 worktree 失败: %v\n":                       "cannot remove the worktree: %v\n",
	"已删除 worktree: %s\n":                         "removed worktree: %s\n",
	"main（只读/只合并）":                               "main (read-only / merge-only)",
	"用法: kiln path <issue> [<issue>…] | --all [--stat|--shortstat|--json]": "usage: kiln path <issue> [<issue>…] | --all [--stat|--shortstat|--json]",
	"--all 与问题参数不能同用":                                                      "cannot combine --all with issue arguments",
	"flag --%s 不接受值: %s\n":                                                 "flag --%s takes no value: %s\n",
	"flag --%s 只能给一次（当前给了 %d 次）\n":                                         "flag --%s may be given only once (got %d)\n",
	"flag --%s 需要一个值（命令末尾）\n":                                              "flag --%s needs a value (end of command)\n",
	"flag --%s 需要一个值（下一个 token 是 %s，以 -- 开头；若它就是字面值，请写 --%s=%s）\n":         "flag --%s needs a value (the next token is %s, which starts with --; if that is the literal value, write --%s=%s)\n",
	"--title 不能为空（每个动作都要有标题）\n":                                            "--title must not be empty (every action has a title)\n",
	"%s 需要 --title（每个动作都要有标题）\n":                                           "%s needs --title (every action has a title)\n",
	"多余的参数: %s\n":                                                          "unexpected argument: %s\n",
	"用法: %s\n":                                                             "usage: %s\n",
	"用法:\n":                                                                "usage:\n",
	"--status 只接受 open 或 done（当前: %s）\n":                                   "--status only accepts open or done (got: %s)\n",
	"--tag 只接受一个标签（当前: %q）；多个标签暂不支持\n":                                     "--tag takes one tag (got: %q); several tags are not supported yet\n",
	"--search 需要一个非空查询串":                                                   "--search needs a non-empty query",
	"没有匹配 %q 的问题\n":                                                        "no issues match %q\n",
	"没有匹配 %s 的问题\n":                                                        "no issues match %s\n",
	"仓库里还没有任何动作（用 kiln do <issue> ask 开一条）\n":                              "no actions in this repository yet (open one with kiln do <issue> ask)\n",
	"仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）\n":                 "no actions in this repository yet (open one with kiln do <issue> ask --title <text>)\n",
	"批量取路径只支持 --json（--all 或多条问题）":                                         "batch paths are JSON-only (--all or several issues)",
	"重建问题的研究路径（--all 一次读全仓；可加 --stat / --shortstat / --json）":              "rebuild the issue's research path (--all reads the whole repository at once; add --stat / --shortstat / --json)",
	"未找到问题: %s\n":                                                          "no such issue: %s\n",
	"%s  %s（当前 %s）\n":                                                      "%s  %s (currently %s)\n",
	"读取变更失败: %v\n":                                                         "cannot read the changes: %v\n",
	"  关系:":                                                                "  relations:",
	"用法: kiln show <issue> [n] [--stat] [--json]":                          "usage: kiln show <issue> [n] [--stat] [--json]",
	"问题没有动作: %s\n":                                                         "the issue has no actions: %s\n",
	"动作序号超出范围: %q（可选 1..%d）\n":                                             "action number out of range: %q (choose 1..%d)\n",
	"没有 %q 动作: %s（它的动作: %s）\n":                                             "no %q action: %s (its actions: %s)\n",
	"提示: %s 有 %d 个 %s（序号 %s）；已显示 %d/%d（最后一个）\n":                            "hint: %s has %d %s action(s) (numbers %s); showing %d/%d (the last one)\n",
	"  看别的：kiln show %s %d\n":                                              "  see another: kiln show %s %d\n",
	"无法识别的选择器: %q（用序号 1..%d，或动作名: %s）\n":                                   "unrecognised selector: %q (use a number 1..%d, or an action name: %s)\n",
	"问题总数: 0（完成 0 / 进行中 0）｜%s\n":                                           "issues: 0 (0 done / 0 in progress) | %s\n",
	"kiln 动作数: %d\n":                                                       "kiln actions: %d\n",
	"提示: 用 `kiln do <issue> <action>` 生成规范提交信息":                            "hint: use `kiln do <issue> <action>` to generate the canonical commit message",
	"提示: 用 `kiln do <issue> <action> --title <文本>` 生成规范提交信息":               "hint: use `kiln do <issue> <action> --title <text>` to generate the canonical commit message",
	"问题总数: %d（完成 %d / 进行中 %d）｜domain %d 个｜%s\n":                            "issues: %d (%d done / %d in progress) | domains: %d | %s\n",
	"kiln 动作数: %d\n\n":                                                     "kiln actions: %d\n\n",
	"worktree %s（已开 %d 个）":                                                 "worktree %s (%d open)",
	" = 已开 worktree——各自状态见 kiln worktree list":                             " = worktree open — see kiln worktree list for each state",
	"｜当前：worktree ":                                                        " | here: worktree ",
	"提示: issue ID 已归一化：%q → %q\n":                                          "hint: issue ID normalised: %q → %q\n",
	"拒绝: 问题 %q 已经有起点动作 %q（共 %d 个动作）——同名即同一条问题，不需要再新建\n":         "refused: issue %q already has its start action %q (%d actions in all) — same name, same issue; there is nothing to create\n",
	"提示: 要继续它，用 kiln commit %s <action>（可选: %s）\n":              "hint: to continue it, run kiln commit %s <action> (allowed: %s)\n",
	"提示: 要继续它，用 kiln commit %s <action> --title <文本>（可选: %s）\n": "hint: to continue it, run kiln commit %s <action> --title <text> (allowed: %s)\n",
	"没有 kiln 动作（检查提交信息是否同时带了 issue: 与 action:）":                 "no kiln actions (check that the commit message carries both issue: and action:)",
	"--json 与 --mermaid 冲突":                     "--json conflicts with --mermaid",
	"--json 与 --format=%s 冲突\n":                 "--json conflicts with --format=%s\n",
	"--mermaid 与 --format=%s 冲突\n":              "--mermaid conflicts with --format=%s\n",
	"仅支持 --format=ascii|mermaid|json（当前: %s）\n": "only --format=ascii|mermaid|json is supported (got: %s)\n",
	"（空）":                        "(empty)",
	"--issue/--depth 只服务 --json": "--issue/--depth only work with --json",
	"--depth 需要 --issue":         "--depth needs --issue",
	"--depth 需要非负整数":             "--depth needs a non-negative integer",
	"读取历史失败: %v\n":               "cannot read the history: %v\n",
	"未知":                         "unknown",
	"已完成":                        "done",
	"进行中":                        "in progress",
	"输出 JSON 失败: %v\n":           "cannot write the JSON: %v\n",
	"读取 %s: %w":                  "read %s: %w",
	"解析 %s: %w":                  "parse %s: %w",
	"非法 worktree: %q（可选 on / off）":         "invalid worktree: %q (choose on / off)",
	"不在仓库里":                                "not a repository",
	"不在仓库里，定位不到 %s":                        "not in a repository, cannot locate %s",
	"第 %d 行: 块注释没有闭合的 */":                  "line %d: block comment has no closing */",
	"缺少 %s：问题没有起点动作":                       "missing %s: the issue has no start action",
	"缺少终点动作（当前 %s）":                        "missing a terminal action (currently %s)",
	"关系目标不存在：%s 指向 %s":                     "relation target missing: %s points at %s",
	"未知 trailer 键: %s":                     "unknown trailer key: %s",
	"完整（%s）":                               "complete (%s)",
	"（%s ＝ %s）":                            " (%s = %s)",
	"relation slot 缺 name":                 "a relation slot has no name",
	"relation name 为空":                     "a relation name is empty",
	"relation 重名: %s":                      "duplicate relation name: %s",
	"relation %s 的 slot 不存在: %s":           "relation %s uses an unknown slot: %s",
	"relation %s 的 rank 必须为正":              "relation %s needs a positive rank",
	"alias %s 指向未知的 relation %s":           "alias %s points at an unknown relation %s",
	"slot %s 内 rank %d 被 %s 与 %s 共用（槽内全序）": "slot %s has rank %d shared by %s and %s (ranks are a total order per slot)",
	"vcs=git，但 %s 下没有 .git":                "vcs=git, but there is no .git under %s",
	"vcs=jj，但 %s 下没有 .jj":                  "vcs=jj, but there is no .jj under %s",
	"未知 vcs: %q（可选 auto / git / jj）":       "unknown vcs: %q (choose auto / git / jj)",
	"%s（从 %s 向上也没有 .git / .jj）":            "%s (and there is no .git / .jj upward from %s)",
	"没有改动可记录: %s":                          "no change to record: %s",
	"worktree 还有未提交的改动: %s（要丢弃请加 --force）": "the worktree still has uncommitted changes: %s (add --force to discard them)",
	"分支 %s 还有未合并的提交（要丢弃请加 --force）":        "branch %s still has unmerged commits (add --force to discard them)",
	"jj workspace list 未列出当前 worktree: %s": "jj workspace list did not list the current worktree: %s",

	"(未分类)": "(no domain)",

	// Relation glosses, reached through RelationSet.Gloss rather than a literal
	// at the call site.
	"我阻塞谁":  "what I block",
	"谁阻塞我":  "who blocks me",
	"我属于谁":  "what I am part of",
	"谁属于我":  "what belongs to me",
	"无向的关联": "an undirected association",
	"谁引发了我": "who caused me",
	"我引发了谁": "what I caused",
	"我取代谁":  "what I supersede",
	"谁取代了我": "who supersedes me",

	// The command list under the usage header: its descriptions live in a table
	// and are translated when the table is rendered.
	"生成规范提交信息（不写文件）":                              "print the canonical commit message (writes no file)",
	"用规范信息直接提交（--only 只提交列出的路径）":                  "commit with the canonical message (--only commits just the listed paths)",
	"重建问题的研究路径（可加 --stat / --shortstat / --json）": "rebuild an issue's research path (add --stat / --shortstat / --json)",
	"展开某个动作的完整变更（可加 --stat / --json）":             "show one action's full change (add --stat / --json)",
	"全库概览":                "whole-repository overview",
	"语义完整性校验":             "semantic completeness check",
	"按条件过滤问题":             "filter issues by field",
	"列出可用的 action 类型（词表）": "list the available action types (vocabulary)",
	"全局动作流（一条动作一行；--jsonl 给 agent 读；--limit 只看最近 N 条）":                      "the global action stream (one action per line; --jsonl is for agents; --limit keeps the newest N)",
	"action list 用 --json，不用 --jsonl":                                       "action list takes --json, not --jsonl",
	"--limit 只服务 action log":                                                "--limit belongs to action log",
	"action log 用 --jsonl（一行一个动作），不用 --json":                                "action log takes --jsonl (one action per line), not --json",
	"用法: kiln action list [--json] | kiln action log [--jsonl] [--limit N]": "usage: kiln action list [--json] | kiln action log [--jsonl] [--limit N]",
	"--limit 需要 ≥ 1 的整数（当前: %q）\n":                                          "--limit needs an integer >= 1 (got: %q)\n",
	"当前后端不支持 action log 的流式读取":                                              "this backend has no streaming read for action log",
	"输出失败: %v\n": "cannot write the output: %v\n",
	"问题关系图（--mermaid 出流程图文本；--json 给 agent 读，--issue/--depth 收窄邻域）": "issue relation graph (--mermaid prints flowchart text; --json is for agents, --issue/--depth narrows the neighbourhood)",
	"建 worktree（main 只读，动作只在 worktree 提交）":                          "create a worktree (main is read-only; actions commit in a worktree)",
	"列出全部 worktree（含 main）及各自状态（可加 --json）":                         "list every worktree (main included) and its state (add --json)",
	"删除 worktree（尚有工作的默认拒绝，--force 强删）":                             "remove a worktree (refused while it still has work; --force overrides)",
	"打印配置路径（缺失时写入一份带注释的默认配置）":                                       "print the config path (a commented default config is written when missing)",
	"打印版本、可执行文件路径与构建时间（-V / version 同义）":                            "print the version, executable path and build time (-V / version are the same)",

	// Multi-line blocks: the key must match cli.go's usage consts and config.go's
	// seed byte for byte, because the lookup is by content. A test renders the
	// help in English and fails on any remaining CJK.
	`kiln — git-trailer 驱动的问题追踪与路径重建

用法:
`: "kiln — issue tracking and path rebuilding driven by git trailers\n\nusage:\n",
	`
action 词表默认: ask | hypothesize | collect | test | conclude | apply（生效词表见 kiln action list）
worktree 默认: off（允许在 main 提交）| on（动作只在 worktree 里提交——开工前先 kiln worktree new <issue>）
提交范围: 默认提交当前工作区的全部改动（快照）；--only <paths> 只提交列出的路径（两后端同义；空动作允许）
（配置：仓库根的 .kilnconfig.jsonc，JSONC；用 kiln config path 打印路径）
`: "\ndefault action vocabulary: ask | hypothesize | collect | test | conclude | apply (effective vocabulary: kiln action list)\ndefault worktree: off (commits on main allowed) | on (actions commit in a worktree only — run kiln worktree new <issue> first)\ncommit scope: by default the whole working area is committed (a snapshot); --only <paths> commits just the listed paths (same meaning on both backends; empty actions are allowed)\n(config: .kilnconfig.jsonc at the repository root, JSONC; kiln config path prints the path)\n",
	`
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
`: `
path flags:  --stat（a file summary per action）  --shortstat（one total line per action）  --json（machine-readable output）
extra path flag:  --all（read the whole repository in one process; --json only, or pass several issues directly）
show flags:  --stat（the file summary for that action only）  --json（machine-readable output）
worktree list flags:  --json（machine-readable output）
status / list / check flags:  --json（machine-readable output: stdout carries one JSON document only）
extra list flag:  --status:  open（still in progress）| done（the last action is terminal）
extra list flag:  --tag:  one literal tag, commas are not split (multi-tag filtering is not supported yet)
extra list flag:  --search:  whitespace-split terms, AND, literal, case-insensitive (id / title / action subject)
extra commit flag:  --force（**required** to commit on main while worktree=on, as a one-off waiver）  --only <paths>（commit just the listed paths）
extra worktree remove flag:  --force（remove even with an unmerged branch / a worktree that still has changes）
--lang <zh|en>:  global language switch (by default detected from KILN_LANG / LC_ALL / LC_MESSAGES / LANG, Chinese when unset)
`,
	`// kiln 配置 —— 随仓库提交（项目共享），放在仓库根。
// 位置：<仓库根>/.kilnconfig.jsonc（kiln config path 打印路径）
// 格式：JSONC（JSON + 注释）；除注释外是严格 JSON，不支持尾逗号。
{
  // action 词表：每个项目可以不同 —— 这是 kiln 的自定义点。
  // 顺序不是约束；只需含起点动作，并以终点动作收尾。
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  // VCS 后端：auto（.jj 优先，否则 .git）/ git / jj
  "vcs": "auto",
  // worktree 模式：off（默认，允许在 main 提交）| on（动作只在 worktree 提交——开工前先 kiln worktree new <issue>）
  "worktree": "off"
}
`: `// kiln config — commit it with the repository (shared by the project), at the repository root.
// Location: <repository root>/.kilnconfig.jsonc (kiln config path prints it)
// Format: JSONC (JSON + comments); apart from comments this is strict JSON, so no trailing commas.
{
  // action vocabulary: every project may differ — this is kiln's customisation point.
  // Order is not a constraint; it only has to contain the start action and end with a terminal action.
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  // VCS backend: auto (.jj first, otherwise .git) / git / jj
  "vcs": "auto",
  // worktree mode: off (default, commits on main allowed) | on (actions commit in a worktree only — run kiln worktree new <issue> first)
  "worktree": "off"
}
`,
}
