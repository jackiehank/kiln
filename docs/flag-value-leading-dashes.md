# issue: flag-value-leading-dashes — 实测：标量 flag 取值被吞的三种形态

- **domain**：cli
- **关联**：relates-to `refs-empty-value` / `silent-flag-drop`
- **实测版本**：`kiln v0.1.2-0.20261027151656-73a780c46267`（HEAD `73a780c`）；Windows；git 2.55.0；Go 1.27；PowerShell 7.6.9

## 0. 一句话

`parse` 的取值规则是「下一个 token **不以 `--` 开头**才当值」。于是「合法的值以 `--` 开头」与「漏写值、后面紧跟另一个 flag」得到**同一个解析结果**：该 flag 的值变成空串，那个 token 被当成新 flag。空值随后按各命令自己的规则落地，分成三种截然不同的形态——**显式拒绝 / 警告但丢字段 / 完全静默**。最危险的是第三种：被吞的值**恰好是本命令的已知 flag** 时，只报未知 flag 的警告机制不会触发，丢的是一个字段，生效的是**另一个 flag 的语义**。

## 1. 代码事实（先把规则钉住）

`internal/cli/cli.go`：

| 位置 | 事实 |
| --- | --- |
| `parse`（L580-604） | `--k=v` 按第一个 `=` 切分，值**无条件**采用；否则若 key 不在 switches 里、下一 token 存在且**不以 `--` 开头**，就把下一 token 当值；其余情况值为 `""`。不以 `--` 开头的 token 一律是位置参数，因此 `-x` 永远不是 flag。 |
| switches | 每个命令显式列出：do/commit `force`；path `all json stat shortstat`；show `stat json`；status `json`；check `json verbose`；list `json`；graph `json mermaid`。switch 永不消费下一个 token——这条本身是对的，与取值规则正交。 |
| `extractLang`（L496-524） | 分派前独立解析全局 `--lang`：**无条件**吃掉下一个 token 再校验（`--lang=zh` 亦可）。它是当前唯一「总是当值」的取值 flag：`--lang --json` → `--lang 只接受 zh 或 en（当前: --json）`；末尾裸 `--lang` → `--lang 需要一个值（zh 或 en）`。 |
| `warnUnknownFlags`（L352-365） | 任何不在白名单里的 flag → stderr `警告: 未知 flag --<key>（可用 flag 见 kiln -h）`，**退出码不变**。它只认「名字」，不知道这个名字本来是别人的值。 |
| `write`（L658-708） | `title/target/scope/domain` 取最后一个值；`refs/tag` 走 `f.list`（逗号切分、空白丢弃）。空值交给 `trailer.Format`，标量行被跳过；`title` 为空时连头行都不写，提交第一行变成 `issue: …`。 |
| `write`（L721） | `--force` 的判据是 `f.has("force")`——**存在即真**，哪怕值是空串。 |

概括：kiln 没有采用下面第 4 节里的任何一条主流惯例，而是**按形状猜**（`--` 前缀）。在词法上，「值」与「下一个 flag」因此不可区分。

## 2. 实测矩阵

`do` 的用例在真仓库跑（只打印、不写仓库）；`commit` 一例在 `$env:TEMP` 的临时 git 仓库跑，未污染本仓。

### A. 被吞的值恰好是本命令已知 flag → 无警告、字段丢失、**另一个 flag 生效**（exit 0）

| # | 命令 | 观察 |
| --- | --- | --- |
| A1 | `kiln list --domain --json` | 63 行全量，`filter` 为 `{}`；无警告；exit 0 |
| A2 | `kiln list --tag --json` / `kiln list --action --json` | 同上——过滤条件凭空消失 |
| A3 | `kiln graph --issue --json` | 整图 `{issues:63, edges:89}`——本意「某个 issue 的邻域」变成全量 |
| A4 | `kiln do <id> collect --title --force` | 消息无 `title:`（头行是 `issue:`）、**无警告**；`force` 被记为真 |
| A5 | `kiln do <id> collect --title --tag x` | 无 title，`tags: x`；无警告——标题丢了，`--tag` 独立生效 |
| A6 | 临时仓 `kiln commit probe apply --title --only a.txt` | 提交信息为 `issue: probe` / `action: apply`（无标题）、只含 `a.txt`（同目录的 `b.txt` 留在工作区）；无警告；exit 0 |

