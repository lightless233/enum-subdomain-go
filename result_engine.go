package main

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"
	"sync"
)

type AppResult struct {
	dnsResult  *DNSResolveResult
	httpResult *HTTPResult
	// httpError  string
}

type ResultEngine struct {
	mainWG    *sync.WaitGroup
	waitGroup *sync.WaitGroup

	resultChan chan *AppResult
}

func NewResultEngine(mainWG *sync.WaitGroup, resultChan chan *AppResult) *ResultEngine {
	var wg sync.WaitGroup
	return &ResultEngine{
		mainWG:     mainWG,
		waitGroup:  &wg,
		resultChan: resultChan,
	}
}

func (engine *ResultEngine) Run() {
	defer engine.mainWG.Done()

	engine.waitGroup.Add(1)
	go engine.worker()
	engine.waitGroup.Wait()
}

func (engine *ResultEngine) worker() {
	defer engine.waitGroup.Done()

	fp, err := os.OpenFile(appArgs.OutputFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { _ = fp.Close() }()
	if err != nil {
		sugarLogger.Fatalf("Can't open output file to write. filename: %s, error: %+v", appArgs.OutputFile, err)
		panic(err)
	}
	writer := csv.NewWriter(fp)
	_ = writer.Write([]string{
		"DOMAIN", "CNAME", "A", "STATUS_CODE", "TITLE", "LOCATION", "CONTENT_LENGTH", "HTTP_ERROR",
	})

	// 加个 buffer 去重使用
	buffer := make(map[string]string)

	sugarLogger.Debugf("ResultEngine start.")
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
		// err := writer.Write([]string{task.domain, strings.Join(task.CNAMERecord, ","), strings.Join(task.ARecord, ",")})
		if err != nil {
			sugarLogger.Fatalf("Can't write result file, filename: %s, error: %+v", appArgs.OutputFile, err)
			panic(err)
		}
		writer.Flush()

		// 打印结果
		sugarLogger.Infof("%s - %v - %v || %d - %s - %s - %d",
			task.dnsResult.domain, task.dnsResult.CNAMERecord, task.dnsResult.ARecord,
			task.httpResult.statusCode, task.httpResult.title, task.httpResult.location, task.httpResult.bodyLength,
		)
	}
	sugarLogger.Debugf("ResultEngine end.")
}
