package engine

import (
	"os"
	"os/exec"
	"testing"
)

// 覆盖「没有提交身份 -> 界面引导 -> 配置身份 -> 提交成功」这条完整链路。
//
// 这是用户的真实场景：刚装好 git 的用户没有 user.name / user.email，
// 直接点提交会得到一句英文 fatal。软件必须能主动发现并引导。
func TestIdentityFlow(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		// 隔离全局配置，确保从「没有身份」开始
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v 失败: %s", args, out)
		}
	}
	run("init", "-q", "--initial-branch=main")
	if err := os.WriteFile(dir+"/f.txt", []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := e.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}

	// 1) 快照必须如实反映「没有身份」，界面据此显示引导
	if snap.IdentityName != "" || snap.IdentityEmail != "" {
		t.Fatalf("期望没有身份，实际 %q / %q", snap.IdentityName, snap.IdentityEmail)
	}
	t.Log("✅ 快照正确报告「未配置身份」")

	// 2) 没配身份就提交，必须给出可操作的中文提示（而不是 git 的英文 fatal）
	if _, err := e.Commit("测试", ""); err == nil {
		t.Fatal("没配身份时提交应当失败")
	} else {
		t.Logf("✅ 未配身份时提交被拦下: %v", err)
	}

	// 3) 配置身份（仓库级，避免影响真实全局配置）
	snap2, err := e.SetIdentity("张三", "zhangsan@example.com", false)
	if err != nil {
		t.Fatalf("SetIdentity 失败: %v", err)
	}
	if snap2.IdentityName != "张三" || snap2.IdentityEmail != "zhangsan@example.com" {
		t.Fatalf("配置后快照未更新: %q / %q", snap2.IdentityName, snap2.IdentityEmail)
	}
	t.Log("✅ 配置身份后快照立即更新")

	// 4) 现在提交应当成功
	if _, err := e.Commit("配置身份后的第一次提交", ""); err != nil {
		t.Fatalf("配置身份后提交仍失败: %v", err)
	}
	out, _ := exec.Command("git", "-C", dir, "log", "--oneline", "-1").Output()
	t.Logf("✅ 提交成功: %s", out)

	// 5) 校验输入有效性
	if _, err := e.SetIdentity("", "a@b.com", false); err == nil {
		t.Error("空名字应当报错")
	}
	if _, err := e.SetIdentity("张三", "not-an-email", false); err == nil {
		t.Error("非法邮箱应当报错")
	}
	t.Log("✅ 参数校验生效")
}
