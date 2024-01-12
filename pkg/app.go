package enum_subdomain

import (
	"fmt"
	"github.com/lightless233/enum-subdomain-go/internal"
	"slices"
	"sync"
)

var logger = internal.GetSugar()

type EnumSubdomainApp struct {
	args *AppArgs
}

func NewEnumSubdomainApp(args *AppArgs) *EnumSubdomainApp {
	// 如果是从 SDK 进来的，需要初始化一下日志
	if logger == nil {
		if args.Debug {
			internal.InitLogger(true)
		} else {
			internal.InitLogger(false)
		}
	}

	return &EnumSubdomainApp{args: args}
}

func (app *EnumSubdomainApp) checkTechnicals() error {
	if app.args.Technicals == nil || len(app.args.Technicals) == 0 {
		return fmt.Errorf("technical can't be empty")
	}

	for _, tech := range app.args.Technicals {
		if tech == "F" && app.args.FofaToken == "" {
			return fmt.Errorf("fofa token can't be empty when set 'F' technical")
		}
	}

	return nil
}

func (app *EnumSubdomainApp) checkNameserver() (*DNSClient, error) {
	// 如果没有设置 nameservers，那么使用默认值
	if app.args.Nameserver == nil || len(app.args.Nameserver) == 0 {
		app.args.Nameserver = []string{
			"223.5.5.5:53", "223.6.6.6:53",
			"8.8.8.8:53", "8.8.4.4:53",
			"9.9.9.9:53",
			"114.114.114.114:53", "114.114.115.115:53",
			"1.2.4.8:53", "210.2.4.8:53",
			"119.29.29.29:53",
		}

		logger.Infof("Use default nameservers: %+v", app.args.Nameserver)

	}

	// 检查 nameserver 的连通性，移除无法连通的 nameserver
	logger.Debug("Start checking ns connection...")
	dnsClient := NewDNSClient(app.args.Nameserver)
	unconnectedNSList, connectedNSList := dnsClient.RemoveUnconnectedNS()
	if len(unconnectedNSList) > 0 {
		logger.Warnf("Remove unconnected nameservers: %+v", unconnectedNSList)
	}

	// 如果移除完了之后是空的，则返回错误
	if len(dnsClient.nameservers) == 0 {
		return nil, fmt.Errorf("all nameservers are unconnected")
	}

	// 把连通的 NS 更新回 appArgs ，后续使用
	app.args.Nameserver = connectedNSList

	return dnsClient, nil
}

func (app *EnumSubdomainApp) checkWildcard(dnsClient *DNSClient) error {
	if dnsClient.CheckDomainWildcard(app.args.Target) {
		logger.Warnf("Found wildcard, only `F` technical will execute.")

		// 如果没有设置 F 模式，直接返回 error
		if !slices.Contains(app.args.Technicals, "F") {
			return fmt.Errorf("found wildcard, only `F` technical will execute")
		}

		app.args.HasWildcard = true
	}

	return nil
}

// checkArgs 检查指定的 args 是否合法
func (app *EnumSubdomainApp) checkArgs() error {

	// 检查 technicals 是否合法
	if err := app.checkTechnicals(); err != nil {
		return err
	}

	// 检查 nameserver 是否合法
	dnsClient, err := app.checkNameserver()
	if err != nil {
		return err
	}

	// 如果设定了泛解析检查，先跑一次 DNS 解析
	if app.args.CheckWildcard {
		logger.Info("Start checking wildcard...")
		if err := app.checkWildcard(dnsClient); err != nil {
			return err
		}
	}

	return nil
}

// Run 真正的程序入口，不管是 CLI 进来的，还是 API 进来的，都会调用这个函数开始执行
func (app *EnumSubdomainApp) Run() ([]*SubdomainResult, error) {
	// 检查参数是否合法
	if err := app.checkArgs(); err != nil {
		return nil, err
	}

	// 主 goroutine 同步使用
	var waitGroup sync.WaitGroup

	// 创建所有的队列
	bruteTaskChan := make(chan string, 256)
	fofaTaskChan := make(chan string, 1)
	resultChan := make(chan *SubdomainResult, 128)

	// 启动 resultEngine
	resultEngine := NewResultEngine(app.args, &waitGroup, resultChan)
	waitGroup.Add(1)
	go resultEngine.Run()

	// 启动 engine wrapper
	engineWrapper := NewEngineWrapper(app.args, &waitGroup, bruteTaskChan, fofaTaskChan, resultChan)
	waitGroup.Add(1)
	go engineWrapper.Run()

	// 启动 taskBuilder
	taskBuilderEngine := NewTaskBuilderEngine(app.args, &waitGroup, bruteTaskChan, fofaTaskChan)
	waitGroup.Add(1)
	go taskBuilderEngine.Run()

	waitGroup.Wait()

	// 等待结束后，所有的引擎已经正常退出了，获取 ResultEngine 中的结果
	subdomains := resultEngine.subdomainResult
	return subdomains, nil
}