A4-A6 是**语义级**后果，不只是「少一行」：`--force` 一次性豁免 worktree 守卫、`--only` 缩小提交范围、`--tag x` 加标签——而用户以为自己只是给了个标题。

### B. 被吞的值不是已知 flag → 一行警告 + 字段丢失（exit 0）

| # | 命令 | 观察 |
| --- | --- | --- |
| B1 | `kiln do <id> collect --title --json` | 无 title；`警告: 未知 flag --json` |
| B2 | `kiln do <id> collect --scope "--json 布尔 flag"` | 无 scope；`警告: 未知 flag --json 布尔 flag`——**把整个值当成 flag 名**，带空格时读起来是病句 |
| B3 | `--refs --json` / `--target --json` / `--relates-to --json` | 对应行消失；同样的 `未知 flag --json` |

警告把「值」报成「flag」，方向正好相反：用户看到「我不认识 `--json`」，真正的问题是「`--title` 没有值」。

### C. 已经显式拒绝的（exit 2）——现有护栏，可作参照

| 命令 | 输出 |
| --- | --- |
| `kiln list --search --json` | `--search 需要一个非空查询串` |
| `kiln list --status --json` | `--status 只接受 open 或 done（当前: ）` |
| `kiln graph --depth --json` | `--depth 需要 --issue`（用户确实给了 `--depth`，消息指错了地方） |
| `kiln graph --issue <id> --depth --json` | `--depth 需要非负整数` |
| `kiln graph --json=1` | `flag --json 不接受值: 1` |

C 组是「空值有校验」的 flag；D 组是「空值等于不设置」的 flag。同一个解析结果，含义完全由各命令自行决定。

### D. 静默：缺值、显式空串、假终止符

| # | 命令 | 观察 |
| --- | --- | --- |
| D1 | `kiln do <id> collect --title`（末尾） | 无 title；无警告；exit 0 |
| D2 | `kiln do <id> collect --title ""` | 与 D1 输出逐字节相同——**空串与缺值不可区分** |
| D3 | `kiln do <id> collect -- --title y` | `警告: 未知 flag --`；`title: y` 照常生效——`--` 不是终止符，而是名为空串的 flag |
| D4 | `kiln do <id> collect --title "-x"` | `title: -x` 正常——只有 `--` 前缀触发，单横线不受影响 |

### E. 历史量化

```
git log --all --format=%B，匹配 ^(title|scope|refs|target|domain|tag|only|search):
  709 行；含 "--" 的 83 行（title 77、scope 6）；以 "--" 开头的 0 行；全仓 697 个提交
```

含 `--` 的值都出现在**串中**，例如 `title: apply：--only 只校验一次…`，因此不受影响。以 `--` 开头的 0 行不能当作「没发生过」：那种输入正是被吞掉的一类，根本没有机会进入历史。历史只能证明「串中的 `--` 没事」与「`=` 写法可用」，给不出发生频率。

## 3. 触发面

- **自由文本**：`--title` / `--scope` / `--refs` / `--search`。后两者尤其容易——搜索一个 flag 名、引用一段含 `--foo` 的文本，都是正常用法。
- **结构化值**：`--domain` / `--tag` / `--action` / `--status` / `--format` / `--issue` / `--depth` / `--only`。值本身以 `--` 开头几乎不可能，但「漏写值、下一 token 是 flag」同样落到这里。
- **两种成因、一个结果**：(i) 合法的值以 `--` 开头；(ii) 忘写值。解析器区分不了，只能由调用方约定。
- **静默那一档为何最危险**：`warnUnknownFlags` 只在名字不在白名单时出声；被吞的值若是白名单里的名字（`--force` / `--only` / `--json` / `--tag`），既没有警告，又有一个真实的 flag 被点亮。

## 4. 与三条主流惯例的对照（本机实测）

