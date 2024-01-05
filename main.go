package main

import (
	"fmt"
	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"
	"math/rand"
	"net"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
)

const LETTERS = "abcdefghijklmnopqrstuvwxyz0123456789"

func main() {
	app := &cli.App{
		Usage:   "Enumerate Subdomains",
		Action:  mainAction,
		Version: "0.1.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "target",
				Usage:       "target domain",
				Destination: &appArgs.Target,
				Aliases:     []string{"t"},
				Required:    true,
				Action: func(context *cli.Context, s string) error {
					// TODO 校验 target 是否为合法的域名
					return nil
				},
			},

			&cli.StringFlag{
				Name:    "technicals",
				Aliases: []string{"x"},
				Usage:   "enumerate technical, available options: D, L, F",
				Value:   "DL",
				Action: func(context *cli.Context, s string) error {
					// 检查 tech 参数是否合法, 如果包含逗号则按照逗号切分，否则直接切分
					var parts []string
					if strings.Contains(s, ",") {
						parts = strings.Split(s, ",")
					} else {
						parts = strings.Split(s, "")
					}

					// 依次检查每个 technical 是否合法
					for _, tech := range parts {
						tech = strings.ToUpper(strings.TrimSpace(tech))
						if tech != "D" && tech != "L" && tech != "F" {
							return fmt.Errorf("technicals argument error, only D, L, F allowed")
						} else {
							appArgs.Technicals = append(appArgs.Technicals, tech)
						}
					}

					return nil
				},
			},

			&cli.StringFlag{
				Name:        "dict-file",
				Usage:       "dice file path",
				Destination: &appArgs.DictFile,
				DefaultText: "empty, use inner dict",
				Aliases:     []string{"d"},
				Value:       "",
				Action: func(context *cli.Context, s string) error {
					// TODO 校验 dict file 是否存在且可以打开
					return nil
				},
			},
			&cli.StringFlag{
				Name:        "brute-length",
				Usage:       "brute length, e.g. 3 or 1-3",
				Aliases:     []string{"l"},
				Destination: &appArgs.BruteLength,
				Value:       "1-3",
				Action: func(context *cli.Context, s string) error {
					// 校验参数是否合法
					pattern := regexp.MustCompile(`(\d+|\d+-\d+)`)
					if pattern.MatchString(s) {
						return nil
					} else {
						return fmt.Errorf("brute length format error")
					}
				},
			},
			&cli.StringFlag{
				Name:        "fofa-token",
				Usage:       "fofa token, format: email|token",
				Aliases:     []string{"f"},
				Destination: &appArgs.FofaToken,
				Value:       "",
			},

			&cli.UintFlag{
				Name:        "task-count",
				Usage:       "Task count",
				Destination: &appArgs.TaskCount,
				Aliases:     []string{"n"},
				Value:       uint(2*runtime.NumCPU() + 1),
				DefaultText: "2 * CPU + 1",
			},
			&cli.BoolFlag{
				Name:        "check-wildcard",
				Usage:       "Whether to detect wildcard and stop brute mode and dict mode if wildcard is detected.",
				Destination: &appArgs.CheckWildcard,
				Value:       true,
			},
			&cli.StringFlag{
				Name:  "nameserver",
				Usage: "Specify DNS servers, use comma to separate multiple DNS",
				Action: func(context *cli.Context, s string) error {
					nameservers := strings.Split(s, ",")
					// 校验 IP 是否合法
					for _, ip := range nameservers {
						parsedIP := net.ParseIP(ip)
						if parsedIP != nil {
							appArgs.Nameserver = append(appArgs.Nameserver, fmt.Sprintf("%s:53", strings.TrimSpace(ip)))
							sugarLogger.Debugf("after append: %+v", appArgs.Nameserver)
						} else {
							return fmt.Errorf("error when parse nameserver: %s", ip)
						}
					}

					return nil
				},
			},
			&cli.BoolFlag{
				Name:        "fetch-title",
				Usage:       "Whether to get the page title",
				Destination: &appArgs.FetchTitle,
				Value:       false,
			},
			&cli.StringFlag{
				Name:        "output",
				Usage:       "output filename",
				Aliases:     []string{"o"},
				Destination: &appArgs.OutputFile,
				Value:       "./out.txt",
			},
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Debug mode",
				Value:       false,
				Destination: &appArgs.Debug,
				Hidden:      true,
			},
		},
		Before: func(context *cli.Context) error {
			// 初始化日志系统
			debug := context.Bool("debug")
			InitLogger(debug)

			return nil
		},
	}

	// Run App
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("\nError when run app. Error: %+v\n", err)
		os.Exit(1)
	}
}

