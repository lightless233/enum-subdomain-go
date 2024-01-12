package enum_subdomain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"net"
	"os"
	"regexp"
	"runtime"
	"strings"
)

type AppArgs struct {
	Target string

	Technicals []string

	DictFile    string
	BruteLength string
	FofaToken   string

	OutputFile    string
	TaskCount     uint
	CheckWildcard bool
	Nameserver    []string
	FetchTitle    bool

	FromCLI     bool // true 表示是从命令行进入的，默认为 false 表示从 SDK 引入
	Debug       bool
	HasWildcard bool
}

func (a *AppArgs) PrettyString() string {
	bs, _ := json.Marshal(a)
	var out bytes.Buffer
	json.Indent(&out, bs, "", "    ")
	return out.String()
}

func ParseCLIArgs() (*AppArgs, error) {

	appArgs := &AppArgs{}

	app := &cli.App{
		Usage: "Enumerate Subdomains",
		Action: func(context *cli.Context) error {
			appArgs.FromCLI = true
			return nil
		},
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
	}

	if err := app.Run(os.Args); err != nil {
		return nil, fmt.Errorf("error when run app. %+v", err)
	}

	return appArgs, nil
}
