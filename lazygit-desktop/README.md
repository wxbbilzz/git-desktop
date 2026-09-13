<img src="assets/bingit-icon.svg" width="72" align="left" hspace="14" vspace="4">

# Bingit · 开发说明

产品介绍、功能清单、截图和构建步骤在[仓库根目录的 README](../README.md)。
这里只写给要改这个项目的人看：引擎怎么工作、代码在哪、怎么加东西。

<br clear="left">

---

## 一、引擎怎么工作

整个设计只有一条规则：**界面不推算 git 状态，永远只渲染引擎给的快照。**

```
UI(任意技术栈) --意图--> Engine --调用--> lazygit pkg/commands / git 子进程
UI(任意技术栈) <--快照-- Engine
```

所以引擎的每个对外方法都长这样：**加锁 → 校验 → 执行 git → 重新读一份完整快照返回**。
前端拿到新快照直接替换本地状态，不需要知道刚才发生了什么。

`RepoSnapshot` 里带着界面要渲染的一切：当前分支、工作区/暂存区文件、提交列表、
本地分支、标签、远端、远端分支，以及「能否撤销上一步」和撤销会撤掉什么。

### 加锁规则

- 绝大多数方法从头到尾持锁（`e.mu`）。
- **网络操作不持锁**（`syncOp`）：只在校验参数和最后刷新快照时加锁，中间几十秒的
  传输是无锁的。否则推送时界面所有请求都会被阻塞，看起来像死机。
- 文件监听的检查在**不持锁的 goroutine** 里跑，回调只做「发个事件」这种极轻的事，
  绝不能回调进引擎方法（会撞上引擎锁）。

### 自动刷新

`watch.go` 用 fsnotify 监听仓库：`.git` 下的关键位置（index / HEAD / refs / logs）
覆盖暂存、提交、切换分支、fetch 这些 git 层面的变化，工作区则覆盖编辑器改文件。
事件经过 debounce 合并后回调出去，`app.go` 把它转成 `repo:changed` 事件推给前端。

跳过 `node_modules`、`dist` 这类目录，并有目录数上限（全加进去会吃光 inotify 配额）。
监听必须在 `OpenRepo` 之前用 `SetRepoChangeHandler` 注册才会启动。

### 失败处理

- `retry.go` 判断一次失败是**瞬时**还是**确定性**的。瞬时（连接重置、TLS 握手中途
  被掐、5xx、429）自动重试并退避；确定性（non-fast-forward、认证失败、仓库重名）
  立刻结束。匹配的模式**中英文都有** —— git 的报错跟随系统语言。
- `netdiag.go` 只在失败路径上跑，主动探测「域名是否被 /etc/hosts 指到 127.0.0.1」
  「环境里配的代理是否连得上」，把「连接被拒绝」翻译成一句能照着做的话。

---

## 二、引擎文件一览

`engine/` 不依赖任何界面代码，可以单独 `go test`。

| 文件 | 管什么 |
|---|---|
| `engine.go` | 初始化 lazygit 核心、读快照、挂载文件监听 |
| `dto.go` | 发往前端的结构体与模型转换 |
| `actions.go` | 暂存 / 提交 / 分支 / 同步 / diff 这些基础动作 |
| `gitops.go` | 一等公民操作：合并、变基、拣选、revert、reset、修补提交、继续/中止、撤销、标签与远端的增删改 |
| `refs.go` | 读取标签、远端、远端分支（lazygit 的 BranchLoader 只给本地分支） |
| `catalog.go` | **git 命令目录：数据即功能**，110 个操作都在这 |
| `runop.go` | 操作目录的通用执行器 + 操作面板用的查询 |
| `retry.go` | 重试策略与瞬时/永久失败分类 |
| `netdiag.go` | 上传失败的原因诊断 |
| `publish.go` | 一键上传到 GitHub / Gitee |
| `restore.go` | 丢弃文件的回收站（用 git 对象备份内容） |
| `watch.go` | 仓库文件监听（自动刷新） |
| `repo.go` | 仓库建立：init / clone / 文件夹检查 |
| `staging.go` | 行级暂存 |
| `commit_files.go` | 按文件查看某次提交 |
| `repofiles.go` | 仓库文件树与文件内容 |
| `stash.go` | 储藏 |
| `identity.go` | 提交身份的读写 |
| `conflicts.go` | 冲突文件按块解析与解决（**后端已实现，界面尚未接入**） |

---

## 三、前端结构

`frontend/src/`。设计上刻意做了一件事：**`api.ts` 在检测不到 Wails 环境时
自动回退到 `mock.ts` 的演示数据**，所以调样式完全不需要编译 Go。

