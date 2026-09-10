package engine

import (
	"fmt"
	"os/exec"
	"strings"
)

// git 提交必须知道「你是谁」。如果 user.name / user.email 没配置，
// git 会直接拒绝提交：
//
//	fatal: unable to auto-detect email address
//	*** Please tell me who you are.
//
// 这是新手最容易卡住的地方（尤其是刚装完 git 的用户），
// 所以界面需要能主动发现并引导用户设置，而不是丢一句报错。

// identityLocked 读取当前生效的提交身份。调用方需持有 e.mu。
func (e *Engine) identityLocked() (string, string) {
	if e.gitConfig == nil {
		return "", ""
	}
	name := strings.TrimSpace(e.gitConfig.Get("user.name"))
	email := strings.TrimSpace(e.gitConfig.Get("user.email"))
	return name, email
}

// Identity 返回当前生效的提交身份。
func (e *Engine) Identity() (string, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.identityLocked()
}

// SetIdentity 设置提交身份。
//
// global=true 写全局配置（~/.gitconfig，对所有仓库生效），
// global=false 只写当前仓库（.git/config）。
//
// 对绝大多数用户来说，第一次打开软件时设一次全局身份就够了。
func (e *Engine) SetIdentity(name, email string, global bool) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		return nil, fmt.Errorf("请填写名字")
	}
	if email == "" {
		return nil, fmt.Errorf("请填写邮箱")
	}
	if !strings.Contains(email, "@") {
		return nil, fmt.Errorf("邮箱格式看起来不对：%s", email)
	}

	scope := "--local"
	if global {
		scope = "--global"
	}

	for _, kv := range [][2]string{{"user.name", name}, {"user.email", email}} {
		cmd := exec.Command("git", "config", scope, kv[0], kv[1])
		cmd.Dir = e.repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("设置 %s 失败: %v %s", kv[0], err, strings.TrimSpace(string(out)))
		}
	}

	// 配置读的是缓存，改完必须清掉，否则界面还是显示旧值
	if e.gitConfig != nil {
		e.gitConfig.DropCache()
	}

	return e.snapshotLocked()
}
