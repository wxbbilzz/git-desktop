package engine

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// 仓库文件监听。
//
// 以前界面只在「用户自己点了什么」之后才刷新，于是在编辑器里改完文件切回来，
// 列表还是旧的 —— 看起来像卡住了。这里监听仓库目录，有变化就回调出去，
// 由 app 层转成事件推给前端，前端重新拉一次快照。
//
// 两个关键点：
//
//  1. **监听点选在「会变的东西」上**：`.git` 下的 index / HEAD / refs / logs
//     覆盖了暂存、提交、切换分支、fetch、merge、rebase —— 这些都是 git 层面
//     的状态变化；工作区则是编辑器改文件。
//  2. **必须 debounce**。一次 git 操作会连写好几个文件（index、index.lock、
//     refs、logs），不合并的话界面会被连续刷新十几次。
const (
	// 事件在这段时间内合并成一次刷新
	watchDebounce = 400 * time.Millisecond
	// 最多监听的目录数。仓库里挂个 node_modules 就是几万个目录，
	// 全加进去会吃光 inotify 配额（而且毫无意义）。
	watchMaxDirs = 2000
)

// watchSkipDirs 是明确不需要监听的目录名。
//
// 都是「内容量大、且变化不影响 git 状态」的：依赖目录、构建产物、缓存。
var watchSkipDirs = map[string]bool{
	"node_modules": true, ".venv": true, "venv": true, "env": true,
	"__pycache__": true, ".mypy_cache": true, ".pytest_cache": true, ".ruff_cache": true,
	"dist": true, "build": true, "target": true, "vendor": true,
	".next": true, ".nuxt": true, ".cache": true, ".parcel-cache": true,
	"coverage": true, ".tox": true, "Pods": true, ".gradle": true,
	".idea": true, ".vscode": true,
}

// shouldSkipDir 判断一个目录要不要跳过。
//
// gitDir 为 true 表示这个目录位于 .git 里面：那里只关心 HEAD / index /
// refs / logs 这几个位置，不需要往下钻 —— 尤其是 .git/objects，每次提交和
// fetch 都往里写，量大且没有监听价值（refs 和 index 的变化已经能说明问题）。
func shouldSkipDir(name string, gitDir bool) bool {
	if gitDir {
		switch name {
		case "refs", "logs":
			return false
		}
		return true
	}
	// .git 由 addTree 单独按 gitDir=true 的规则加，这里不要跟着工作区走一遍
	if name == ".git" {
		return true
	}
	return watchSkipDirs[name]
}

// repoWatcher 是对 fsnotify 的一层封装：递归添加监听 + 合并事件。
type repoWatcher struct {
	watcher  *fsnotify.Watcher
	repoPath string
	onChange func()

	mu     sync.Mutex
	timer  *time.Timer
	closed bool
	done   chan struct{}
}

// startRepoWatcher 开始监听仓库，返回的 watcher 需要调用 Stop 释放。
//
// onChange 会在「仓库有变化」时被调用，已经过 debounce 合并。
// 它跑在自己的 goroutine 里，所以回调里只应该做很轻的事（发个事件），
// 绝不要回调进引擎方法 —— 那会撞上引擎锁。
func startRepoWatcher(repoPath string, onChange func()) (*repoWatcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &repoWatcher{
		watcher:  fsw,
		repoPath: repoPath,
		onChange: onChange,
		done:     make(chan struct{}),
	}

	// 工作区：跳过依赖 / 构建目录
	w.addTree(repoPath, false)
	// .git：只加少数几个关键位置
	w.addTree(filepath.Join(repoPath, ".git"), true)

	go w.loop()
	return w, nil
}

// addTree 递归把目录加进监听列表。
func (w *repoWatcher) addTree(root string, gitDir bool) {
	added := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// 单个目录读不了（权限等）不该让整件事失败
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root {
			if shouldSkipDir(d.Name(), gitDir) {
				return filepath.SkipDir
			}
		}
		if added >= watchMaxDirs {
			return filepath.SkipDir
		}
		if err := w.watcher.Add(path); err == nil {
			added++
		}
		return nil
	})
}

func (w *repoWatcher) loop() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// 只关心内容变化；Chmod 是权限位变动，不反映 git 状态
			if ev.Op == fsnotify.Chmod {
				continue
			}
			// 新目录要补上监听，否则在里面改文件不会被发现
			if ev.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					inGit := strings.HasPrefix(ev.Name, filepath.Join(w.repoPath, ".git"))
					if !shouldSkipDir(filepath.Base(ev.Name), inGit) {
						w.addTree(ev.Name, inGit)
					}
				}
			}
			w.schedule()
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// 监听出错不外抛：自动刷新只是锦上添花，不能影响正常使用
		}
	}
}

// schedule 把短时间内的多次变化合并成一次回调。
func (w *repoWatcher) schedule() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(watchDebounce, func() {
		w.mu.Lock()
		closed := w.closed
		w.mu.Unlock()
		if closed {
			return
		}
		if w.onChange != nil {
			w.onChange()
		}
	})
}

// Stop 停止监听并释放资源。可以重复调用。
func (w *repoWatcher) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	if w.timer != nil {
		w.timer.Stop()
	}
	w.mu.Unlock()

	close(w.done)
	_ = w.watcher.Close()
}