func mainAction(context *cli.Context) error {
	sugarLogger.Debugf("raw app args: %+v", appArgs)

	// 校验参数是否冲突
	if !context.IsSet("technicals") {
		return fmt.Errorf("technicals can't be empty")
	}
	for _, tech := range appArgs.Technicals {
		if tech == "F" && appArgs.FofaToken == "" {
			return fmt.Errorf("fofa token can't be empty when set 'F' technical")
		}
	}
	// 如果没有从命令行参数传入 NS，那么就手动设置
	if appArgs.Nameserver == nil || len(appArgs.Nameserver) == 0 {
		appArgs.Nameserver = []string{
			"223.5.5.5:53", "223.6.6.6:53",
			"8.8.8.8:53", "8.8.4.4:53",
			"9.9.9.9:53",
			"114.114.114.114:53",
			"1.2.4.8:53", "210.2.4.8:53",
		}
		sugarLogger.Info("Use default nameserver list:\n", appArgs.Nameserver)
	}

	// 检查 nameserver 的连通性
	sugarLogger.Info("Checking nameserver connection...")
	dnsClient := BuildDNSClient()
	connectedNameserver := checkNameserversConnection(dnsClient)
	if len(connectedNameserver) == 0 {
		sugarLogger.Error("No connected nameserver, exit.")
		os.Exit(1)
	} else {
		appArgs.Nameserver = connectedNameserver
	}

	// 如果设定了--dict参数且technicals中不包含D，添加进 technicals
	// 不再做这个解析了，只根据 technicals 决定启动的引擎
	//if !slices.Contains(appArgs.Technicals, "D") && context.IsSet("dict-file") {
	//	appArgs.Technicals = append(appArgs.Technicals, "D")
	//}
	//if !slices.Contains(appArgs.Technicals, "L") && context.IsSet("brute-length") {
	//	appArgs.Technicals = append(appArgs.Technicals, "L")
	//}
	//if !slices.Contains(appArgs.Technicals, "F") && context.IsSet("fofa-token") {
	//	appArgs.Technicals = append(appArgs.Technicals, "F")
	//}

	sugarLogger.Debugf("after format args: %+v", appArgs.PrettyString())

	// 先进行一次泛解析检查，如果有泛解析并且未指定 fofa 模式，直接退出
	if appArgs.CheckWildcard {
		sugarLogger.Infof("Start check wildcard")
		if checkWildcard(dnsClient) {
			if slices.Contains(appArgs.Technicals, "F") {
				sugarLogger.Warnf("Found wildcard, only `F` technical will execute.")
			} else {
				sugarLogger.Error("Found wildcard, exit.")
				os.Exit(1)
			}
		}
	}

	// 主 goroutine 同步使用
	var waitGroup sync.WaitGroup

	// 创建所有的队列
	bruteTaskChan := make(chan string, 256)
	fofaTaskChan := make(chan string, 1)
	resultChan := make(chan *AppResult, 128)

	// TODO 根据不同参数，启动不同的引擎
	// 启动 resultEngine
	resultEngine := NewResultEngine(&waitGroup, resultChan)
	waitGroup.Add(1)
	go resultEngine.Run()

	// 启动 engine wrapper
	engineWrapper := NewEngineWrapper(&waitGroup, bruteTaskChan, fofaTaskChan, resultChan)
	waitGroup.Add(1)
	go engineWrapper.Run()

	// 启动 taskBuilder
	taskBuilderEngine := NewTaskBuilderEngine(&waitGroup, bruteTaskChan, fofaTaskChan)
	waitGroup.Add(1)
	go taskBuilderEngine.Run()

	waitGroup.Wait()

	return nil
}

func randStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = LETTERS[rand.Intn(len(LETTERS))]
	}
	return string(b)
}

// checkNameserversConnection 检查 appArgs.Nameserver 的连通性
func checkNameserversConnection(dnsClient *dns.Client) []string {
	// 构造 query 消息
	var msg dns.Msg
	msg.SetQuestion(dns.Fqdn("www.baidu.com"), dns.TypeA)
	msg.RecursionDesired = true

	connectedNameserver := make([]string, 0, len(appArgs.Nameserver))

	for _, ns := range appArgs.Nameserver {
		if checkNameserverConnection(dnsClient, ns, &msg) {
			connectedNameserver = append(connectedNameserver, ns)
		} else {
			sugarLogger.Warnf("nameserver %s connected failed, remove it.", ns)
		}
	}

	return connectedNameserver
}

// checkNameserverConnection 检查单个 ns 的连通性
func checkNameserverConnection(dnsClient *dns.Client, ns string, msg *dns.Msg) bool {

	for i := 3; i > 0; i-- {
		_, _, err := dnsClient.Exchange(msg, ns)
		if err == nil {
			return true
		}
	}

	return false
}

// checkWildcard 检查目标是否存在泛解析
// true: 有泛解析，false：无泛解析
func checkWildcard(dnsClient *dns.Client) bool {
	// 生成一个随机域名，一个写死的不存在域名
	domains := []string{"this-domain-will-never-exist", randStr(8)}

	hasWildcard := []bool{false, false}
	for idx, domain := range domains {
		res, err := DoDNSResolve(domain, dnsClient)
		if err != nil {
			continue
		} else {
			if len(res.ARecord) != 0 || len(res.CNAMERecord) != 0 {
				hasWildcard[idx] = true
			}
		}
	}

	// 两个域名都有结果，大概率是泛解析
	// TODO 如果设置了 fofa 模式，则跳过字典爆破和暴力爆破，仅启动fofa模式
	if !slices.Contains(hasWildcard, false) {
		// sugarLogger.Infof("Found wildcard, stop.")
		appArgs.HasWildcard = true
		return true
	}

	return false
}
