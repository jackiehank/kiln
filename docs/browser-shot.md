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