| 文件 | 管什么 |
|---|---|
| `api.ts` | 与引擎之间的唯一通道 + mock 回退 + 事件订阅 |
| `mock.ts` | 演示数据 |
| `App.tsx` | 界面编排：持有状态、调 api、把结果分发给各面板 |
| `types.ts` | 与 `engine/dto.go` 一一对应的类型 |
| `commitGraph.ts` / `fileTree.ts` | 提交泳道图、目录树（单链压缩） |
| `sound.ts` | Web Audio 实时合成音效，不含音频文件 |
| `components/` | 各面板组件 |

`types.ts` 里的字段名必须和 `engine/dto.go` 的 JSON tag 对得上 ——
对不上就是面板渲染不出来（空白），这是这类项目最常见的一类 bug。

---

## 四、怎么加东西

### 加一个 git 操作（推荐先看这条）

如果只是「把某个 git 子命令做成表单」，**不需要写任何界面代码**：
往 `engine/catalog.go` 里加一条数据即可，前端会自动渲染出表单、下拉和危险标记。

```go
{
    ID: "tag.create", Category: "标签", Name: "创建标签",
    Description: "在指定提交上创建标签",
    Base:        []string{"tag"},
    Params: []Param{
        s("name", "标签名", true, "v1.0.0"),
        ref("ref", "目标提交", SourceCommit, false),
    },
}
```

参数构造函数有四个：`s`（文本）、`sf`（带 flag 的文本）、`ref`（从仓库数据里选）、
`bo`（布尔开关）。写错的命令名会被 `catalog_test.go` 挡下来（它拿
`git --list-cmds` 比对）。

### 加一个一等公民动作

如果一个动作值得专属界面（像合并、变基那样），要走四层：

1. `engine/` 里加方法，返回 `(*RepoSnapshot, error)`
2. `app.go` 里加一行转发（Wails 会自动生成 TS 绑定）
3. `frontend/src/api.ts` 里加一个通道方法（记得写 mock 回退）
4. 在组件里调用它；危险操作请走 `ConfirmDialog`，并把**即将执行的命令**显示出来

---

## 五、测试

```bash
cd lazygit-desktop
go test ./engine/...      # 50 个测试
go vet ./...
```

引擎测试都是**真的在临时目录里建仓库、跑真实的 git 命令**，不是打桩。
新增能力时请照这个路子写：用 `t.TempDir()` 建仓库，用 `GIT_CONFIG_GLOBAL=/dev/null`
隔离用户配置，需要提交身份时写进仓库的本地 config（`GIT_AUTHOR_*` 环境变量只够
git 自己造提交用，引擎在提交前还会读 git config 检查身份）。

几个容易忘的点：

- 并发相关的代码用 `go test -race` 过一遍（文件监听那部分尤其）。
- 断言 git 报错文案时**中英文都要顾及**：CI 和开发者机器的 git 语言可能不同。
- 涉及到「写进 .git/config 的东西」的测试，记得验证失败路径不会留下带 token 的配置。

前端：

```bash
cd frontend
npm run build             # tsc --noEmit && vite build
```

---

## 六、打包

```bash
sudo bash packaging/build-deb.sh
sudo dpkg -i build/deb/org.bingit.app_<版本>_amd64.deb
```

脚本自己会先 `wails build`，然后按《UOS 应用打包规范》组织目录结构。
版本号在脚本头部，改版本时记得和 `wails.json`、`engine.New()` 里传的版本保持一致。

---

## 七、注意事项

1. **`replace` 指向本地 lazygit 源码**。`go.mod` 最后一行把
   `github.com/jesseduffield/lazygit` 指向了一个**开发机上的绝对路径** ——
   换机器、换人就必须改这一行，否则编译不过。源码压缩包解出来是嵌套的同名目录，
   要指向里层那个。
2. **webkit2gtk 版本**。`wails.json` 带 `"build:tags": "webkit2_41"`。
   系统装的是 4.0 的话要去掉这个标签，否则编译失败。
3. **一次只能打开一个仓库**，且 `OpenRepo` 会 `os.Chdir`（lazygit 的命令层假设
   进程工作目录就在仓库里）。要做多标签页，得先把对进程 cwd 的依赖去掉。
4. **`gocui.Task` 是唯一的残留耦合**。`git_commands` 的 `Push/Fetch/Pull` 需要一个
   `gocui.Task`，它的接口带未导出方法、外部包无法实现，所以临时借用
   `gocui.NewFakeTask()`。要彻底解耦，把那几个方法的参数换成自定义窄接口。
5. **界面还没有冲突解决**。`conflicts.go` 后端是全的（按块解析、逐块选择、写回文件），
   缺的是组件。要接的话记得同时补上测试 —— 那份代码目前没有任何测试覆盖。
