// dump 是一个引擎自检工具：不启动界面，直接把引擎读到的仓库快照打成 JSON。
//
// 用途：当界面显示不对时，先用它确认到底是「引擎没读到数据」还是「界面没画出来」。
//
//	go run ./cmd/dump /path/to/repo
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"lazygit-desktop/engine"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	e, err := engine.New()
	if err != nil {
		fmt.Println("engine.New 失败:", err)
		os.Exit(1)
	}

	snap, err := e.OpenRepo(dir)
	if err != nil {
		fmt.Println("OpenRepo 失败:", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Println("序列化失败:", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
