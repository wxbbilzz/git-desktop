# Lazygit Desktop

按 **方案 B：复用核心，重写 UI** 实现的 lazygit 桌面客户端。

> **本机已构建完成。** 终端里 `cd` 到任意 git 仓库后运行 `lazygit-desktop` 即可。
> 详见下方「二、运行方式」。

一句话概括：**git 逻辑继续用 lazygit 的，界面换成带圆角边框和胶囊按钮的桌面 App。**

---

## 一、架构

```
┌──────────────────────────────────────────────────────┐
│  UI 层   React + TypeScript                          │
│  在 workspaces 里就是：                               │
│    TopBar / Sidebar / DiffPanel / CommitPanel /       │
│    HistoryPanel                                       │
└───────────────────────┬──────────────────────────────┘
        意图(intent) ↓        ↑ 快照(snapshot)
                 Wails 自动生成的绑定
┌───────────────────────┴──────────────────────────────┐
│  App 层  (app.go)  很薄，只做转发 + 调系统目录选择框    │
└───────────────────────┬──────────────────────────────┘
┌───────────────────────┴──────────────────────────────┐
│  Engine 层  (engine/)  无界面，可独立测试               │
│    · 状态快照  · 动作执行  · diff 提取                  │
└───────────────────────┬──────────────────────────────┘
│  lazygit 核心（直接 import，未改动一行）                │
│    pkg/commands            git 命令封装                │
│    pkg/commands/models     commit/branch/file 模型     │
│    pkg/commands/patch      patch 解析                  │
│    pkg/config              用户配置（含你的现有配置）    │
│    pkg/i18n                文案                        │
└──────────────────────────────────────────────────────┘
```

要点：

- **完全没有引入 lazygit 的 `pkg/gui` 和 `pkg/gocui`**。原来的 TUI 视图 / 上下文 / 控制器一层都不用。
- 界面上每一次操作都是「UI 发意图 → 引擎执行 git → 返回新快照 → UI 重绘」。
  前端不推算 git 状态，永远只渲染引擎给的快照，所以 UI 可以随意重写。
- **直接复用你已有的 lazygit 配置**：`pkg/config` 读取的仍是 `~/.config/lazygit/config.yml`
  （`NewAppConfig` 的 `name` 参数不参与配置路径），所以你的 `git.mainBranches`、
  日志排序、diff renderer 等设置都会生效。

### 目录

```
lazygit-desktop/
├── main.go                   Wails 入口（窗口、嵌入前端产物）
├── app.go                    绑定给前端的薄转发层
├── engine/
│   ├── engine.go             引擎：初始化 lazygit 核心、读快照
│   ├── dto.go                DTO 与模型转换
│   └── actions.go            动作：暂存/提交/分支/同步/diff
├── frontend/
│   ├── src/App.tsx           界面编排
│   ├── src/api.ts            前端唯一通道 + 浏览器 mock 回退
│   ├── src/mock.ts           演示数据（不编译也能预览 UI）
│   ├── src/styles.css        圆角边框 + 胶囊按钮的样式系统
│   └── src/components/       面板组件
├── wails.json
└── go.mod                    ← replace 指向本地 lazygit 源码
```

---

## 二、运行方式（终端）

本机依赖已装齐、项目也已编译完成，直接在终端启动即可。

### 基本用法

```bash
cd /path/to/你的仓库
lazygit-desktop
```

`lazygit-desktop` 已链接到 `/usr/local/bin`，任何目录都能直接调用。
它会自动打开**当前所在目录**对应的 git 仓库。

> 如果当前目录不在任何 git 仓库里，程序会打开，并显示「还没有打开仓库」
> 的空状态，点右上角「打开仓库」选一个目录即可。

### 直接运行二进制

```bash
/home/wxbbi/Desktop/lazygit-desktop/build/bin/lazygit-desktop
```

效果和上面一样，只是不依赖 `PATH` 里的软链接。

### 建议加个别名

如果想让启动更顺手，可以在 `~/.bashrc` 里加：

```bash
alias lg='lazygit-desktop'
```

之后进入任意仓库敲 `lg` 就能打开。

### 本机已安装的环境

| 组件 | 版本 | 位置 |
|---|---|---|
| Go | go1.27.1 | `/usr/local/go`，已链接到 `/usr/local/bin/go` |
| Wails CLI | v2.15.0 | `/home/wxbbi/go/bin/wails`，已链接到 `/usr/local/bin/wails` |
| libwebkit2gtk-4.1-dev | 2.52.5 | apt 安装 |
| libgtk-3-dev | 3.24.41 | apt 安装 |
| Node / npm | v20.15.1 / 9.2.0 | 系统自带 |

