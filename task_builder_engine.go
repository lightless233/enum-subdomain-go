package main

import (
	"bufio"
	_ "embed"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

//go:embed default_dict.txt
var defaultDict string

type TaskBuilderEngine struct {
	mainWG        *sync.WaitGroup
	waitGroup     *sync.WaitGroup
	bruteTaskChan chan string
	fofaTaskChan  chan string

	alphaTable []string
}

func NewTaskBuilderEngine(mainWG *sync.WaitGroup, bruteTaskChan, fofaTaskChan chan string) *TaskBuilderEngine {
	var wg sync.WaitGroup
	return &TaskBuilderEngine{
		mainWG:        mainWG,
		waitGroup:     &wg,
		bruteTaskChan: bruteTaskChan,
		fofaTaskChan:  fofaTaskChan,
		alphaTable:    buildAlphaTable(),
	}
}

func (e *TaskBuilderEngine) Run() {
	defer func() {
		e.mainWG.Done()
		close(e.bruteTaskChan)
		close(e.fofaTaskChan)
	}()
	e.waitGroup.Add(1)
	go e.worker()
	e.waitGroup.Wait()
}

func (e *TaskBuilderEngine) worker() {
	defer e.waitGroup.Done()

	// 遍历 Technicals，根据指定的 tech 生成任务
	for _, tech := range appArgs.Technicals {
		sugarLogger.Infof("Build task for technical %s", tech)
		if tech == "D" {
			// 字典的
			if !appArgs.HasWildcard {
				e.buildDictTask()
			}
		} else if tech == "L" {
			// 长度爆破的
			if !appArgs.HasWildcard {
				e.buildBruteLengthTask()
			}
		} else if tech == "F" {
			// FOFA 收集的
			e.buildFofaTask()
		} else {
			sugarLogger.Warnf("Unknown technical: %s, skip it.", tech)
		}
	}
}

// buildDictTask 从字典模式构建任务
func (e *TaskBuilderEngine) buildDictTask() {
	if appArgs.DictFile != "" {
		fp, err := os.Open(appArgs.DictFile)
		defer func() { _ = fp.Close() }()
		if err != nil {
			sugarLogger.Warnf("Error when reading dict file %s, error: %+v", appArgs.DictFile, err)
			return
		}

		// 按行读文件
		br := bufio.NewReader(fp)
		for {
			line, err := br.ReadString('\n')
			if err != nil && err != io.EOF {
				// 读取过程中遇到了错误
				sugarLogger.Warnf("Error when reading line in dict file %s, error: %+v", appArgs.DictFile, err)
				break
			}

			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}

			e.bruteTaskChan <- line
			sugarLogger.Debugf("Add task %s to chan", line)

			if err == io.EOF {
				break
			}
		}
	} else {
		// 使用内置字典
		innerDict := strings.Split(defaultDict, "\n")
		sugarLogger.Infof("Load inner default dict, count: %d", len(innerDict))
		for _, task := range innerDict {
			task = strings.TrimSpace(task)
			if task == "" || strings.HasPrefix(task, "#") {
				continue
			}

			e.bruteTaskChan <- task
		}
	}
}

// buildBruteLengthTask 从爆破模式构建任务
func (e *TaskBuilderEngine) buildBruteLengthTask() {
	// 先解析 brute-length 参数，如果是单个数字，直接跑，如果是区间，则依次生成
	var minLength, maxLength uint64
	if strings.Contains(appArgs.BruteLength, "-") {
		// 区间
		parts := strings.Split(appArgs.BruteLength, "-")
		minLength, _ = strconv.ParseUint(parts[0], 10, 32)
		maxLength, _ = strconv.ParseUint(parts[1], 10, 32)
	} else {
		l, _ := strconv.ParseUint(appArgs.BruteLength, 10, 32)
		minLength = l
		maxLength = l
	}

	for i := minLength; i <= maxLength; i++ {
		sugarLogger.Debug("Start build task for length ", i)
		for _, item := range product(e.alphaTable, int(i)) {
			task := strings.Join(item, "")
			if !strings.HasSuffix(task, "-") && !strings.HasPrefix(task, "-") {
				e.bruteTaskChan <- task
			}
		}
		sugarLogger.Debugf("Build task for length %d done.", i)
	}
}

// buildFofaTask 创建一个 fofa 任务
func (e *TaskBuilderEngine) buildFofaTask() {
	// fofa 只要发个通知就行了
	e.fofaTaskChan <- "fofa"
}

func buildAlphaTable() []string {
	table := make([]string, 0, 37)
	for i := 97; i < 123; i++ {
		table = append(table, string(rune(i)))
	}
	for i := 48; i < 58; i++ {
		table = append(table, string(rune(i)))
	}
	table = append(table, "-")
	return table
}

func product(a []string, k int) [][]string {
	indexes := make([]int, k)
	var ps [][]string

	for indexes != nil {
		p := make([]string, k)
		for i, x := range indexes {
			p[i] = a[x]
		}

		for i := len(indexes) - 1; i >= 0; i-- {
			indexes[i]++
			if indexes[i] < len(a) {
				break
			}
			indexes[i] = 0
			if i <= 0 {
				indexes = nil
				break
			}
		}
		ps = append(ps, p)
	}
	return ps
}
