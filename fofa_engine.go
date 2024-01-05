package main

import (
	"encoding/base64"
	"fmt"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	netURL "net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FofaEngine struct {
	mainWG         *sync.WaitGroup
	waitGroup      *sync.WaitGroup
	fofaTaskChan   chan string
	fofaResultChan chan string
}

func NewFofaEngine(mainWG *sync.WaitGroup, fofaTaskChan, fofaResultChan chan string) *FofaEngine {
	var wg sync.WaitGroup
	return &FofaEngine{
		mainWG:         mainWG,
		waitGroup:      &wg,
		fofaTaskChan:   fofaTaskChan,
		fofaResultChan: fofaResultChan,
	}
}

func (engine *FofaEngine) Run() {
	defer func() {
		engine.mainWG.Done()
		close(engine.fofaResultChan)
	}()

	engine.waitGroup.Add(1)
	go engine.worker()

	engine.waitGroup.Wait()
}

func (engine *FofaEngine) worker() {
	defer engine.waitGroup.Done()

	if appArgs.FofaToken == "" {
		return
	}

	fofaURL := "https://fofa.info/api/v1/search/all?email=${e}&key=${k}&qbase64=${q}&page=${p}"
	fofaParts := strings.Split(appArgs.FofaToken, "|")
	fofaEmail := fofaParts[0]
	fofaToken := fofaParts[1]

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	sugarLogger.Debugf("FofaEngine start.")
	for {
		task, opened := <-engine.fofaTaskChan
		if !opened {
			break
		}
		sugarLogger.Debugf("Received fofa task: %+v", task)

		// 最大查询 30 页
		var fofaResults []string
		q := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("domain=%s", appArgs.Target)))
		for p := 1; p <= 30; p++ {
			url := strings.ReplaceAll(fofaURL, "${q}", q)
			url = strings.ReplaceAll(url, "${p}", strconv.Itoa(p))
			url = strings.ReplaceAll(url, "${e}", fofaEmail)
			url = strings.ReplaceAll(url, "${k}", fofaToken)

			bContent, err := func() ([]byte, error) {
				sugarLogger.Debug("Start fetch page ", p)
				request, err := http.NewRequest("GET", url, nil)

				if err != nil {
					return nil, err
				}
				response, err := httpClient.Do(request)
				defer func() { _ = response.Body.Close() }()
				if err != nil {
					return nil, err
				}
				bContent, err := io.ReadAll(response.Body)
				if err != nil {
					return nil, err

				}

				return bContent, nil
			}()
			if err != nil {
				sugarLogger.Warnf("error when fetch page %d, error: %+v, skip this page", p, err)
				continue
			}

			/**
			请求结果样例：
			{
			    "error": false,
			    "consumed_fpoint": 0,
			    "required_fpoints": 0,
			    "size": 11,
			    "page": 1,
			    "mode": "extended",
			    "query": "domain=\"lightless.me\"",
			    "results": [
			        ["c1.lightless.me", "43.129.25.182", "80"],
			        ["https://c1.lightless.me", "43.129.25.182", "443"],
			        ["www.lightless.me", "43.129.25.182", "80"],
			        ["https://www.lightless.me", "43.129.25.182", "443"],
			        ["https://lightless.me", "43.129.25.182", "443"],
			        ["https://lightless.me", "43.129.25.182", "443"],
			        ["lightless.me", "43.129.25.182", "80"],
			        ["lightless.me:53", "43.129.25.182", "53"],
			        ["lightless.me:22", "43.129.25.182", "22"],
			        ["lightless.me", "43.129.25.182", "80"],
			        ["ss.lightless.me:10022", "216.24.176.101", "10022"]
			    ]
			}
			*/
			// 把结果转换成 JSON 对象
			var data map[string]interface{}
			err = sonic.Unmarshal(bContent, &data)
			if err != nil {
				sugarLogger.Warnf("error when convert page %d data to json, error: %+v, skip this page", p, err)
				continue
			}

			// 判断是否有结果，如果没有结果了，直接跳出循环
			hasError := data["error"].(bool)
			if hasError {
				break
			}
			results := data["results"].([]interface{})
			if len(results) == 0 {
				break
			}

			for _, res := range results {
				rawURL := res.([]interface{})[0].(string)
				if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
					rawURL = fmt.Sprintf("https://%s", rawURL)
				}

				parsed, err := netURL.Parse(rawURL)
				if err != nil {
					sugarLogger.Warnf("Error when parse raw url: %s, err: %+v", rawURL, err)
					continue
				}

				domain := parsed.Hostname()

				// 存起来
				if !slices.Contains(fofaResults, domain) {
					fofaResults = append(fofaResults, domain)
				}
			}
		}

		// 从fofa获取完成，通过其他的 channel 发送给 brute_engine
		for _, domain := range fofaResults {
			sugarLogger.Debugf("Put %s to channel", domain)
			engine.fofaResultChan <- domain
		}
		sugarLogger.Infof("Found %d domain from fofa, start verify...", len(fofaResults))

	}
	sugarLogger.Debugf("FofaEngine end.")
}
