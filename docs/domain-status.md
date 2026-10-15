# domain 的 status 怎么显示

`status` 以 domain 为目录：**块头写一次域名，行内只写裸 issue ID**（缩进两格），块之间空一行。

```
cli
  help-alignment    ✓ apply
  status-by-domain    ask

design
  action-config     ✓ apply
  commit-format     ✓ apply
  problem-naming    ✓ apply
```

- **不画树、不写前缀**：一个 issue 只出现在**自己 domain** 的块里，行里的名字就是命令能吃的名字（`kiln path help-alignment`）。
- 跨域关系（父的 `spawns:` / 子的 `derived-from:`）**不再搬动行**——关系的渲染是 `kiln graph` / `kiln path` 的活；`status` 只管目录。
- 列宽按 **issue 行**（不含块头）算，跨块统一对齐。