| 惯例 | 实测 | 结果 |
| --- | --- | --- |
| git：取值 flag 无条件吃下一个 token | `git log -1 --pretty=%s --grep --stat` 返回 subject 含 `--stat` 的提交；`--grep=--stat` 结果相同 | 值就是值，哪怕长得像选项 |
| git：不认识的选项直接失败 | `git log --nope` → `fatal: unrecognized argument: --nope`，exit 128 | 不猜 |
| Go `flag`：无条件吃下一个 token | `-t --json` → `t="--json"`；`-t` 单独 → `flag needs an argument: -t`，exit 2 | 值就是值；缺值报错 |
| POSIX/GNU `--`：终止选项 | kiln 无此语义：`--` 变成名为空串的未知 flag（见 D3） | 没有逃生门 |

kiln 现在三条都不满足：既不无条件吃值，也不在缺值时报错，也没有 `--` 终止符；它靠「形状」猜，而形状恰好是唯一的歧义源。

## 5. 与既有裁决的接口

- `silent-flag-drop`（已 apply）：补上了「未知 flag → 警告」，是本 issue 的**部分缓解**——B 组有声音；但 A 组（值撞上已知 flag）绕过了它，因为它检查的是名字。
- `refs-empty-value`：空值跳过、不报错，是 D 组「空串等于不设置」的来源；`--title ""` 因此无法表达。
- `list --search` 的空值检查（C 组）是**唯一把空值当错误**的样例，证明「显式拒绝」在现有结构里可行。
- 现有测试 `TestParseLeavesPositionalsAfterSwitches` 只覆盖「switch 不吃位置参数」与常规取值；以 `--` 开头的值**没有测试**——这是未定义行为，不是回归。

## 6. 待定（交给 hypothesize，不在此裁定）

1. **取值策略**：A 无条件吃掉下一 token（对齐 git/go；代价：`--title --force` 变成字面值 `--force`，忘写值会把下一个 flag 吞成值）；B 缺值即报错、以 `--` 开头的值必须用 `=`（严格；代价：`--title --force` 也被拒，写法更啰嗦）；C 维持现状 + 支持 `--` 终止符（代价：只要有人不用 `--`，静默仍在）。
2. **空值语义**：空串算「不设置」（现状）还是错误？`--title ""` 与 `--title` 要不要分开？各 flag 是否统一（现在 search/status 拒绝，domain/tag/action 放行）？
3. **警告文案**：被吞时能否报「`--title` 缺少值，下一个 token 是 `--json`」，而不是「未知 flag --json」？
4. **兼容性**：改成无条件吃值后，`--title --force` 会从「静默丢标题」变成「标题字面为 `--force`」——两种都不对，但后者至少**可见**；是否接受这次行为迁移？

## collect 的非目标

- 不改 `parse`、不改任何命令的行为（collect 只收事实）。
- 不重开 `silent-flag-drop` 的「未知 flag 只警告、不改退出码」裁决；不讨论 `--toon` 等输出格式。

## hypothesize（2026-10-27）：不猜——已知 flag 缺值即拒绝，字面值走 `=`

### 主张（逐条回答 §6）

1. **取值策略选 B，但只对已知 flag 拒绝**：空格形式下，若下一个 token 以 `--` 开头、而当前 flag 在**本命令白名单**里 → **exit 2**，并给出 `=` 写法；若当前 flag **未知**，维持 `silent-flag-drop` 的裁决——只警告、exit 0。字面值以 `--` 开头时一律写 `--flag=值`。
2. **空值语义不动**：`--flag=` 与 `--flag ""` 仍表示「不设置」；`--search` 拒绝空查询、`--status` 拒绝空值、`--domain` 空 = 无过滤，全部照旧（`refs-empty-value` 与各命令的裁决不重开）。
3. **警告文案**：这类输入不再走到 `warnUnknownFlags`（它在解析后才跑）——报错直接说明哪个 flag 缺值、下一个 token 是什么、字面值该怎么写。
4. **兼容性**：破坏性，但只覆盖「以前静默成功」的输入；历史 0 条以 `--` 开头的值，不设过渡期（写侧、可恢复）。

### 规则（精确到可写测试）

