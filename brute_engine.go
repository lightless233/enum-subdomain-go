package main

import (
	"fmt"
	"github.com/miekg/dns"
	"slices"
	"sync"
	"time"
)

type BruteEngine struct {
	mainWG    *sync.WaitGroup
	waitGroup *sync.WaitGroup

	bruteTaskChan  chan string
	fofaResultChan chan string
	resultChan     chan *AppResult

	channelStatus []bool
}

func NewBruteEngine(mainWG *sync.WaitGroup, bruteTaskChan, fofaResultChan chan string, resultChan chan *AppResult) *BruteEngine {
	var wg sync.WaitGroup

	return &BruteEngine{
		mainWG:         mainWG,
		waitGroup:      &wg,
		bruteTaskChan:  bruteTaskChan,
		fofaResultChan: fofaResultChan,
		resultChan:     resultChan,
		channelStatus:  []bool{true, true},
	}
}

func (e *BruteEngine) Run() {
	defer e.mainWG.Done()

	var idx uint = 0
	for ; idx < appArgs.TaskCount; idx++ {
		e.waitGroup.Add(1)
		go e.worker(idx)
	}

	e.waitGroup.Wait()
}

// fetchTaskFromChannel 从多个 channel 中监听任务，收到任务后就返回
func (e *BruteEngine) fetchTaskFromChannel(timeout time.Duration) string {
	var domain string

	// 同时监听两个 channel，获取任务
	select {
	case v, opened := <-e.bruteTaskChan:
		if !opened {
			e.bruteTaskChan = nil
			e.channelStatus[0] = false
			break
		}

		domain = fmt.Sprintf("%s.%s", v, appArgs.Target)
	case v, opened := <-e.fofaResultChan:
		if !opened {
			e.fofaResultChan = nil
			e.channelStatus[1] = false
			break
		}

		domain = v
	default:
		time.Sleep(timeout * time.Second)
		break
	}

	return domain
}

// resolve 执行 DNS 解析，最多重试三次
func (e *BruteEngine) resolve(domain string, dnsClient *dns.Client) *DNSResolveResult {
	for retry := 3; retry > 0; retry-- {
		result, err := DoDNSResolve(domain, dnsClient)
		if err != nil {
			sugarLogger.Warnf("Error when dns resolve, domain: %s, err: %+v, retry: %d", domain, err, retry)
			continue
		} else {
			return result
		}
	}
	return nil
}

func (e *BruteEngine) worker(idx uint) {
	defer e.waitGroup.Done()

	tag := fmt.Sprintf("[BruteEngine-%d]", idx)

	// 每个协程都自己维护一个 client
	dnsClient := BuildDNSClient()

	sugarLogger.Debugf("%s start!", tag)

	for {
		// 如果所有监听的channel都关闭了，状态都是 false 就跳出主循环
		if !slices.Contains(e.channelStatus, true) {
			break
		}

		// 从监听的channel中获取任务
		domain := e.fetchTaskFromChannel(1)
		if domain == "" {
			continue
		}
		// sugarLogger.Debugf("%s Receive task: %s", tag, domain)

		// 执行 DNS 解析
		result := e.resolve(domain, dnsClient)
		if result == nil {
			continue
		}

		// 提前跳过没有解析记录的结果
		if len(result.ARecord) == 0 && len(result.CNAMERecord) == 0 {
			continue
		}

		// 最终的扫描结果
		appResult := &AppResult{}
		appResult.dnsResult = result

		// 如果设置了获取 HTTP 标题的功能，则在这里去获取
		if appArgs.FetchTitle {
			httpResult := FetchIndexTitle(domain)
			appResult.httpResult = httpResult
			// appResult.httpError = err.Error()
		} else {
			appResult.httpResult = &HTTPResult{}
			// appResult.httpError = ""
		}

		// 添加到 result channel
		if len(result.CNAMERecord) != 0 || len(result.ARecord) != 0 {
			// sugarLogger.Infof("%s - %v", domain, result)
			e.resultChan <- appResult
		}
	}

	sugarLogger.Debugf("%s stop.", tag)
}
