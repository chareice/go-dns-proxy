package server

import (
	"fmt"
	"go-dns-server/domain"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"sync"
)

type DnsServer struct {
	listenConn         *net.UDPConn
	upstreamAddr       *net.UDPAddr
	queryClient        map[uint16]string
	chinaDomainService *domain.ChinaDomainService
}

func NewDnsServer(listenPort int, apiKey string, beianCacheFile string) *DnsServer {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: listenPort, IP: net.ParseIP("0.0.0.0")})
	if err != nil {
		log.Fatal(err)
	}

	upstreamAddr, err := net.ResolveUDPAddr("udp4", "223.5.5.5:53")
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}

	return &DnsServer{
		listenConn:         conn,
		upstreamAddr:       upstreamAddr,
		queryClient:        make(map[uint16]string),
		chinaDomainService: domain.NewChinaDomainService(apiKey, beianCacheFile),
	}
}

func (s *DnsServer) Start() {
	var wg sync.WaitGroup
	go s.startDNSServer()
	go s.startOverSeaClient()

	wg.Add(1)
	wg.Wait()
}

//func (s *DnsServer) startOverSeaClient() {
//	for {
//		message := make([]byte, 1024)
//
//		_, err := s.overseaConn.Read(message)
//
//		if err != nil && err == io.EOF {
//			log.Fatal(err)
//		}
//
//		if err != nil {
//			log.Println(err)
//			continue
//		}
//
//		var m dnsmessage.Message
//
//		err = m.Unpack(message)
//
//		if err != nil {
//			log.Println(err)
//			continue
//		}
//
//		go s.handleDNSMessage(m)
//	}
//}

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

		if !m.Response {
			// 记录查询ID和客户端地址的映射关系
			// 应当定时清除
			queryId := m.ID
			s.queryClient[queryId] = senderAddr.String()
		}

		go s.handleDNSMessage(m)
	}
}

func (s *DnsServer) handleDNSResponse(m dnsmessage.Message) {
	udpMessage, err := m.Pack()

	if err != nil {
		log.Println(err)
		return
	}

	senderAddrString := s.queryClient[m.ID]
	senderUdpAddr, err := net.ResolveUDPAddr("udp4", senderAddrString)
	if err != nil {
		log.Println(err)
		return
	}

	_, err = s.listenConn.WriteToUDP(udpMessage, senderUdpAddr)

	if err != nil {
		log.Println(err)
		return
	}

	return
}

func (s *DnsServer) handleDNSMessage(m dnsmessage.Message) {
	fmt.Println(m)

	if m.Header.Response {
		s.handleDNSResponse(m)
		return
	}

	queryQuestion := m.Questions[0]
	domainName := queryQuestion.Name

	udpMessage, err := m.Pack()

	if err != nil {
		log.Println(err)
		return
	}

	if s.chinaDomainService.IsChinaDomain(domainName.String()) {
		// 国内域名 转发至国内的DNS服务
		_, err = s.listenConn.WriteToUDP(udpMessage, s.upstreamAddr)

		if err != nil {
			log.Println(err)
			return
		}
	} else {
		// 海外域名 请求海外服务
		resp, err := HandleOverseaDNSQuery(m)

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

		s.handleDNSResponse(respM)
	}
}
