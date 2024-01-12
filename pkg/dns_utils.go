package enum_subdomain

import (
	"github.com/miekg/dns"
	"math/rand"
	"slices"
	"time"
)

type DNSResolveResult struct {
	domain      string
	ARecord     []string
	CNAMERecord []string
}

type DNSClient struct {
	client      *dns.Client
	nameservers []string
}

func NewDNSClient(ns []string) *DNSClient {
	return &DNSClient{
		client:      &dns.Client{Timeout: 2 * time.Second},
		nameservers: ns,
	}
}

// CheckNSConnection 检查指定的 NS 是否可以连通
func (d *DNSClient) CheckNSConnection(ns string, msg *dns.Msg) bool {
	// 构造 query 消息
	if msg == nil {
		msg = &dns.Msg{}
		msg.SetQuestion(dns.Fqdn("www.baidu.com"), dns.TypeA)
		msg.RecursionDesired = true
	}

	for i := 3; i > 0; i-- {
		_, _, err := d.client.Exchange(msg, ns)
		if err == nil {
			return true
		}
	}

	return false
}

// RemoveUnconnectedNS 检查自身的 ns 列表，移除无法连接的 ns，返回被移除的ns列表
func (d *DNSClient) RemoveUnconnectedNS() ([]string, []string) {
	connected := make([]string, 0, len(d.nameservers))
	unconnected := make([]string, 0)
	for _, ns := range d.nameservers {
		if d.CheckNSConnection(ns, nil) {
			connected = append(connected, ns)
		} else {
			unconnected = append(unconnected, ns)
		}
	}
	d.nameservers = connected
	return unconnected, connected
}

func (d *DNSClient) CheckDomainWildcard(domain string) bool {
	domains := []string{"this-domain-will-never-exist", "neither-will-this-one", RandString(8)}
	wildcardResults := make([]bool, 0, len(domains))

	for _, domain := range domains {
		result, err := d.DoDNSResolve(domain)
		if err != nil {
			continue
		}

		hasRecord := len(result.ARecord) != 0 || len(result.CNAMERecord) != 0
		wildcardResults = append(wildcardResults, hasRecord)
	}

	// 如果最终结果里没有 false，即所有的域名都有解析记录
	// 那么大概率是存在泛解析的
	if !slices.Contains(wildcardResults, false) {
		return true
	}

	return false
}

// DoDNSResolve 执行 DNS 解析
// 返回值：(ARecord, CNAMERecord, Error)
func (d *DNSClient) DoDNSResolve(domain string) (*DNSResolveResult, error) {
	var msg dns.Msg
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	msg.RecursionDesired = true

	// 每次请求的时候，从提供的 ns 中随机取一个
	ns := d.nameservers[rand.Intn(len(d.nameservers))]
	response, _, err := d.client.Exchange(&msg, ns)

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
