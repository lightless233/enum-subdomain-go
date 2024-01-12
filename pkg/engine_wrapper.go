package pkg

import "sync"

type EngineWrapper struct {
	mainWG    *sync.WaitGroup
	waitGroup *sync.WaitGroup

	bruteTaskChan chan string
	fofaTaskChan  chan string
	resultChan    chan *SubdomainResult

	appArgs *AppArgs
}

func NewEngineWrapper(appArgs *AppArgs, mainWG *sync.WaitGroup, bruteTaskChan, fofaTaskChan chan string, resultChan chan *SubdomainResult) *EngineWrapper {
	var wg sync.WaitGroup
	return &EngineWrapper{
		mainWG:        mainWG,
		waitGroup:     &wg,
		bruteTaskChan: bruteTaskChan,
		fofaTaskChan:  fofaTaskChan,
		resultChan:    resultChan,
		appArgs:       appArgs,
	}
}

func (wrapper *EngineWrapper) Run() {
	defer func() {
		wrapper.mainWG.Done()
		close(wrapper.resultChan)
	}()

	// 这个 channel 只在 fofaEngine 和 bruteEngine 中使用，不需要暴露出去
	fofaResultChan := make(chan string, 128)

	// 启动 dns engine 和 fofa engine
	bruteEngine := NewBruteEngine(wrapper.appArgs, wrapper.waitGroup, wrapper.bruteTaskChan, fofaResultChan, wrapper.resultChan)
	wrapper.waitGroup.Add(1)
	go bruteEngine.Run()

	fofaEngine := NewFofaEngine(wrapper.appArgs, wrapper.waitGroup, wrapper.fofaTaskChan, fofaResultChan)
	wrapper.waitGroup.Add(1)
	go fofaEngine.Run()

	// 等待子引擎结束
	wrapper.waitGroup.Wait()
}
