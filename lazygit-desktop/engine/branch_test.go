package engine

import (
	"os"
	"os/exec"
	"testing"
)

// 回归测试：CreateBranch 曾经用 lazygit 的 Branch.New(name, "")，
// 而它内部用的是 Arg()（不跳过空串），会生成 `git checkout -b 名字 ""`，
// git 报 "empty string is not a valid pathspec" —— 也就是这个函数从来就是坏的。
func TestCreateBranchFrom(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v 失败: %s", args, out)
		}
	}
	git("init", "-q", "--initial-branch=main")
	if err := os.WriteFile(dir+"/a.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "c1")

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(dir); err != nil {
		t.Fatal(err)
	}

	// 起点留空 = 以当前 HEAD 为起点，并且要切过去
	snap, err := e.CreateBranchFrom("feature/from-head", "", true)
	if err != nil {
		t.Fatalf("起点留空时创建失败（就是当年那个空串 bug）: %v", err)
	}
	if snap.Branch != "feature/from-head" {
		t.Errorf("应当切换到新分支，实际停在 %q", snap.Branch)
	}

	// 从指定分支创建，不切换
	snap2, err := e.CreateBranchFrom("feature/from-main", "main", false)
	if err != nil {
		t.Fatalf("从 main 创建失败: %v", err)
	}
	if snap2.Branch != "feature/from-head" {
		t.Errorf("checkout=false 时不应切换分支，实际变成了 %q", snap2.Branch)
	}

	names := map[string]bool{}
	for _, b := range snap2.Branches {
		names[b.Name] = true
	}
	if !names["feature/from-head"] || !names["feature/from-main"] {
		t.Errorf("分支列表不对: %v", names)
	}

	// 非法名字要拦下来
	for _, bad := range []string{"", "bad name", "a~b", "a^b"} {
		if _, err := e.CreateBranchFrom(bad, "", true); err == nil {
			t.Errorf("非法分支名 %q 应当被拦住", bad)
		}
	}

	// 删除
	snap3, err := e.DeleteBranch("feature/from-main", false)
	if err != nil {
		t.Fatalf("删除分支失败: %v", err)
	}
	for _, b := range snap3.Branches {
		if b.Name == "feature/from-main" {
			t.Error("feature/from-main 应该已被删除")
		}
	}
}
