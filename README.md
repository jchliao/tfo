# ⚡ TFO - The Flash Note（快闪记事本）

极简、本地优先的碎片化记事工具。快捷键一键呼出，随手记录灵感；微信消息直达笔记；所有数据以 Markdown 文件存储在本地，无数据库，无云端依赖。



## ✨ 核心优势

- **本地优先，数据自主** — 所有笔记存储在你自己的电脑上，隐私完全可控
- **一键快闪记事** — 全局快捷键呼出窗口，粘贴即保存，捕捉稍纵即逝的想法
- **随手贴图** — 截图直接粘贴或拖进输入框，图片和笔记一起存在本地数据目录，离线可用
- **微信碎片收集** — 给微信 Bot 发文字、语音或图片，自动转为笔记，消息仅经过微信服务器
- **纯 Markdown 文件** — 无数据库，笔记即文件；迁移/同步只需复制文件夹
- **索引可重建** — 搜索索引基于 Markdown 文件构建，随时可从源文件重建，零锁定
- **全文搜索** — 基于 Bleve 的高性能本地全文检索
- **跨平台** — 支持 macOS / Windows / Linux，提供桌面应用与 CLI 两种形态


## 📸 界面预览

### 主界面

<img src="docs/screenshots/main.png" width="800" alt="主界面" />

### 快闪记事

<img src="docs/screenshots/quick-capture.png" width="480" alt="快闪记事" />

### 微信 Bot

<img src="docs/screenshots/wechat-bot.png" width="480" alt="微信 Bot" />


## 🚀 快速开始

### 方式一：下载预编译版本（推荐）

