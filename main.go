package main

import (
	"github.com/Mrs4s/go-cqhttp/cmd/gocq"
	_ "github.com/Mrs4s/go-cqhttp/db/leveldb" // leveldb
	"github.com/Mrs4s/go-cqhttp/global"
	_ "github.com/Mrs4s/go-cqhttp/modules/mime"  // mime检查模块
	_ "github.com/Mrs4s/go-cqhttp/modules/pprof" // pprof 性能分析
	_ "github.com/Mrs4s/go-cqhttp/modules/silk"  // silk编码模块
	"runtime"
)

func main() {
	if runtime.GOOS == "windows"{
		global.AdjustConsoleWindowSize(640, 800)
	}
	gocq.Main()
}


