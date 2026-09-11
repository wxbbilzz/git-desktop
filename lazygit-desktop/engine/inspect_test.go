package engine

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// 打开一个文件夹时，要先判断它是什么情况：
//  1. 本身就是仓库          -> 直接打开
//  2. 是某个仓库的子目录    -> 打开上级仓库
//  3. 完全不受 git 管理     -> 让界面问「要不要在这里建仓库」
func TestInspectFolder(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	root := t.TempDir()
	plain := root + "/普通文件夹"
	os.MkdirAll(plain, 0o755)
	os.WriteFile(plain+"/a.txt", []byte("x"), 0o644)
	os.WriteFile(plain+"/b.txt", []byte("y"), 0o644)

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}

	// ---- 情况 3：普通文件夹 ----
	info, err := e.InspectFolder(plain)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsRepo {
		t.Error("普通文件夹不该被判定为仓库")
	}
	if info.ParentRepo != "" {
		t.Errorf("不该找到上级仓库，实际 %q", info.ParentRepo)
	}
	if info.FileCount != 2 {
		t.Errorf("应数出 2 个条目，实际 %d", info.FileCount)
	}
	t.Logf("✅ 普通文件夹：isRepo=false, 有 %d 个条目", info.FileCount)

	// ---- 在这一步初始化 ----
	snap, err := e.InitRepoAt(plain, "main")
	if err != nil {
		t.Fatalf("在已有文件夹里 git init 失败: %v", err)
	}
	if snap.Branch != "main" {
		t.Errorf("初始分支应为 main，实际 %q", snap.Branch)
	}
	// 原有文件必须还在（不能因为 init 把人家东西弄没了）
	if _, err := os.Stat(plain + "/a.txt"); err != nil {
		t.Error("git init 之后原文件不该消失")
	}
	t.Logf("✅ 初始化成功，原有文件保留，分支=%s", snap.Branch)

	// ---- 情况 1：现在它自己就是仓库了 ----
	info, _ = e.InspectFolder(plain)
	if !info.IsRepo {
		t.Error("初始化后应当被判定为仓库")
	}
	t.Log("✅ 初始化后再检查：isRepo=true")

	// ---- 情况 2：子目录 ----
	sub := plain + "/src/deep"
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	info, _ = e.InspectFolder(sub)
	if info.IsRepo {
		t.Error("子目录本身不是仓库根")
	}
	if info.ParentRepo == "" {
		t.Error("应该能找到上级仓库")
	} else {
		// 路径可能带 /private 之类的前缀，比较结尾即可
		if !strings.HasSuffix(info.ParentRepo, "普通文件夹") {
			t.Errorf("上级仓库路径不对: %s", info.ParentRepo)
		}
		t.Logf("✅ 子目录：找到上级仓库 %s", info.ParentRepo)
	}

	// ---- 重复 init 不应该报错（幂等）----
	if _, err := e.InitRepoAt(plain, "main"); err != nil {
		t.Errorf("对已是仓库的目录再 init 不该失败: %v", err)
	}
	t.Log("✅ 重复初始化是幂等的")
}

// InspectFolder 对不存在的路径要报错，而不是返回一个空结果
func TestInspectFolderInvalid(t *testing.T) {
	e, _ := New()
	if _, err := e.InspectFolder("/definitely/not/here/xyz"); err == nil {
		t.Error("不存在的路径应当报错")
	}
	// 传文件而不是目录也要报错
	f := t.TempDir() + "/f.txt"
	os.WriteFile(f, []byte("x"), 0o644)
	if _, err := e.InspectFolder(f); err == nil {
		t.Error("传文件应当报错")
	}
}

var _ = exec.Command
