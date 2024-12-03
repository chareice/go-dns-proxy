package server

import (
	"go-dns-proxy/client"
	"go-dns-proxy/domain"
	"log"
	"net"
	"strings"
	"sync"

	"golang.org/x/net/dns/dnsmessage"
)

type DnsServer struct {
	listenConn *net.UDPConn

	chinaResolver     client.DNSResolver
	overseaResolver   client.DNSResolver
	chinaDomainService *domain.ChinaDomainService
	mu                 sync.RWMutex
}

type NewServerOptions struct {
	ListenPort          int
	BeianCacheFile      string
	BeianCacheInterval  int
	ChinaServerAddr     string
	OverSeaServerAddr   string
	ApiKey              string
}

func NewDnsServer(options *NewServerOptions) *DnsServer {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: options.ListenPort, IP: net.ParseIP("0.0.0.0")})

	if err != nil {
		log.Fatal(err)
	}

	var chinaResolver, overseaResolver client.DNSResolver

	// 检查服务器类型
	chinaResolver = createResolver(options.ChinaServerAddr)
	overseaResolver = createResolver(options.OverSeaServerAddr)

	return &DnsServer{
		listenConn:         conn,
		chinaResolver:      chinaResolver,
		overseaResolver:    overseaResolver,
		chinaDomainService: domain.NewChinaDomainService(options.ApiKey, options.BeianCacheFile, options.BeianCacheInterval),
	}
}

// createResolver 根据地址创建对应的解析器
func createResolver(addr string) client.DNSResolver {
	addrLower := strings.ToLower(addr)
	
	switch {
	case strings.HasPrefix(addrLower, "https://"):
		return client.NewDOHClient(addr)
	case strings.HasPrefix(addrLower, "tls://"):
		return client.NewDOTClient(strings.TrimPrefix(addr, "tls://"))
	default:
		// 普通 DNS，添加默认端口
		if !strings.Contains(addr, ":") {
			addr = addr + ":53"
		}
		return client.NewDNSClient(addr)
	}
}

func (s *DnsServer) Start() {
	s.startDNSServer()
}

func (s *DnsServer) startDNSServer() {
	for {
		message := make([]byte, 512)

		_, senderAddr, err := s.listenConn.ReadFromUDP(message)

		if err != nil {
			log.Println(err)
			continue
		}

		var m dnsmessage.Message

		err = m.Unpack(message)

		if err != nil {
			log.Println(err)
			continue
		}

		go s.handleDNSMessage(senderAddr, m)
	}
}

func (s *DnsServer) handleDNSMessage(senderAddr *net.UDPAddr, m dnsmessage.Message) {
	queryQuestion := m.Questions[0]
	domainName := queryQuestion.Name

	var resolver client.DNSResolver

	resolver = s.overseaResolver

	if s.chinaDomainService.IsChinaDomain(domainName.String()) {
		resolver = s.chinaResolver
	}

	resp, err := resolver.Request(m)

	if err != nil {
		log.Println(err)
		return
	}

	var respM dnsmessage.Message

	err = respM.Unpack(resp)

	if err != nil {
		log.Println(err)
		return
	}

	_, err = s.listenConn.WriteToUDP(resp, senderAddr)
}
