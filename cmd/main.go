package main

import (
	"github.com/lightless233/enum-subdomain-go/internal"
	"github.com/lightless233/enum-subdomain-go/pkg"
)

func main() {
	// 解析命令行参数
	appArgs, err := pkg.ParseCLIArgs()
	if err != nil {
		panic(err)
		return
	}

	// 初始化日志系统
	if appArgs.Debug {
		internal.InitLogger(true)
	} else {
		internal.InitLogger(false)
	}
	logger := internal.GetSugar()

	if appArgs.Debug {
		logger.Infof("AppArgs: %+v", appArgs.PrettyString())
	}

	// 创建 App 并执行
	app := pkg.NewEnumSubdomainApp(appArgs)
	_, err = app.Run()
	if err != nil {
		logger.Fatalf("Error when run EnumSubdomain, error: %+v", err)
		return
	}
}
