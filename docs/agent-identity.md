# issue: agent-identity — 人 / agent 的身份怎么标，业界到底有没有成熟做法

- **domain**：design
- **关联**：relates-to `author-attribution`（写侧不加 trailer 的结论；本条的触发正是"多行动者时缺身份"）；relates-to `json-author`（读侧已把 git author 投影出来，缺的是"有区别的身份"）；relates-to `readme-collaboration`（README 把多人协作写成明示边界，依据就是本条要回答的问题）

## 背景

`json-author` 之后，`--json` 已经能读到每条动作的 git 作者。但今天所有提交共用同一个 `user.name`（人 / agent 糊在一起），字段是常量——**有观测点，没有可观测的差别**。

问题因此落到"身份本身"：

- **技术上不难**：`GIT_AUTHOR_NAME` / `GIT_AUTHOR_EMAIL`、`git commit --author`、`git -c user.name=…` 都是一行的事；kiln 真要支持，只需在 `commit` 里透传或加一个 flag。
- **难在"标了之后信不信"**：谁都能改 `user.name`；trailer 谁都能手打；签名能绑定身份，但把密钥管理摊给每个 agent；平台侧（GitHub / GitLab）另有自己的机器人账号与审计。
- **更难在"业界没有共识"**：同一件事至少有五六种表达，而且最大的开源项目正在往回退（见 collect）。

所以本条不问"能不能写"，只问"**该学哪一种、kiln 该落在哪一层**"。

## 要回答的问题

1. 业界有没有成熟做法？分几层：git 原生身份、trailer 约定、平台机器人账号、notes / 外部元数据、密码学签名？
2. 每种做法**证明了什么、证明了谁**？哪些可核验，哪些只是"谁都能打的一行字"？
3. git 的 author / committer 双角色够不够用？（人给方向、agent 提交）
4. kiln 该不该介入、在哪一层介入：`commit` 时设身份 / 读侧识别几种 trailer / 只做 `--json` 投影（现状）/ 什么都不做只写文档？
5. 单人 + agent 的最小可行是什么？成本与可逆性如何？

## 非目标

- 不做身份认证与密钥管理（那是签名与 IAM 的事）；
- 不定义"agent 的边界"（模型级 vs 会话级）；
- 不承诺能防伪；
- 不改 `commit` 的默认行为——任何身份相关改动都必须显式。

## collect：业界现状（2026-11-02，联网核实）

> 来源类型：**联网检索 + 原文抓取**。标 [一手] 的是抓到了原文；标 [检索] 的只拿到标题/摘要（站点拒绝抓取）。

### 1. 五层做法，五种"证明"

| 机制 | 谁在用 | 证明了什么 | 弱点 |
| --- | --- | --- | --- |
| **git 原生 author / committer** | git 本身 | 提交里有两个身份槽位；`--author`、`GIT_AUTHOR_*` 可任意改写 | 零门槛也零可信：谁都能填任何名字 |
| **`Assisted-by:` trailer** | Linux 内核 | 披露"用过 AI"；且内核明确 **AI agent 不得加 `Signed-off-by`**（DCO 只能人签）[一手] | 措辞是 "should" 不是 "must"；只是披露，不是身份 |
| **`Co-Authored-By:` trailer** | Claude Code 等 agent 工具 | "它也参与了" | IETF 草案直接点破：*unverifiable… any committer can paste any string* [一手] |
| **平台账号 / 机器人身份** | GitHub Copilot coding agent | **反过来**：agent 当 author、下指令的人当 co-author，再加 `Agent-Logs-Url` 指回会话日志 [一手] | 只在该平台内成立；本地 clone 里剩下的还是那行字 |
| **notes / 外部元数据** | git-ai（v1.0，2025-10） | `refs/notes/ai` 里存**行级**归属 + `agent_id{tool,id,model}` / human，schema `authorship/3.0.0` [一手] | notes **默认不随 clone/fetch 走**，要显式配置；记的是行不是提交 |
| **三层 trailer + 签名** | IETF 个人草案 `draft-morrison-identity-attributed-commits` | 把身份分 Sovereign / Bot / Instrument 三层（`Acted-By` / `Executed-By` / `Drafted-With`），Ed25519 签 tree hash，可跨 rebase [一手] | individual draft，没有任何实现与采用；依赖自定义 `~handle` 与 DNS |