从 [Releases](https://github.com/libi/tfo/releases) 下载对应平台的二进制，解压即用。

**MacOS**：打开 `TFOApp.app`，应用常驻菜单栏  
**Windows**：运行 `tfo-desktop.exe`，应用常驻系统托盘  
**Linux/Cli**：运行 `tfo`，浏览器打开 `http://localhost:8080`

### 方式二：从源码构建

#### 前置条件

- Go 1.25+
- Node.js 18+
- macOS 桌面版额外需要 Xcode（用于 Swift 编译）

#### 开发模式

```bash
# 1. 启动前端开发服务器
cd frontend && npm install && npm run dev

# 2. 另开终端，启动后端
go run ./cmd/tfo
```

前端默认运行在 `http://localhost:3000`，后端 API 在 `http://localhost:8080`。

#### 生产构建

```bash
# 构建 CLI（全平台）
./scripts/build.sh cli

# 构建桌面应用（macOS）
./scripts/build.sh desktop --os darwin

# 构建桌面应用（Windows）
./scripts/build.sh desktop --os windows

# 全量构建
./scripts/build.sh all
```

构建产物输出到 `dist/` 目录。

### 使用流程

1. **启动应用** — 双击桌面应用或终端运行 CLI
2. **快闪记事** — 按 `Alt+S`（可自定义）呼出快捷输入窗口，输入内容后回车保存
3. **插入图片** — 在输入框里直接 `Ctrl+V` 粘贴截图，或把图片文件拖进输入框
4. **浏览笔记** — 在 Web 界面查看、搜索、编辑所有笔记
5. **微信收集**（可选）— 在设置中配置微信 Bot，之后发给 Bot 的文字、语音、图片都会自动保存为笔记
6. **数据迁移** — 复制数据文件夹到新设备即可，搜索索引会自动重建

## 🖼️ 图片支持

笔记正文支持三种方式插入图片，插入的是标准 Markdown 图片语法：

- 在输入框（或编辑态）里直接 **粘贴截图**（`Ctrl+V` / `⌘V`）
- 把图片 **拖进输入框**
- 点击输入框右下角的 **图片按钮**，从文件系统选择（支持多选）

上传后的文件保存在数据目录的 `attachments/<年-月>/` 下，按内容哈希命名（同一张图只会存一份），
正文中的引用写成相对路径，例如：

```
2026-10/20261026-211500.md          ← 笔记文件
attachments/2026-10/1f2e3d4c5b6a7890.png
```

```markdown
今天的界面草图

![](../attachments/2026-10/1f2e3d4c5b6a7890.png)
```

因为用的是相对路径，所以：

- 整个数据目录拷到别的机器，图片依然能显示；
- 用 Typora / Obsidian / VS Code 直接打开 `.md` 文件也能看到图片；
- 笔记里的图片只是普通文件，可以随时备份、同步、删除。

限制：单张图片最大 10 MB，支持 PNG / JPEG / GIF / WebP / BMP（按文件头识别，改后缀无效）。
列表页会为含图片的笔记展示缩略图，点击可看大图。

### 微信 Bot 发来的图片

直接给 Bot 发图片即可（也可以图片配文字）。Bot 会从微信 CDN 下载密文、用消息里的
AES 密钥解密，再存进同一个 `attachments/<年-月>/` 目录，正文按「文字在前、图片在后」
的顺序自动插入图片语法。只发图片不配文字也会生成笔记。单条消息最多处理 9 张图片，
单张同样是 10 MB 上限；下载失败的那张会被跳过，不影响同一条消息里的其他内容。

> 删除笔记时不会连带删除图片文件（同一张图可能被多条笔记引用），
> 需要时可以手动清理数据目录下未被引用的 `attachments/` 文件。

> 注意：桌面端托盘里的迷你快捷输入窗（Windows）目前只支持纯文本，图片请在主界面输入框中插入。

## ⚙️ 配置

配置文件位于数据目录下的 `.config.json`：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `dataDir` | 数据目录路径 | - |
| `hotkeyGlobalQuickCapture` | 全局唤起快捷键（桌面端） | `Alt+Shift+F` |
| `hotkeyInputFocus` | 聚焦输入框快捷键（网页端） | `Alt+S` |
| `wechat.enabled` | 启用微信 Bot | `false` |
| `wechat.baseUrl` | 微信 Bot 服务地址 | - |
| `indexRebuildOnStart` | 启动时重建搜索索引 | `false` |



## 🏗️ 架构

```
┌─────────────┐     ┌──────────────┐
│  Desktop App│     │   CLI (tfo)  │
│ (系统托盘)   │     │  (终端运行)   │
└──────┬──────┘     └──────┬───────┘
       │                   │
       └───────┬───────────┘
               ▼
      ┌─────────────────┐
      │   HTTP Server    │  ← Gin
      │   (API + SPA)   │
      └────────┬────────┘
               │
  ┌────────────┼────────────┐
  ▼            ▼            ▼
┌──────┐  ┌────────┐  ┌─────────┐
│ Note │  │ Search │  │ Channel │
│Service│  │(Bleve) │  │(WeChat) │
└──┬───┘  └────────┘  └─────────┘
   ▼
┌──────────┐
│ Markdown │  ← 数据文件夹
│  Files   │
└──────────┘
```

**后端**：Go — Gin HTTP 框架 / Bleve 全文搜索 / fsnotify 文件监听  
**前端**：Next.js + React + Tailwind CSS，构建后嵌入二进制  
**桌面**：系统托盘 + 全局快捷键（macOS 使用 Swift 原生壳）

## 📁 项目结构

```
cmd/
  tfo/         — CLI 入口
  desktop/     — 桌面应用入口（系统托盘 + 全局快捷键）
    macos/     — macOS 原生 Swift 包装
frontend/      — Next.js 前端（构建产物嵌入 Go 二进制）
internal/
  app/         — 应用生命周期管理
  attachment/  — 图片附件存储（uploads / 内容寻址）
  note/        — 笔记 CRUD、Markdown 解析
  search/      — Bleve 全文索引与搜索
  channel/     — 消息通道（微信 Bot 适配）
  config/      — 配置管理
  server/      — Gin HTTP API
  watcher/     — 文件变更监听，自动更新索引
pkg/mdutil/    — Markdown 工具函数
scripts/       — 构建脚本
```

## 📄 License

MIT
