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

// 回归测试：git show 对合并提交默认不输出差异（combined diff），
// 之前点历史里的合并提交会看到「0 个文件」。
func TestCommitFilesOnMergeCommit(t *testing.T) {
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
	write := func(name, content string) {
		if err := os.WriteFile(dir+"/"+name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q", "--initial-branch=main")
	write("base.txt", "b")
	git("add", "-A")
	git("commit", "-qm", "base")

	git("checkout", "-qb", "feat")
	write("feat.txt", "f")
	git("add", "-A")
	git("commit", "-qm", "feat")

	git("checkout", "-q", "main")
	write("main.txt", "m")
	git("add", "-A")
	git("commit", "-qm", "main work")

	git("merge", "-q", "--no-ff", "feat", "-m", "merge feat")

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
	if len(files) == 0 {
		t.Fatal("合并提交应该能列出「带进来了哪些文件」，实际为空")
	}

	// 合并带进来的是 feat.txt
	found := false
	for _, f := range files {
		if f.Path == "feat.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("合并提交的文件列表里应该有 feat.txt，实际 %+v", files)
	}

	// 单文件 diff 也要能取到
	if _, err := e.CommitFileDiff("HEAD", "feat.txt"); err != nil {
		t.Errorf("合并提交的单文件 diff 失败: %v", err)
	}
}

// 回归测试：文件已从工作区删除、但还在索引里时，
// 文件树会列出它，点开必须仍然能看到内容（从索引读）。
func TestFileContentFallsBackToIndex(t *testing.T) {
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
	if err := os.WriteFile(dir+"/gone.txt", []byte("重要内容\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "init")
	if err := os.Remove(dir + "/gone.txt"); err != nil {
		t.Fatal(err)
	}

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(dir); err != nil {
		t.Fatal(err)
	}

	c, err := e.FileContent("gone.txt")
	if err != nil {
		t.Fatalf("已删除的文件应当能从索引读到内容，实际报错: %v", err)
	}
	if !c.FromIndex {
		t.Error("应当标记内容来自索引")
	}
	if !strings.Contains(c.Content, "重要内容") {
		t.Errorf("内容不对: %q", c.Content)
	}
}
