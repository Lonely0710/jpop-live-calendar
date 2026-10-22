# Live Calendar · 日本音乐 Live 情报日历

<p align="center">
  <a href="#更新记录"><img src="https://img.shields.io/badge/version-1.0.1-E85A3A?style=flat-square&labelColor=2E2A28" alt="版本 1.0.1"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-2E2A28?style=flat-square" alt="许可证 MIT"></a>
  <a href="https://github.com/egoist/mygo"><img src="https://img.shields.io/badge/MyGo-v0.5.5-3D86F5?style=flat-square&labelColor=2E2A28&logo=go&logoColor=white" alt="MyGo v0.5.5"></a>
</p>

<p align="center">
  <b>简体中文</b> · <a href="README.ja.md">日本語</a> · <a href="README.en.md">English</a>
</p>

<p align="center">
  <img src="resources/icon.png" width="112" alt="Live Calendar 应用图标">
</p>

macOS / Windows 原生桌面应用（**MyGo Native UI + Go**），把日本音乐的 Live、抽选 / 先行受付、结果公布、一般发售等**公开信息**整理成日历。无数据库、无登录。

![月视图（真实数据）](assets/screenshots/month-live.png)

## 功能

- **四种视图**：月 / 周 / 日 / 列表；⌘← / ⌘→ 翻页，⌘T 回到今天。Live 是带色条的实心条，受付开始 / 截止、结果公布、一般发售是带圆点的描边胶囊，截止加粗，延期 / 中止标红。
- **一眼看清票务**：同一场演出的各轮受付、结果公布与一般发售独立成节点，各自出现在日历上，各自有倒计时与购票 / 受付链接。
- **每一条都可追溯**：每个场次和票务节点都能回到来源公告，看到发布时间、抓取时间、第几版与解析用文本，并可打开原文。
- **只抓公开信息**：默认关注 21 位艺人，启用 34 个官方 / 公开源（官网、RSS、Sony Music 公开 JSON）；不登录、不读会员页、不抓票务平台，遵守 robots.txt，同一站点请求串行且间隔 ≥ 1 秒。
- **缺失字段不猜测**：没公布的时间显示「时间未公布」，没公布的场地显示「场地未公布」，不确定的推断标「需确认」；延期、中止、截止时间变化写入变更记录，不静默覆盖。
- **本地快照 + 后台同步**：公开公告与头像分别缓存在 `cache.json` / `avatars.json`，重启先显示上次结果；单个数据源失败只记录到它自己，不影响其他源，也不会清空已有数据。
- **常驻与提醒**：关闭窗口只是隐藏，菜单栏（macOS）/ 系统托盘（Windows）继续同步；抽选截止前 3 天提醒一次（`notified.json`），截止延长后按新时间再提醒。
- **只读、无账号**：不做登录、收藏、个人购票状态或云同步；侧边栏底部的账号入口只是占位。

| 详情与票务节点 | 数据源 |
| --- | --- |
| ![详情面板：日期、开场 / 开演、场馆、票务节点与变更记录](assets/screenshots/inspector.png) | ![数据源：按艺人分组，可单独同步、停用或删除](assets/screenshots/sources.png) |

## 快速开始

### 环境要求

| 项目 | 版本 |
| --- | --- |
| macOS | 13+（Apple Silicon / Intel 均可） |
| Windows | 10 / 11 x64（可以在 macOS / Linux 上交叉编译） |
| Go | **1.27 或更高**（MyGo v0.5.5 要求 go 1.27） |
| Xcode Command Line Tools | 在 macOS 上打包 `.app` / `.dmg` 时需要：`xcode-select --install` |
| NSIS（可选） | 在 Windows 上生成安装包时需要 `makensis`；没有时仍会生成便携 zip |

```bash
brew install go        # 或从 https://go.dev/dl/ 安装
go version             # 需显示 go1.27.x 或更高
```

### 运行

```bash
go mod download        # 首次：下载 MyGo 等依赖
go tool mygo dev       # 编译并打开原生窗口，保存代码后自动重启
go run .               # 或直接运行
```

### 测试

```bash
go test ./...
go vet ./... && go tool mygo vet .
```

### 打包

`scripts/package.sh` 会从 `mygo.json` 读取版本号，调用 MyGo 构建，并把产物命名为 `jpop-live-calendar-{平台}-{版本}`：

