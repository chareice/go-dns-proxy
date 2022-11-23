package server

import (
	"fmt"
	"go-dns-server/client"
	"go-dns-server/domain"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"sync"
)

type DnsServer struct {
	listenConn *net.UDPConn

	chinaDOHClient     *client.DOHClient
	overseaDOHClient   *client.DOHClient
	chinaDomainService *domain.ChinaDomainService
	mu                 sync.RWMutex
}

type NewServerOptions struct {
	ListenPort          int
	BeianCacheFile      string
	BeianCacheInterval  int
	ChinaDOHServerUrl   string
	OverSeaDOHServerUrl string
	ApiKey              string
}

func NewDnsServer(options *NewServerOptions) *DnsServer {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: options.ListenPort, IP: net.ParseIP("0.0.0.0")})

	if err != nil {
		log.Fatal(err)
	}

	return &DnsServer{
		listenConn:         conn,
		chinaDOHClient:     client.NewDOHClient(options.ChinaDOHServerUrl),
		overseaDOHClient:   client.NewDOHClient(options.OverSeaDOHServerUrl),
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

	var dohClient *client.DOHClient

	dohClient = s.overseaDOHClient

	if s.chinaDomainService.IsChinaDomain(domainName.String()) {
		dohClient = s.chinaDOHClient
	}

	resp, err := dohClient.Request(m)

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

	fmt.Println(respM)

	_, err = s.listenConn.WriteToUDP(resp, senderAddr)
}
