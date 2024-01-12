package enumsubdomain

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"
	"sync"
)

type SubdomainResult struct {
	dnsResult  *DNSResolveResult
	httpResult *HTTPResult
	// httpError  string
}

type ResultEngine struct {
	mainWG          *sync.WaitGroup
	waitGroup       *sync.WaitGroup
	resultChan      chan *SubdomainResult
	appArgs         *AppArgs
	subdomainResult []*SubdomainResult
}

func NewResultEngine(appArgs *AppArgs, mainWG *sync.WaitGroup, resultChan chan *SubdomainResult) *ResultEngine {
	var wg sync.WaitGroup
	return &ResultEngine{
		mainWG:          mainWG,
		waitGroup:       &wg,
		resultChan:      resultChan,
		appArgs:         appArgs,
		subdomainResult: make([]*SubdomainResult, 0),
	}
}

func (engine *ResultEngine) Run() {
	defer engine.mainWG.Done()

	engine.waitGroup.Add(1)
	go engine.worker()
	engine.waitGroup.Wait()
}

func (engine *ResultEngine) worker() {
	defer func() {
		engine.waitGroup.Done()
		if r := recover(); r != nil {
			logger.Errorf("ResultEngine panic: %+v", r)
		}
	}()

	fp, err := os.OpenFile(engine.appArgs.OutputFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { _ = fp.Close() }()
	if err != nil {
		logger.Fatalf("Can't open output file to write. filename: %s, error: %+v", engine.appArgs.OutputFile, err)
		panic(err)
	}
	writer := csv.NewWriter(fp)
	_ = writer.Write([]string{
		"DOMAIN", "CNAME", "A", "STATUS_CODE", "TITLE", "LOCATION", "CONTENT_LENGTH", "HTTP_ERROR",
	})

	// 加个 buffer 去重使用
	buffer := make(map[string]string)

	logger.Debugf("ResultEngine start.")
	for {
		task, opened := <-engine.resultChan
		if !opened {
			break
		}

		// 过滤掉为空的结果
		if len(task.dnsResult.ARecord) == 0 && len(task.dnsResult.CNAMERecord) == 0 {
			continue
		}

		// 去重逻辑
		_, ok := buffer[task.dnsResult.domain]
		if ok {
			continue
		} else {
			buffer[task.dnsResult.domain] = ""
		}

		// 写入结果文件
		err := writer.Write([]string{
			task.dnsResult.domain,
			strings.Join(task.dnsResult.CNAMERecord, ","),
			strings.Join(task.dnsResult.ARecord, ","),
			strconv.Itoa(int(task.httpResult.statusCode)),
			task.httpResult.title,
			task.httpResult.location,
			strconv.Itoa(int(task.httpResult.bodyLength)),
			task.httpResult.error,
		})
		if err != nil {
			logger.Fatalf("Can't write result file, filename: %s, error: %+v", engine.appArgs.OutputFile, err)
			panic(err)
		}
		writer.Flush()

		engine.subdomainResult = append(engine.subdomainResult, task)

		// 只有从命令行执行的时候才打印结果
		if engine.appArgs.FromCLI {
			logger.Infof("%s - %v - %v || %d - %s - %s - %d",
				task.dnsResult.domain, task.dnsResult.CNAMERecord, task.dnsResult.ARecord,
				task.httpResult.statusCode, task.httpResult.title, task.httpResult.location, task.httpResult.bodyLength,
			)
		}
	}
	logger.Debugf("ResultEngine end.")
}