| 输入 | 结果 |
| --- | --- |
| `--title --json`（`title` 已知） | exit 2：`flag --title 需要一个值（下一个 token 是 --json，以 -- 开头；若它就是字面值，请写 --title=--json）` |
| `--title`（末尾） | exit 2：`flag --title 需要一个值（命令末尾）` |
| `--title=--json` | title = `--json` |
| `--title=` / `--title ""` | 显式空 → 不设置（照旧）；`--title "-x"` → `-x`（照旧） |
| `--newflag --json`（`newflag` 未知） | 只警告 `未知 flag --newflag`，`--json` 照常生效（照旧，保 forward-compat） |
| switch（`--json …` / `--stat …`） | 永不消费下一个 token（照旧） |
| 多个缺值 | 按 argv 顺序报第一个（与现有「一次一条错误」一致） |

### 为什么是拒绝，而不是 git/go 的「无条件吃值」

collect 第 4 节已确认：git 与 Go `flag` 都无条件吃下一个 token，只在末尾缺值时报错。kiln 若照抄（方案 A），实测的四类形态会变成：

- `--title --force` → 标题字面 `--force`，`force` 不再生效——语义劫持消失，但**一次漏写值变成永久记录的错误标题**（改标题要重写历史，见 `evidence-fluidity` 的代价）；
- `--scope --jsno` → 拼错的 flag 被吞成 scope；
- `--only --json`（collect 实测：现在**直接提交全部**、exit 0）→ 方案 A 下变成「路径 `--json` 没有改动」——仍是错误，但指向路径，不指向真正的笔误。

kiln 的写侧已有判例：重复标量、多余位置参数、switch 带值，全部 **exit 2**，理由都是同一个——不让错输静默落地。方案 B 是这条判例在「取值」上的自然延伸：**歧义不可判定，就拒绝，并教一次写法**。

### 为什么只拒已知 flag（而不是一律拒）

`--newflag --json`（未知 flag 后跟另一个 flag）若一律报错，会破坏 `silent-flag-drop` 定下的 forward-compat：老二进制收到新 agent 发的 flag 只能警告、不能失败。因此缺值检查只对**本命令白名单里的 value flag** 报错；未知名字仍是「警告 + 忽略」。这条边界同时解释了为什么错误信息敢直接教 `=` 写法——只有已知 flag 才走到这里。

### 备选与否决

| 方案 | 形态 | 否决理由 |
| --- | --- | --- |
| A 无条件吃值（git / Go `flag`） | 值永远是下一个 token；末尾缺值才报错 | 漏写值 / typo 被记成字面值，错误在记录之后才可见；与写侧「拒绝静默」判例相反 |
| **B 已知 flag 缺值即拒绝（本假设）** | 见上表 | 代价：`--` 开头的字面值必须写 `=`；与主流 CLI 习惯不同（一次学习） |
| C 现状 + `--` 终止符 | 加逃生门，其余照旧 | 不修主因：只要不用 `--`，静默与语义劫持仍在 |
| D 警告后按空值处理 | 出声但不失败 | 警告在 `commit` 之后到达（提交已落地）；与重复 flag 的 error 判例不一致 |

### 契约（apply 的验收）

1. `kiln do x collect --title --force` → exit 2，消息含 `--title`、`需要一个值`、`--title=--force`；stdout 无消息。
2. `kiln commit x apply --title --only a.txt` → exit 2，**不产生提交**（`git log` 不变）。
3. `kiln list --domain --json` / `kiln graph --issue --json` → exit 2，不再是全量输出。
4. `kiln do x collect --title=--json` → exit 0，title 为 `--json`；`--title "-x"` 照旧。
5. `kiln do x collect --title ""`、`--title=` → 照旧无 title、exit 0。
6. `--json <pos>` / `--stat <issue>` 顺序仍不消费位置参数（回归）。
7. `kiln list --newflag --json` → 警告 + 正常 JSON、exit 0（forward-compat 回归）。
8. 中英双语下文案都成立（`--lang en` 断言一次）。

### 落点