### 2. 三个正在互相打架的信号

1. **内核在往回退**：文档里 `Assisted-by` 还在（"should include"），但 2026-10 的标题是"计划移除 Assisted-by 要求"[检索：Phoronix 等多处转述]。同一个项目里，"要披露"与"别标了"同时存在。
2. **Copilot 把身份反过来**：agent 当作者、人当 co-author；连"为什么改"都放进了 trailer（`Agent-Logs-Url`）。
3. **Cursor 把标注做成产品开关**：`Made with Cursor` trailer 默认开、个人与 enterprise 两级都能关 [一手]。**默认值能被关掉，说明还没有共识。**

### 3. 结论（collect 能确定的部分）

- **没有成熟做法**。没有一种机制同时满足：可核验 + 跨 clone 带走 + 零额外基础设施 + 不依赖平台。已有的每条都只覆盖其中两三项。
- **唯一稳定的共识是"责任在人"**：DCO / `Signed-off-by` 只能由人签（内核把这条写成 MUST NOT 给 AI）。也就是说：**作者可以是 agent，责任必须是人**——这两件事在 git 里本来就分属 author 与 committer 两个槽位。
- **kiln 不该新造身份机制**。四个候选落点：① 只写文档、教人配 git 身份（零代码）；② `commit` 透传 `GIT_AUTHOR_*`——**git 本来就认，已经天然支持**；③ 读侧识别 `Co-authored-by` / `Assisted-by` 并投影进 `--json`（只读，最轻）；④ notes 方案——**与"`git clone` 即带走"直接冲突**，不推荐。

### 4. 留给 hypothesize 的判断

身份问题的成本不在"写"，在"信"与"跨 clone"。真实需求出现前（第二个人 / 第二个独立 agent），最省的做法是 **①+③**：文档教怎么配身份，读侧顺手认得出 `Co-authored-by` / `Assisted-by`——两者都不动写侧，也不承诺防伪。

### 5. 来源

[一手] 抓到原文：

- Linux 内核 AI Coding Assistants 文档（`Assisted-by` 与 "AI MUST NOT add Signed-off-by"）：<https://docs.kernel.org/process/coding-assistants.html>
- GitHub Copilot coding agent 提交署名与 `Agent-Logs-Url`：<https://github.blog/changelog/2026-03-20-trace-any-copilot-coding-agent-commit-to-its-session-logs/>
- Cursor 的 `Made with Cursor` trailer 与 Attribution 开关：<https://cursor.com/help/integrations/git>
- git-ai 的 `refs/notes/ai` 归属格式（v1.4，schema `authorship/3.0.0`）：<https://github.com/git-ai-project/git-ai/blob/8a6184a3a8fc4e997ccfff3b16d79688e4255af4/docs/migrations/sessions-v2-note-format.md>
- IETF 草案（三层 trailer、tree-hash 签名、对 `Co-Authored-By` 的评价）：<https://datatracker.ietf.org/doc/draft-morrison-identity-attributed-commits/01/>

[检索] 只拿到标题/摘要：

- 内核计划移除 `Assisted-by` 要求（Phoronix 原文被 403，多家转述）：<https://www.phoronix.com/news/Linux-AI-Attribution-Again>
- Aider 用 `GIT_AUTHOR_NAME` 把身份改成 `"User Name (aider)"`、以及 `Co-Authored-By` 相关讨论（二手研究笔记与 issue）：<https://github.com/Aider-AI/aider/issues/3270>

## 记录

- 2026-11-02：由人工提出——作者字段有了，但身份本身不好解决，先核实业界现状，再决定 kiln 要不要碰。
