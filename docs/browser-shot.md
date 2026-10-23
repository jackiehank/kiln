# issue: browser-shot — 把「浏览器截图」做成可复用的基础能力

- **domain**：tooling
- **关联**：relates-to `graph-svg`（截图能力先解决它的"看图"问题）

## 背景

2026-10-23 给 `graph-svg` 做 collect 时发现：本环境里 Edge 无头模式完全起不来（`exit 13` / `0x80000003`），于是当时绕道"自绘 SVG + ImageMagick"。但这只是一次绕路，不是结论——**浏览器截图本身是调试任何 Web 项目的基础能力**，值得单独做成稳定入口，而不是每次临时找替代方案。

用户提供的线索：这台机器上 Trae 跑通过 Playwright；实在不行还有 Docker / WSL / qemu 里的浏览器。

## 问题

1. 沙箱内 Edge 起不来的根因是什么（安装问题？还是 DSH 文件沙箱）？
2. 沙箱外哪条路最省：系统 Edge 无头、Playwright、Docker、WSL、qemu？
3. 怎么给后续会话一个稳定、可复用的入口（而不是每次重写一段 CDP 脚本）？

## 验收（草案）

- **E1**：任意 `http(s)://` URL 或本地 HTML 文件 → PNG，退出码 0，并打印尺寸/标题。
- **E2**：支持整页截图、元素级截图（`--clip`）、等待渲染完成（`--wait-for`）。
- **E3**：`.mmd` 文件 → PNG（供 `kiln graph --mermaid` 自查），输出按图自然尺寸而不是视口宽度。
- **E4**：失败可诊断：沙箱内运行时给出明确原因，而不是静默 `exit 13`。
- **E5**：不给 `kiln` 产品模块引入任何运行时依赖（工具放 `tools/`，浏览器复用系统 Edge）。

## 非目标

- 不做成 `kiln` 子命令；
- 不下载/内置 Chromium（Playwright 浏览器缓存约 150MB，且本机网络受限）；
- 不引入 Docker/qemu 中间层（除非上面的路全断）。

## 采集（2026-10-23）

| 假设 | 结果 |
| --- | --- |
| 沙箱内 Edge 无头可用 | ❌ `--headless=new/old` 全部 `exit 13`（`0x80000003` STATUS_BREAKPOINT）；非无头 `about:blank` 4 秒内退出；`--version` 报 `Lock file can not be created! Error code: 5` + `Failed to create a ProcessSingleton for your profile directory`（默认 profile 在 workspace 外，沙箱禁止写） |
| 是 Edge 安装坏了 | ❌ 不是：同一台机器 `%APPDATA%\npm\node_modules\playwright-core` 1.63.0 在，系统 Edge 155.0.4283.62 在 |
| 沙箱外 Edge 无头可用 | ✅ `danger-full-access` 下 `--headless=new --disable-gpu --no-sandbox --screenshot=... about:blank` 写出 2727B PNG，exit 0 ⇒ **根因是 DSH 沙箱，不是浏览器** |
| 沙箱外 Docker 可用 | ✅ `docker version` → server 29.8.2；`wsl -l -v` → `docker-desktop (Stopped)`；但沙箱内两者都被拒（`permission denied ... npipe:////./pipe/dockerDesktopLinuxEngine`、`WSL_E_ACCESS_DENIED`） |
| Playwright 能用系统 Edge | ✅ `playwright-core` + `channel: 'msedge'` 直接驱动 Edge，**不需要** `ms-playwright` 浏览器缓存（本机也确实没有） |
| 能渲染本地 mermaid 预览页 | ✅ `file://` + 本地 `mermaid.min.js`，等 `svg` 出现后截图；`.mmd` 直出 PNG 4287×1961（图自然尺寸） |
| qemu 可用 | ❌ 未安装，且要镜像下载（网络受限） |

**采集结论**：路径 = **Playwright-core + 系统 Edge（headless）**。唯一约束是：Chromium 在 DSH `workspace-write` 沙箱内必然崩溃，所以 agent 运行截图脚本需要 `danger-full-access`；用户自己的终端没有沙箱，直接跑即可。Docker / WSL / qemu 不必引入——它们解决的是同一个沙箱限制，而提权就够了。

## apply（2026-10-23）

工具落在 `tools/browser-shot/`（不进 `cmd/kiln`，无产品依赖）：

- `shot.mjs`：通用截图。`node shot.mjs <url|file> [--out p.png] [--width 1440] [--height 900] [--wait-for sel|text:foo] [--clip sel] [--full-page] [--settle ms] [--channel msedge] [--headful]`；输出一行 JSON（`ok/out/title/url/size`）。
- `mermaid-shot.mjs`：`.mmd` → PNG。`node mermaid-shot.mjs g.mmd --out g.png [--theme default|dark|neutral] [--scale 2]`；按 `viewBox` 钉住 SVG 尺寸再截，避免 mermaid 默认把图缩到容器宽度。
- `README.md`：用法与沙箱约束。

两处实现细节：`playwright-core` 依次从 `require` 链路、`%APPDATA%\npm\node_modules`、`%ProgramFiles%\nodejs\node_modules` 找；mermaid bundle 依次从 `$KILN_MERMAID_JS`、`.kiln/mermaid.min.js`、`%LOCALAPPDATA%\Programs` 下的浏览器包找。

## 结论（已定）

复核（2026-10-23，沙箱外）：

| 项 | 结果 |
| --- | --- |
| `shot.mjs` 截本地 HTML | ✅ `{"ok":true,...}`，截到 `svg` 元素，exit 0 |
| `mermaid-shot.mjs` 直出 | ✅ `graph-preview.mmd` → 4287×1961 PNG（自然尺寸，非视口宽） |
| 真仓渲染 | ✅ 48 节点 / 62 条边全部出图；有向箭头、对称虚线、中文标签正确 |
| `kiln check` | ✅ `ok=true`，0 错误（50 issue / 38 完成 / 12 进行中） |

边界：agent 侧必须 `danger-full-access`（沙箱内 Chromium 必崩，原因已定位到 profile 锁/`ProcessSingleton`）；Windows 上 `msedge --version` 会挂起，读版本用 `(Get-Item).VersionInfo`；只出 PNG。

与 `graph-svg` 的分工：**产品出口仍是 `--svg`**（零依赖、用户直接开），浏览器截图用于 agent 自查与调试任何 Web 项目；两者不互相替代。