- `parse`（L580-604）返回值加 `missing`（flag 名 + 下一个 token）；**不**在 parse 里报错——是否拒绝由命令决定。
- 新增 `rejectMissingValues(errw, missing, allowed)`：只对 `allowed` 里的名字报错（exit 2）；未知名字留给 `warnUnknownFlags`。9 处 `parse` 调用点各加一行，且**先于** `warnUnknownFlags`（错误优先，避免再冒一条「未知 flag」）。
- `extractLang` 对齐：`--lang --json` 报「需要一个值」，不再报「只接受 zh 或 en（当前: --json）」。
- i18n 两个键 + `-h` 的 flags 说明补一行：值以 `--` 开头时写 `--flag=值`。
- 测试：parse 单测 5 条（已知 / 未知 / 末尾 / `=` / 单横线）；命令级 7 条（上面 1-7）；改 1 条既有断言——`TestListSearchRejectsAnEmptyQuery` 的末尾 `--search` 用例，期望从「非空」改为「需要一个值」。

### 检验办法（预测，apply/test 时验证）

- collect 的 A/B/C/D 四组里，凡「值以 `--` 开头」的已知 flag 用例全部收敛为 exit 2；`=` 用例全部不变；未知 flag 用例仍 exit 0 + 警告。
- 既有测试除上述 1 条断言外零改动（switch 不消费、位置参数顺序、search 空串、`--json=x` 拒绝等）。
- 预计净增代码 < 60 行（不含测试）。

### 反方（保留）

- **与肌肉记忆冲突**：`git commit -m --fix`、`flag` 的 `-x --y` 都能用，kiln 会拒绝。辩护：kiln 的值 flag 少、`--only` / `--force` 误吞的代价高（提交范围、守卫豁免），且错误信息直接给出可用写法——「一次学习换永不静默」。
- **MCP 化后规则失效**：结构化调用不经过 argv，这条规则只在 CLI 表面生效——不构成长期负担，也不阻碍规模化。
- **字面值确实常见吗**：`--title "--json 输出"` 是真实需求；保留 `=` 后仍可写，只是从「唯一绕法」变成「规定写法」。

### 不做（本假设范围外）

- 不加 `--` 终止符（无位置参数以 `--` 开头；多一条规则不划算）。若将来出现以 `--` 开头的路径 / issue id，再议。
- 不统一各命令对显式空串的语义（`--search ""` 拒绝、`--domain ""` 无过滤）——那是命令层裁决，不在解析器这一层。
- 不设兼容期、不自动改写；不做 `--flag:` 之类的第二语法。

## hypothesize（续，2026-10-27）：title 由谁写——手填、采纳既有 HEAD，还是接受整条消息

### 问题拆成两层

上一版把 `--title` 定为必填，是把两件事捆在了一起：

- **必填**：动作必须有标题（已定）；
- **来源**：标题的值从哪来（本轮）。

三个候选：

- **M1 手填**（上一版）：`--title` 参数，kiln 造消息。
- **M2a 采纳既有 HEAD**：先 `git commit` 写出自然消息，再让 kiln 读首行作 title、补 trailer、改写 HEAD。
- **M2b 接受整条消息**：`-F/--message` 给整条消息，kiln 仍造新提交，title = 首行。

> 关键区分：`kiln commit` **自己造消息**，对它来说不存在"已存在的消息"。所以 M2a 是另一条流程（先提交、后认领），不能塞进 `commit` 里隐式发生；M2b 才是"消息由调用方给、提交由 kiln 造"。

### 实测前提（本机）

1. `git commit --amend -m` 会把**暂存区改动卷进提交**：临时仓里暂存 `a.txt` 的修改后 amend，`git show --stat` 出现该改动。→ M2a 必须要求索引干净，否则会静默扩大提交。
2. 索引干净时 amend **只改消息**：临时仓里 subject 与正文保留、trailer 追加、tree 不变，hash 变（`78c70f9` → `03bc8ef`）。
3. 本仓 302 个动作、0 个 merge，消息形状统一（subject + trailer 块）；HEAD 领先 `origin/master` 36 个提交且**尚未推送**（`git branch -r --contains HEAD` 为空）。→ adopt 的可用窗口是"刚提交、未推送"。

### M2a：`kiln adopt <issue> <action>`（独立命令，不碰 `commit`）

前置（任一不满足即 exit 1，什么都不写）：

