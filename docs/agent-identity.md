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

## 记录

- 2026-11-02：由人工提出——作者字段有了，但身份本身不好解决，先核实业界现状，再决定 kiln 要不要碰。
