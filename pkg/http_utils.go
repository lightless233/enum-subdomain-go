package pkg

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

type HTTPResult struct {
	title      string
	location   string
	statusCode uint
	bodyLength uint
	error      string
}

var httpClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Timeout:   12 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

var titlePattern = regexp.MustCompile(`<title>(?P<t>.+?)</title>`)

// FetchIndexTitle 获取网页标题，状态码，Body 长度等信息
func FetchIndexTitle(domain string) *HTTPResult {
	// 先补 HTTPS ，如果请求失败了再补 HTTP

	urls := []string{
		fmt.Sprintf("https://%s", domain),
		fmt.Sprintf("http://%s", domain),
	}

	lastErr := ""
	for _, url := range urls {
		httpResult := makeRequest(url)
		if httpResult.error == "" {
			// 如果 error 字段是空的，说明请求成功了，直接返回 httpResult 即可
			return httpResult
		} else {
			// 记录下最后一次 err，返回时使用
			lastErr = httpResult.error
		}
	}

	// 如果都请求失败了，则返回一个仅填充了 error 字段的 HTTPResult
	return &HTTPResult{error: lastErr}
}

func makeRequest(url string) *HTTPResult {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &HTTPResult{error: err.Error()}
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return &HTTPResult{error: err.Error()}
	}

	bContent, err := io.ReadAll(response.Body)
	defer func() { _ = response.Body.Close() }()
	if err != nil {
		return &HTTPResult{error: err.Error()}
	}
	content := string(bContent[:])
	statusCode := response.StatusCode
	location := response.Header.Get("Location")

	// 从 content 中获取 title
	var title string
	p := titlePattern.FindStringSubmatch(content)
	if len(p) >= 2 {
		title = p[1]
	}

	return &HTTPResult{
		title:      title,
		location:   location,
		statusCode: uint(statusCode),
		bodyLength: uint(len(content)),
		error:      "",
	}

}
