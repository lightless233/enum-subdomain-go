package enum_subdomain

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

type BruteEngine struct {
	mainWG    *sync.WaitGroup
	waitGroup *sync.WaitGroup

	bruteTaskChan  chan string
	fofaResultChan chan string
	resultChan     chan *SubdomainResult

	channelStatus []bool
	appArgs       *AppArgs
}

func NewBruteEngine(appArgs *AppArgs, mainWG *sync.WaitGroup, bruteTaskChan, fofaResultChan chan string, resultChan chan *SubdomainResult) *BruteEngine {
	var wg sync.WaitGroup

	return &BruteEngine{
		mainWG:         mainWG,
		waitGroup:      &wg,
		bruteTaskChan:  bruteTaskChan,
		fofaResultChan: fofaResultChan,
		resultChan:     resultChan,
		channelStatus:  []bool{true, true},
		appArgs:        appArgs,
	}
}

func (e *BruteEngine) Run() {
	defer e.mainWG.Done()

	var idx uint = 0
	for ; idx < e.appArgs.TaskCount; idx++ {
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

		domain = fmt.Sprintf("%s.%s", v, e.appArgs.Target)
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
func (e *BruteEngine) resolve(domain string, dnsClient *DNSClient) *DNSResolveResult {
	for retry := 3; retry > 0; retry-- {
		result, err := dnsClient.DoDNSResolve(domain)
		if err != nil {
			logger.Warnf("Error when dns resolve, domain: %s, err: %+v, retry: %d", domain, err, retry)
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

	// 每个协程自己维护 client
	dnsClient := NewDNSClient(e.appArgs.Nameserver)

	logger.Debugf("%s start!", tag)

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
		appResult := &SubdomainResult{}
		appResult.dnsResult = result

		// 如果设置了获取 HTTP 标题的功能，则在这里去获取
		if e.appArgs.FetchTitle {
			httpResult := FetchIndexTitle(domain)
			appResult.httpResult = httpResult
		} else {
			appResult.httpResult = &HTTPResult{}
		}

		// 添加到 result channel
		if len(result.CNAMERecord) != 0 || len(result.ARecord) != 0 {
			e.resultChan <- appResult
		}
	}

	logger.Debugf("%s stop.", tag)
}