```bash
scripts/package.sh windows/amd64   # → build/jpop-live-calendar-windows-amd64-1.0.1.zip
scripts/package.sh darwin/arm64    # → build/jpop-live-calendar-darwin-arm64-1.0.1.dmg（或 .zip）
scripts/package.sh darwin/universal
```

Windows 的 zip 解开后是完整目录，双击里面的 `Live Calendar.exe` 运行；不要只取出 exe，旁边的资源文件也要保留。装了 NSIS 时还会输出 `...-setup.exe`。也可以直接 `go tool mygo build -platform windows/amd64`，产物在 `build/windows-amd64/`（文件名是 MyGo 默认格式）。

## 快捷键

macOS 使用 ⌘，Windows 使用 Ctrl：

| 功能 | macOS | Windows |
| --- | --- | --- |
| 上一 / 下一个月（周、天） | ⌘← / ⌘→ | Ctrl+← / Ctrl+→ |
| 回到今天 | ⌘T | Ctrl+T |
| 月 / 周 / 日 / 列表视图 | ⌘1 – ⌘4 | Ctrl+1 – Ctrl+4 |
| 立即同步全部已启用的数据源 | ⌘R | Ctrl+R |
| 详情面板从事件返回当天列表 | Esc | Esc |
| 显示 / 隐藏左侧边栏 | ⌃⌘S | Ctrl+Shift+S |
| 显示 / 隐藏右侧详情面板 | ⌥⌘I | Ctrl+Alt+I |
| 清空搜索 | Esc（搜索框内） | Esc（搜索框内） |

## 数据源配置

读取顺序（找到即用）：环境变量 `LIVE_CALENDAR_CONFIG` → 用户配置目录（macOS 为 `~/Library/Application Support/Live Calendar/sources.yaml`，Windows 为 `%AppData%\Live Calendar\sources.yaml`）→ 当前目录 `config/sources.yaml` → 编译进 App 的默认副本。配置有误时 App 仍会用内置默认配置启动，并在「数据源」页显示错误。

```yaml
sources:
  - id: my-band-live            # 唯一 ID
    name: 某乐队官网 LIVE        # 显示名
    kind: website               # demo | rss | website | api（Sony Music 公开 JSON）
    layer: live                 # live = LIVE/SCHEDULE 日程层；news = NEWS/TICKET 公告层
    url: https://example.com/live/
    enabled: true
    interval: 12h               # 自动同步间隔，最小 10m；0 = 只手动同步
    artist: 某乐队              # 可选：页面一律归属该艺人（官网源）

artists:
  - name: ステラリウム
    aliases: [Stellarium]       # 不同写法会合并成一个艺人
    official_url: https://example.com/
    avatar_url: https://example.com/a.jpg  # 可选；填 none = 不用头像
    color: "#3975AB"            # 可选：指定主题色，优先于从头像取的颜色
    follow: true                # 默认 true：常驻「艺人」页；false = 只用于合并别名

avatars: deezer                 # deezer（默认）| off：不联网获取头像
```

- **头像与主题色**：通过 Deezer 公开 API 在后台查找，限速每秒 1 个请求；主题色取头像中面积最大的色块，文字对比度保证 ≥ 4.5:1。取不到时用默认插画 + 固定配色。
- **进一步过滤**：`article:` 段支持 `keywords` / `exclude` / `include` / `title_only` / `max_age` / `body_start` / `body_end` / `max_per_sync` / `delay`；`follow` 和 `feature` 用正则从列表页 / 公告中发现详情页与巡演专题页。写法见 `config/sources.yaml` 的注释。
- **默认覆盖**：米津玄師、Official髭男dism、Mrs. GREEN APPLE、RADWIMPS、back number、緑黄色社会、YOASOBI、優里、Aimer、なとり、milet、King Gnu、藤井風、星野源、Vaundy、Ado、ZUTOMAYO、ヨルシカ、あたらよ、『ユイカ』、美波。默认启用 34 个源，另有 5 个默认关闭的备用源（演示数据、两个厂牌 NEWS、Ado SPA 官网、『ユイカ』官网）。
- **界面内管理**：可在「数据源」页新增、停用、删除来源；写回配置文件时保留原有注释。新增只接受公开、允许访问的来源。
- **robots.txt**：遵守 RFC 9309，被禁止的路径不抓取；票务链接只从官方公告中取得，不盲爬票务平台。

## 目录结构