另外：Go 模块代理已设为 `https://goproxy.cn`，npm 源已设为
`https://registry.npmmirror.com`，后续拉依赖会自动走国内镜像。

> 注意：本机只有 webkit2gtk **4.1**，所以 `wails.json` 里加了
> `"build:tags": "webkit2_41"`。少了这个标签 Wails 会去找 4.0 而编译失败。

### 改完代码后重新构建

```bash
cd /home/wxbbi/Desktop/lazygit-desktop
wails build     # 重新打包，产物覆盖 build/bin/lazygit-desktop
```

只改界面时的快速方式（热更新，不用重编 Go）：

```bash
wails dev
```

或纯浏览器预览界面（用内置演示数据，不碰真实仓库）：

```bash
cd /home/wxbbi/Desktop/lazygit-desktop/frontend
npm run dev     # 打开 http://localhost:5173
```

### 引擎自检工具

界面显示不对时，先用它判断是「引擎没读到数据」还是「界面没画出来」：

```bash
cd /home/wxbbi/Desktop/lazygit-desktop
go run ./cmd/dump /path/to/repo    # 直接打印引擎读到的 JSON 快照
```

## 三、只想改界面？不用装 Go

界面和引擎是解耦的，`frontend/src/api.ts` 在检测不到 Wails 环境时会自动
回退到 `mock.ts` 的演示数据。所以调样式时：

```bash
cd /home/wxbbi/Desktop/lazygit-desktop/frontend
npm install
npm run dev
```

浏览器打开 http://localhost:5173 ，就能看到完整界面（圆角面板、胶囊按钮、
diff 着色），并且暂存 / 取消暂存 / 提交这些操作在 mock 数据上也是可交互的。
顶栏会有提示告诉你当前是演示模式。

---

## 四、关于 lazygit 的路径

本项目通过 `go.mod` 的 `replace` 直接引用本地 lazygit 源码：

```
replace github.com/jesseduffield/lazygit => /home/wxbbi/Downloads/lazygit-master/lazygit-master
```

如果你**移动了 lazygit 的位置**，改这一行即可。注意源码压缩包解出来是嵌套的
同名目录，真正含 `go.mod` 的是里层那个。

---

## 五、启动界面：三种开始方式

没有打开任何仓库时，会显示启动界面，提供三个入口（圆角卡片）：

| 入口 | 做什么 |
|---|---|
| **新建仓库** | 选存放位置 + 填仓库名 + 指定初始分支名，执行 `git init` 并打开 |
| **打开仓库** | 弹出系统目录选择框，打开已有仓库 |
| **下载仓库** | 填仓库地址 + 选存放位置 + 文件夹名，执行 `git clone` 并打开 |

几个细节：

- 「下载仓库」支持 **GitHub / Gitee / GitLab / 自建服务**，以及
  `git@host:user/repo.git` 形式的 SSH 地址。
- 目录名会根据地址**自动推断**（`https://gitee.com/user/repo.git` → `repo`），
  你也可以手动改。
- 提供 **浅克隆**开关（`--depth=1`），大仓库能快很多。
- 克隆过程中会实时显示进度条，阶段名（接收对象 / 解析差异…）是
  从 git 的 stderr 解析出来的。
- 主界面顶栏有个「🏠」按钮可以随时**回到启动界面**换仓库。

### 关于 GitHub 的网络说明

**这台机器目前访问不了 github.com**（实测连接被拒），所以从 GitHub 克隆会失败。
gitee.com 是通的（实测完整克隆 176MB 的仓库约 20 秒）。

如果之后要用 GitHub，需要先解决网络可达性（代理等），
然后在 `~/.gitconfig` 里配置，或给 git 设 `http.proxy`：

```bash
git config --global http.proxy http://127.0.0.1:端口
```

引擎本身对 GitHub 没有任何特殊处理，能连通就能克隆。

## 六、Git 全命令操作面板

git 有 **159 个顶级命令**。给每个命令手写一套界面是不现实的，
所以这里用的是「**命令目录 + 通用执行器**」的数据驱动方案。

点顶栏的绿色 **「Git 操作」** 按钮打开面板：

