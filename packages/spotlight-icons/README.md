# spotlight-icons

Live Calendar 的 Spotlight 图标库：58 个 24×24 SVG，线宽 1.75–2、圆头，配色严格限制在 Spotlight 色板内（测试会检查）。

| 分组 | 图标 |
| --- | --- |
| actions | add, remove, edit, delete, view, flag, more, help, clear, open, account |
| sources | rss, link, api, data-status, sync, synced, sync-failed, update-log, parse-error, crawl-error |
| details | time, show-time, venue, artist, category, official-info, price, info, source-site |
| status | live, preorder-start, preorder-end, result, general-sale, announcement, postponed, cancelled, suspended |
| navigation | previous, next, today, add-to-calendar, view-switch, download, share |
| views | home, calendar, month, week, day, list, search, filter, sort, settings, sidebar-left, sidebar-right |

**一图一名**：每个图标只有一个名字、一种含义（测试会检查没有两个相同的图形）。0.5.0 合并了重复的图标：

| 旧名字 | 现在用 |
| --- | --- |
| official-site、related-link、copy-link（同一个链条） | `link`（链接 / 官网页面）；复制链接改用 `view-switch` |
| open-externally | `open` |
| refresh | `sync` |
| update | `info`（变更记录用 `update-log`） |
| duplicate | `view-switch` |
| date | `calendar` |
| agenda | 改名为 `list` |

新增 `show-time`（开场 / 开演：门 + 时钟）；`day` 换为日程页（文档 + 右下角珊瑚色圆点，v0.6.0）。

## 结构

```
spotlight-icons/
├── svg/*.svg            # 图标源文件（go:embed 打包进二进制）
├── icons.json           # 名称 → 分组
├── icons.go             # Lookup / SVG / MustSVG / FS / Names / InGroup / Palette
├── names_gen.go         # 自动生成的常量：icons.Calendar、icons.Link …
├── internal/gennames/   # 生成器（go generate）
└── mygoicons/           # MyGo 适配层：Get / Image / Mono
```

根包不依赖任何 UI 库，只提供 SVG 字节；`mygoicons` 子包负责转换成 MyGo 的 `*ui.SVG`（解析一次后缓存）。

## 添加 / 替换图标

```bash
cp ~/Downloads/new-icon.svg svg/new-icon.svg   # 必须是 viewBox="0 0 24 24"，不要用 <mask>
# 在 icons.json 加一行 {"name": "new-icon", "group": "actions"}
go generate ./...                              # 重新生成 names_gen.go → icons.NewIcon
go test ./...                                  # 检查 viewBox、mask、色板、能否被 MyGo 解析
```

如果你有原版导出的 SVG，直接整体覆盖 `svg/` 后执行上面两条命令即可。

## 发布成独立的 Go 包（可选）

图标库现在是 Live Calendar 主 module 里的普通包（import 路径 `jpop-live-calendar/packages/spotlight-icons`），不需要发布。只有想给其他项目复用时才需要：

1. 新建 GitHub 仓库（例如 `github.com/<你的用户名>/spotlight-icons`），把本目录内容复制进去，执行 `go mod init github.com/<你的用户名>/spotlight-icons`，并把 `mygoicons` 中对图标包的 import 改成新路径。
2. 提交、推送并打标签：`git tag v0.1.0 && git push origin v0.1.0`。
3. 其他项目里：`go get github.com/<你的用户名>/spotlight-icons@v0.1.0`。私有仓库需设置 `GOPRIVATE`。