```
jpop-live-calendar/
├── main.go                 # 入口：窗口、菜单栏 / 系统托盘、后台同步
├── mygo.json               # 应用名、Bundle ID、版本、输出目录
├── resources/              # 应用图标、托盘图标、内嵌 Pacifico 字体
├── config/                 # sources.yaml + 加载 / 校验 / 写回
├── internal/
│   ├── domain/             # 公开数据模型（场次、票务轮次、公告、艺人、场馆）
│   ├── sources/ crawler/   # 数据源适配器 + 礼貌抓取器 / 每源隔离的调度器
│   ├── parser/ normalize/  # 日文公告解析、日期 / 时刻 / 都道府县 / 别名规范化
│   ├── dedup/ catalog/     # 去重合并、变更记录、线程安全的内存快照
│   ├── views/ theme/       # MyGo 界面与 Spotlight 主题、配色、尺寸
│   └── avatar/ cache/      # 艺人头像 / 主题色、本地 JSON 快照
├── packages/spotlight-icons/  # 图标库（主 module 内的普通包）
├── scripts/package.sh      # 按 jpop-live-calendar-{平台}-{版本} 打包
├── assets/screenshots/     # README 预览图
├── LICENSE                 # MIT 许可证
└── AGENTS.md               # 编码代理的工作说明（数据源 / 解析 / 界面各有子文件）
```

## 图标库

Spotlight Icons 是本项目**主 module 里的普通包**，直接 import，不需要额外下载或发布。63 个 24×24 SVG，6 个分组，线宽 1.75–2，配色严格限定在 Spotlight 三色中。

```go
import (
	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"
)

mygoicons.Image(c, icons.Calendar, 18) // 保留原色（Ink Charcoal + Live Coral）
mygoicons.Mono(c, icons.Calendar, 18)  // 单色，跟随文字颜色
```

<p align="center">
  <a href="assets/spotlight-icons-overview.png">
    <img src="assets/spotlight-icons-overview.png" width="820" alt="Spotlight Icons 总览：63 个图标、6 个分组">
  </a>
</p>

新增 / 替换图标、重新生成常量与测试的步骤见 [packages/spotlight-icons/README.md](packages/spotlight-icons/README.md)。

## 已知限制

- 解析基于规则，覆盖日文新闻稿的常见写法；没把握的内容一律标「需确认」，请以原文为准。
- 数据只保存在本地 JSON 快照：macOS 在 `~/Library/Application Support/Live Calendar/`，Windows 在 `%AppData%\Live Calendar\`；删除它们只会让下次启动重新同步 / 重新下载，不影响数据源配置。
- Windows 版已交叉编译通过，但快捷键、托盘、通知和标题栏按钮的实际显示仍建议在 Windows 10 / 11 上人工检查一次。
- 界面已通过截图验证；首次在 Mac 上运行时如有显示问题，欢迎反馈截图。

## 后续（Phase 5 之后）

- （可选）日历格子里显示中国 / 日本节假日（如「中·休 国庆节」「日·祝 スポーツの日」）。
- 票务轮次解析继续加强（如「●エントリー期間」下一行才写日期的写法）。
- 新增数据源表单支持 `article` 过滤与 `follow` 正则（现在这些仍可直接编辑 `sources.yaml`）。

## 更新记录

- **1.0.1**（2026-10-22）
  - Windows 10 / 11 x64 支持：系统托盘彩色图标、Ctrl 快捷键、`%AppData%` 数据目录，`scripts/package.sh` 统一命名 zip / 安装包。
  - 艺人页按巡演 / 月份分组，可折叠、按巡演筛选、隐藏已结束的巡演；已结束的场次和票务轮次淡出，列表与详情各自滚动。
  - 窄窗口时把统计收进工具栏，滚动条改为 4 DIP；修正公告列表的日期 +「NEW」徽标被误解析成场次。
  - 图标库增至 63 个（新增 `view-disabled`、`map-route`、`map-japan`）。
- **1.0.0** — Phase 5：菜单栏常驻、抽选截止提醒、界面内新增数据源、默认头像与头像缓存。
- **0.7.0** — 本地 JSON 快照缓存，重启不再从空日历开始。
- **0.6.0** — 数据源重做：日程层 / 公告层、Sony 公开 JSON、巡演专题页与解析修正。
- **0.5.0** — 改用 MyGo 原生控件，侧边栏可折叠为图标栏，数据源可在界面内删除。
- **0.4.0** — Liquid Glass 面板、可折叠侧栏、艺人头像与主题色。
- **Phase 1–3** — 日历界面、真实公开数据源、多公告合并与票务节点。