```
┌─ Git 操作 ────────────────────────────────────────┐
│ 搜索: 合并____________                             │
│ ┌─分类─┐ ┌─操作────────┐ ┌─详情─────────────────┐│
│ │全部  │ │合并分支       │ │合并分支               ││
│ │提交  │ │中止合并       │ │把指定分支合并进当前分支││
│ │分支  │ │变基到分支     │ │[参数表单]             ││
│ │远端  │ │…              │ │[执行]                 ││
│ └──────┘ └───────────────┘ └──────────────────────┘│
└────────────────────────────────────────────────────┘
```

### 收录范围：110 个操作，覆盖 15 个类别

| 类别 | 数量 | 类别 | 数量 |
|---|---|---|---|
| 提交 | 7 | 撤销 | 7 |
| 分支 | 12 | 工作区 | 4 |
| 远端 | 12 | 子模块 | 6 |
| 标签 | 5 | 补丁 | 5 |
| 储藏 | 8 | 查询 | 10 |
| 配置 | 6 | 维护 | 8 |
| 导出 | 4 | 高级 | 16 |

涵盖：提交/修补/回退/拣选、分支增删改合变基、远端增删改与推送拉取、
标签、储藏、撤销恢复 reflog、worktree、submodule、补丁、日志/blame/grep、
仓库维护 gc/fsck、config、archive/bundle、notes/bisect/sparse-checkout/subtree 等。

### 没收录的命令怎么办：原始命令入口

搜索框输入任意内容，列表最后有一项 **「执行任意 git 命令」**，
可以直接执行任何 git 子命令（包括底层 plumbing 命令），支持引号：

```
cat-file -p HEAD:README.md
rev-list --count HEAD
ls-tree -r HEAD --name-only
```

这是「覆盖所有命令」的兜底：**159 个命令全都能用**，
只是常用的有专门表单，冷门的走这个入口。

### 设计要点

1. **加一个新命令 = 加一条数据**。`engine/catalog.go` 里描述每个操作的
   命令、参数、是否危险；前端自动渲染出对应表单，不用写任何 UI 代码。
2. **危险操作有标记**。硬重置、强制推送、清理未跟踪文件这类不可逆操作
   会标红并在执行前二次确认。
3. **参数用 argv 数组传递，不经过 shell**，所以参数里的特殊字符不会被
   解释，没有命令注入风险。
4. **执行结果原样展示**：界面上会显示实际执行的 `git ...` 命令和完整输出，
   方便学习 git、也方便复制到终端。
5. **命令目录有测试保护**：`engine/catalog_test.go` 会把目录里每个命令名
   和 `git --list-cmds` 比对，写错命令名会直接测试失败。

### 已经用专属界面的高频操作

提交、暂存/取消暂存、丢弃改动、切换分支、Fetch/Pull/Push 仍然是主界面上的
专属按钮（更好用），它们和操作面板走的是同一套引擎逻辑。

### 首次使用需要配置提交身份

git 在提交时会记录「是谁提交的」。如果你还没有配置过
`user.name` / `user.email`，**提交会失败**（git 的原始报错是一句英文
`fatal: unable to auto-detect email address`）。

软件会主动检测这种情况：**提交面板会直接变成身份配置表单**，
填上名字和邮箱点「保存身份」即可（写入全局配置，之后所有仓库都不用再填）。
配置完成后面板自动变回正常的提交界面。

也可以用 Git 操作面板里的「配置 → 写入全局配置」手动设置，
或者在终端执行：

```bash
git config --global user.name "你的名字"
git config --global user.email "你的邮箱"
```

## 七、按文件查看提交

在历史里点某个提交，中间面板会变成**左边文件列表 + 右边该文件的改动**，
而不是把整个提交的所有改动堆在一个界面里：