- HEAD 存在；消息首行非空、且不像 `key: value` 的 trailer 行；
- HEAD 消息里没有 `issue:` / `action:`（已经是动作 → 拒绝）；
- **索引干净**（`git diff --cached --quiet`）——否则 amend 会把暂存改动卷进去；
- HEAD 不在任何远端分支上（`git branch -r --contains HEAD` 为空）——已发布的历史**默认拒绝**，`--force` 只放开这一条。

动作：title = 首行（trim）；消息 = 原消息 + 空行 + trailer 块（原消息已有 trailer 块时追加进同一块）；`git commit --amend -F -`。tree / parent / author 不动，committer 更新，hash 变。

jj 侧：`jj describe -r <change> -m <msg>`。jj 没有暂存区，但会 rebase 后代，仍需判断"是否已在远端"。

明确不做：非 HEAD 的采纳（要改写后代）、从消息里猜 issue/action、读 `.git/COMMIT_EDITMSG` 草稿。

### M2b：`-F/--message`（与 `--title` 二选一）

- `kiln commit <issue> <action> -F <文件|->`：消息 = 给定文本 + trailer 块；title = 首行。
- 无历史改写；`--title` 与 `-F` 同时给 → exit 2。
- 代价：弱化"trailer 由结构化参数生成"的取向；收益是 title 直接取首行，subject 与 `title:` 天然一致。

### 对照

| | M1 手填 | M2a adopt | M2b `-F` |
| --- | --- | --- | --- |
| 谁先写消息 | kiln 参数 | git / jj | 调用方 |
| 命令数 | 1 | 2（git commit + adopt） | 1 |
| 历史改写 | 无 | HEAD 重写（hash 变） | 无 |
| 失败模式 | 忘记 / 被吞 | 索引脏、已推送、merge、消息不合规 | 与 `--title` 二选一 |
| 实现成本 | 本批已有 | 新命令 + 两后端 + 守卫 | 小（读 `-F` + 取首行） |
| 适用 | kiln-first（本仓 302/302） | git-first | 想"只写一次"又不改历史 |

### 建议（本假设）

1. **M1 仍是主路径**：本仓所有动作都是 kiln 造的消息，没有 git-first 痕迹；adopt 服务的是一个尚不存在的工作流。
2. **若目标只是"标题只写一次"**：M2b 比 M2a 小得多，且不碰历史——优先做 M2b，不做 adopt。
3. **若确实要 git-first**：M2a 只能作为**独立命令**，四条前置必须硬性（HEAD-only、索引干净、未推送、无 trailer）；`--force` 只放开"未推送"，不放开"索引干净"。
4. 不推荐"从改动派生标题"（文件清单做成标题会把语义标题变成变更摘要，与 `title` 的用途相反）。
5. 顺带说明：无论 M1/M2b，`check` 都不比对 subject 与 `title:` 的一致性（手改任一都会漂移）——本批不做。

### 预测（选了再验证）

- adopt 后 `%s` == 原 subject、`%b` 末尾多出 `issue` / `action` / `title` 三行；`git log --oneline` 不变。
- 索引有暂存 → 拒绝且工作区不变；已推送 → 拒绝；HEAD 已是动作 → 拒绝。
- `-F` 后首行 == `title:`；`-F` + `--title` → exit 2。

### 待你定

1. 你的真实流程是 git-first（先 `git commit`）还是 kiln-first（`kiln commit` 一次到位）？这决定要不要做 M2a。
2. 若做 M2a：`--force` 允许改写**已推送**的 HEAD 吗？
3. 若做 M2b：消息的正文（body）保留吗，还是只取 subject？

> **裁决（2026-10-28）**：选 M1——`--title` 手动填、必填；`do` 与 `commit` 同判（`do` 只是不提交）；M2a（adopt）与 M2b（`-F`）都不做。

## apply（2026-10-28）：已知 flag 缺值即拒绝，`--title` 必填（do / commit 同判）

### 改了什么

