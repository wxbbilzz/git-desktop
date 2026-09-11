package engine

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// 覆盖行级暂存：只把文件里的某几行放进暂存区，其余留在工作区。
func TestStageLines(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%v 失败: %s", args, out)
		}
		return string(out)
	}

	git("init", "-q", "--initial-branch=main")
	if err := os.WriteFile(dir+"/f.txt", []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "init")

	// 制造两处改动：改 line2、加 line4
	if err := os.WriteFile(dir+"/f.txt", []byte("line1\nline2fixed\nline3\nline4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(dir); err != nil {
		t.Fatal(err)
	}

	patch, err := e.FilePatchLines("f.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if !patch.HasChanges {
		t.Fatal("应该检测到变更行")
	}
	t.Logf("结构化 patch 共 %d 行", len(patch.Lines))

	// 只选第一处改动（+line2fixed）
	var picked []int
	for _, l := range patch.Lines {
		if l.Selectable && l.Text == "line2fixed" {
			picked = append(picked, l.Index)
			break
		}
	}
	if len(picked) == 0 {
		t.Fatal("没找到目标行")
	}
	t.Logf("选中行索引 %v", picked)

	if _, err := e.StageLines("f.txt", false, picked); err != nil {
		t.Fatalf("行级暂存失败: %v", err)
	}

	staged := git("diff", "--cached")
	work := git("diff")

	if !strings.Contains(staged, "+line2fixed") {
		t.Errorf("暂存区应包含选中的行:\n%s", staged)
	}
	if strings.Contains(staged, "+line4") {
		t.Errorf("暂存区不应包含未选中的行:\n%s", staged)
	}
	if !strings.Contains(work, "+line4") {
		t.Errorf("未选中的行应留在工作区:\n%s", work)
	}
	t.Log("✅ 只有选中的行进了暂存区，其余留在工作区")
}