```
┌─ 提交 abc1234  [3 个文件] ──────────────────────────┐
│ ┌─ 文件列表 ──────┐ ┌─ 改动 ──────────────────────┐ │
│ │ ● readme.md  修改│ │ @@ -1,3 +1,5 @@             │ │
│ │ ● config.go  新增│ │ -旧内容                     │ │
│ │ ● util.go    修改│ │ +新内容                     │ │
│ └─────────────────┘ └─────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

- 文件列表显示每个文件的状态（新增/修改/删除/重命名）和 `+N -M` 行数
- 点文件名切换右侧的改动，默认打开第一个文件
- 用的是 `git show <hash> -- <path>`，所以每个文件只显示它自己的 diff

## 八、一键上传到 GitHub / Gitee

顶栏点 **「上传」** 按钮，可以把这个本地仓库直接发布到代码托管平台：

1. 选平台（GitHub / Gitee）
2. 填访问令牌（token）
3. 填仓库名、描述，选择公开还是私有
4. 点「创建并上传」

软件会自动完成：**调用平台 API 创建远端仓库 → 添加 origin → 推送当前分支并设置上游**。

### 关于访问令牌

创建仓库是平台 API 的写操作，必须鉴权，所以要你自己生成一个 token：

| 平台 | 生成地址 | 需要的权限 |
|---|---|---|
| Gitee | https://gitee.com/personal_access_tokens | `projects` |
| GitHub | https://github.com/settings/tokens | `repo` |

令牌只用于创建仓库和推送，不会上传到其他地方。

「记住凭据」勾选后，令牌会写进**本仓库的 `.git/config`**（这个文件不会被提交、
也不会推送出去），这样以后在软件里点 `Push` 也能直接用。不勾选则只用于这一次推送，
推送完远端地址会自动恢复成不含令牌的干净形式。

### ⚠️ 本机网络情况

**这台机器目前访问不了 github.com**（实测连接被拒），所以选 GitHub 会提示连接失败。
Gitee 是通的，可以正常使用。

要支持 GitHub 需要先解决网络可达性，然后给 git 配置代理：

```bash
git config --global http.proxy http://127.0.0.1:端口
```

## 九、已经实现的功能

- 启动界面：新建仓库 / 打开本地仓库 / 从网址下载仓库（含进度显示）
- **Git 全命令操作面板**：110 个操作 + 原始命令入口，搜索/分类/动态表单/危险确认
- 每次操作都会显示实际执行的 git 命令和完整输出
- **提交身份引导**：未配置 user.name/user.email 时，提交面板直接变成配置表单
- **按文件查看提交**：点历史里的提交，左列文件列表 + 右侧单文件改动
- **一键上传到 GitHub / Gitee**：自动创建远端仓库并推送
- 文件改动列表，按「工作区 / 暂存区」分组，带状态徽标与 `+N -M` 行数统计
- 暂存 / 取消暂存单个文件、全部暂存、全部取消暂存
- 丢弃改动（已跟踪走 `git checkout --`，未跟踪直接删文件，带二次确认）
- Diff 查看：整段统一 diff 解析成带新旧行号、按增删着色的视图
- 提交：标题 + 描述，支持 `Ctrl/⌘ + Enter`
- 提交历史列表，点选查看该提交的 diff
- 分支列表、切换分支
- Fetch / Pull / Push
- 打开 / 切换仓库（系统目录选择框）

## 十、已知限制

1. **凭据提示走不通**。引擎用的是 `NewNullGuiIO`，克隆/推送时如果远端要账号密码，
   会直接失败而不是弹框询问（克隆时设了 `GIT_TERMINAL_PROMPT=0`，避免卡死）。
   公开仓库、SSH key、已配置的 credential helper 都不受影响。要支持交互式输入，
   得把凭据回调接到前端弹窗。
2. **`gocui.Task` 这处残留耦合**。`git_commands` 的 `Push/Fetch/Pull` 需要一个
   `gocui.Task`，而它的接口带未导出方法、外部无法自己实现，所以临时借用了
   `gocui.NewFakeTask()`。要彻底去掉，把那几个方法的参数换成自定义窄接口即可。
3. **还没有的能力**：行级/块级暂存、交互式 rebase、cherry-pick、stash、worktree、
   merge 冲突处理、提交图（当前是列表不是图）。这些正是原版 lazygit 的强项，
   需要继续从 `controllers/helpers` 里把动作层抽到引擎里。
4. **已验证**：本机已用 Go 1.27.1 + Wails 2.15.0 完整编译通过，并实测运行——
   文件列表、状态徽标、diff 增删着色、点击选中都已确认正常渲染。

## 十一、下一步建议

按价值排序：

1. 先跑通 `wails dev`，确认引擎能真实驱动一次 status → stage → commit。
2. 补行级暂存（lazygit 的 `pkg/commands/patch` 已经提供了 patch 构建能力）。
3. 把 diff 视图升级成双栏 / 支持折叠。
4. 抽引擎时顺手去掉 `gocui.Task` 依赖。
5. 交互式 rebase 做成拖拽排序，会比原版的 TODO 文件编辑更好用。
