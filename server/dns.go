package server

import (
	"go-dns-server/client"
	"go-dns-server/domain"
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

	// 通过检查地址是否包含 http 来判断是否使用 DOH
	isChinaDOH := strings.Contains(strings.ToLower(options.ChinaServerAddr), "http")
	isOverseaDOH := strings.Contains(strings.ToLower(options.OverSeaServerAddr), "http")

	// 处理中国服务器
	if isChinaDOH {
		chinaResolver = client.NewDOHClient(options.ChinaServerAddr)
	} else {
		chinaAddr := options.ChinaServerAddr
		if !strings.Contains(chinaAddr, ":") {
			chinaAddr = chinaAddr + ":53"
		}
		chinaResolver = client.NewDNSClient(chinaAddr)
	}

	// 处理海外服务器
	if isOverseaDOH {
		overseaResolver = client.NewDOHClient(options.OverSeaServerAddr)
	} else {
		overseaAddr := options.OverSeaServerAddr
		if !strings.Contains(overseaAddr, ":") {
			overseaAddr = overseaAddr + ":53"
		}
		overseaResolver = client.NewDNSClient(overseaAddr)
	}

	return &DnsServer{
		listenConn:         conn,
		chinaResolver:      chinaResolver,
		overseaResolver:    overseaResolver,
		chinaDomainService: domain.NewChinaDomainService(options.ApiKey, options.BeianCacheFile, options.BeianCacheInterval),
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
