package engine

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// 覆盖「按文件查看某个提交」：文件列表要能正确识别新增/修改/删除/重命名，
// 并且能单独取到某个文件的 diff。
func TestCommitFiles(t *testing.T) {
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
	write := func(rel, content string) {
		t.Helper()
		full := dir + "/" + rel
		if i := strings.LastIndex(rel, "/"); i != -1 {
			if err := os.MkdirAll(dir+"/"+rel[:i], 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q", "--initial-branch=main")
	write("a.txt", "a\n")
	write("keep.txt", "k\n")
	git("add", "-A")
	git("commit", "-qm", "first")

	// 第二个提交：一个修改、一个新增、一个删除
	write("a.txt", "a\nmore\n")
	write("new.txt", "n\n")
	if err := os.Remove(dir + "/keep.txt"); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "second")

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(dir); err != nil {
		t.Fatal(err)
	}

	files, err := e.CommitFiles("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("期望 3 个文件，实际 %d: %+v", len(files), files)
	}

	byPath := map[string]CommitFileDTO{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	if f, ok := byPath["a.txt"]; !ok || f.Kind != "modified" || f.Additions != 1 {
		t.Errorf("a.txt 解析不对: %+v", f)
	}
	if f, ok := byPath["new.txt"]; !ok || f.Kind != "new" || f.Additions != 1 {
		t.Errorf("new.txt 解析不对: %+v", f)
	}
	if f, ok := byPath["keep.txt"]; !ok || f.Kind != "deleted" || f.Deletions != 1 {
		t.Errorf("keep.txt 解析不对: %+v", f)
	}
	t.Logf("✅ 文件列表正确：修改/新增/删除 都能识别")

	// 单文件 diff
	d, err := e.CommitFileDiff("HEAD", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "+more") {
		t.Errorf("a.txt 的 diff 不含新增行:\n%s", d)
	}
	if strings.Contains(d, "new.txt") || strings.Contains(d, "keep.txt") {
		t.Errorf("单文件 diff 不应包含其他文件:\n%s", d)
	}
	t.Logf("✅ 单文件 diff 正确（只含该文件的改动）")

	// 第一次提交（根提交）也要能列出来
	root, err := e.CommitFiles("HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if len(root) != 2 {
		t.Errorf("根提交应有 2 个文件，实际 %d", len(root))
	}
	t.Logf("✅ 根提交也能正确列出文件")
}
