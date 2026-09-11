package engine

import (
	"os/exec"
	"strings"
	"testing"
)

// 校验命令目录的完整性。
//
// 最有价值的一条是「命令名必须是真实存在的 git 子命令」——
// 目录是手写的数据表，写错一个命令名单靠肉眼很难发现，
// 这里直接拿 git 自带的命令列表来比对。
func TestCatalogIntegrity(t *testing.T) {
	ops := Catalog()
	if len(ops) == 0 {
		t.Fatal("目录为空")
	}

	// 拉取 git 支持的全部子命令作为白名单
	out, err := exec.Command("git", "--list-cmds=main,others,alias,nohelpers").Output()
	if err != nil {
		t.Skipf("无法获取 git 命令列表: %v", err)
	}
	valid := map[string]bool{}
	for _, name := range strings.Fields(string(out)) {
		valid[name] = true
	}

	seen := map[string]bool{}
	for _, op := range ops {
		if op.ID == "" {
			t.Error("存在没有 id 的操作")
		}
		if seen[op.ID] {
			t.Errorf("id 重复: %s", op.ID)
		}
		seen[op.ID] = true

		if len(op.Base) == 0 {
			t.Errorf("%s: Base 为空", op.ID)
			continue
		}
		if !valid[op.Base[0]] {
			t.Errorf("%s: git 子命令不存在 -> %q", op.ID, op.Base[0])
		}
		if op.Name == "" || op.Category == "" {
			t.Errorf("%s: 缺少名称或分类", op.ID)
		}
	}

	t.Logf("目录校验通过：%d 个操作", len(ops))
}

// 参数拼装规则：bool 追加标志、带 Flag 的值拼成 ["-m", 值]、
// 必填为空报错、__paths 统一放到 -- 之后。
func TestBuildArgs(t *testing.T) {
	op := Operation{
		Base: []string{"push"},
		Params: []Param{
			{Name: "remote", Kind: KindString},
			{Name: "up", Label: "设为上游", Kind: KindBool, Flag: "--set-upstream"},
			{Name: "msg", Label: "信息", Kind: KindString, Flag: "-m"},
			{Name: "need", Label: "必填", Kind: KindString, Required: true},
		},
	}

	args, err := op.BuildArgs(map[string]string{
		"remote":  "origin",
		"up":      "true",
		"msg":     "hello world",
		"need":    "x",
		"__paths": "a.txt b.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(args, " ")
	want := "push origin --set-upstream -m hello world x -- a.txt b.txt"
	if got != want {
		t.Errorf("拼装结果:\n  得到 %q\n  期望 %q", got, want)
	}

	// 必填缺失必须报错
	if _, err := op.BuildArgs(map[string]string{"remote": "origin"}); err == nil {
		t.Error("必填参数缺失时应当报错")
	}
}

// 命令行解析要能正确处理引号
func TestSplitCommandLine(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`log --oneline`, []string{"log", "--oneline"}},
		{`log -n 2 "my file.txt"`, []string{"log", "-n", "2", "my file.txt"}},
		{`commit -m 'a b c'`, []string{"commit", "-m", "a b c"}},
		{`  spaced   out  `, []string{"spaced", "out"}},
		{``, nil},
	}
	for _, c := range cases {
		got := SplitCommandLine(c.in)
		if strings.Join(got, "\x00") != strings.Join(c.want, "\x00") {
			t.Errorf("SplitCommandLine(%q) = %v，期望 %v", c.in, got, c.want)
		}
	}
}
