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

## 非目标

- 不改 `parse`、不改任何命令的行为（本动作只收事实）。
- 不重开 `silent-flag-drop` 的「未知 flag 只警告、不改退出码」裁决；不讨论 `--toon` 等输出格式。
