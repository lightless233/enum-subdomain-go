package main

import (
	"github.com/miekg/dns"
	"math/rand"
	"time"
)

type DNSResolveResult struct {
	domain      string
	ARecord     []string
	CNAMERecord []string
}

// BuildDNSClient 构建 DNS Client
func BuildDNSClient() *dns.Client {
	return &dns.Client{Timeout: 2 * time.Second}
}

// DoDNSResolve 执行 DNS 解析
// 返回值：(ARecord, CNAMERecord, Error)
func DoDNSResolve(domain string, dnsClient *dns.Client) (*DNSResolveResult, error) {
	var msg dns.Msg
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	msg.RecursionDesired = true

	// 每次请求的时候，从提供的 ns 中随机取一个
	ns := appArgs.Nameserver[rand.Intn(len(appArgs.Nameserver))]
	response, _, err := dnsClient.Exchange(&msg, ns)

	if err != nil {
		return nil, err
	}

	// 读取 A 记录和 CNAME 记录
	var aRecord []string
	var cnameRecord []string
	for _, answer := range response.Answer {
		if res, ok := answer.(*dns.A); ok {
			aRecord = append(aRecord, res.A.String())
		} else if res, ok := answer.(*dns.CNAME); ok {
			cnameRecord = append(cnameRecord, res.Target)
		}
	}

	return &DNSResolveResult{domain: domain, ARecord: aRecord, CNAMERecord: cnameRecord}, nil
}