1. `parse` 返回值加 `missing []missingValue`：value flag 后面既没有 token、或下一个 token 以 `--` 开头时，记下「flag + 下一个 token」，**不在解析器里报错**；`--k=` / `--k ""` 是显式值，不算缺值。
2. 新增 `rejectMissingValues(errw, missing, allowed)`：按 argv 顺序，只拒**本命令白名单里**的名字；未知名字继续交给 `warnUnknownFlags`（forward-compat 边界，见 `silent-flag-drop`）。
3. 调用点：`write`（do/commit）、`list`、`graph`。其余六个命令没有 value flag，缺值条目必然不在 `allowed` 里，调用恒为 no-op——`parse` 已把信息备好，将来给它们加 value flag 时补一行即可。
4. `write`：缺值检查排在 `allowed` 计算之后、`warnUnknownFlags` 之前；title 校验排在 do 的 `--only` 拒绝之后、格式化与开后端之前。`strings.TrimSpace(title)` 为空即拒：**没给** → `do/commit 需要 --title`，**给了但空白** → `--title 不能为空`；`Format` 用 trim 后的值。
5. `extractLang`：`--lang --json` 归入「需要一个值」，不再报「只接受 zh/en（当前 --json）」。
6. 帮助与提示：do/commit 用法串带上 `--title <text>`；flags 头改成「位置参数 `<issue> <action>` 与 `--title` 必填；其余 flag 选填」；`--title` 词条标「必填」；说明行加「值以 `--` 开头时写 `--flag=值`」；`status` 空仓提示、`list` 空仓提示、重复起点提示都补上 `--title`。
7. README / README.en：命令表与 flags 段同步（含 `=` 写法与必填说明）。

### 行为契约（本机实测）

| 输入 | 改前 | 改后 |
| --- | --- | --- |
| `do … --title --force` | 丢标题、`force` 生效、exit 0 | exit 2：`flag --title 需要一个值（…请写 --title=--force）`；force 不生效 |
| `do … --title`（末尾） | 丢标题、exit 0 | exit 2：`需要一个值（命令末尾）` |
| `do … --title ""` | 丢标题、exit 0 | exit 2：`--title 不能为空` |
| `do …`（无 `--title`） | 无标题消息、exit 0 | exit 2：`do 需要 --title` |
| `do … --title=--json` | title = `--json` | 不变（首行与 `title:` 都是 `--json`） |
| `do … --title "-x"` | title = `-x` | 不变（单横线不触发） |
| `commit … --title --only a.txt` | 无标题 + 只提交 a.txt | exit 2，**不产生提交**（临时仓 commits 1→1） |
| `list --domain --json` | 全量、`filter {}` | exit 2 |
| `graph --issue --json` | 整图 | exit 2 |
| `list --newflag --json` | 警告 + 正常 JSON、exit 0 | 不变（forward-compat） |
| `--lang --json` | `只接受 zh 或 en（当前: --json）` | `--lang 需要一个值（zh 或 en）` |

### 测试

- `go test ./...` 全绿；`gofmt -l` 无输出。
- 新增 6 个测试：parse 缺值上报（含 `=` 与单横线对照）、已知 flag 缺值拒绝（do/commit）、title 必填与空白、读命令拒绝、未知 flag 仅警告、`--lang` 缺值。
- 既有测试：`TestListSearchRejectsAnEmptyQuery` 拆成两段（末尾 `--search` 现在报缺值）；28 处成功路径调用补 `--title`。拒绝类用例不动——重复起点、非法 action、多余参数仍然先于 title 报出。

### 与计划的差异

hypothesize 写的是「9 处 parse 调用点各加一行」；实际只在 `write` / `list` / `graph` 三处调用（理由见上第 3 条）。这是有意收窄，不是遗漏。

### 未做（明确）

- 不加 `--` 终止符。
- 不改显式空值语义：`--refs ""` 仍跳过，`--search ""` 仍报非空；`--title=` 现在被必填挡住（`--title` 的"空 = 不设置"不再可能）。
- `check` 不加「动作缺 title」规则：历史 303 个动作里 302 有 title，唯一例外是 `f8413d8`（`help-relation-flags` 的 hypothesize）；加规则会多一条 warn，留触发条件——若出现第二条再议。
- title 含换行仍会破坏 subject/body 结构（`Format` 原样拼接）——另一类写侧加固，不在本批。
