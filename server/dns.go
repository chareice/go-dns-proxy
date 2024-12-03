package server

import (
	"context"
	"go-dns-proxy/client"
	"go-dns-proxy/domain"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
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
	log.Debug("DNS服务器开始监听请求")
	for {
		message := make([]byte, 512)
		n, senderAddr, err := s.listenConn.ReadFromUDP(message)
		if err != nil {
			log.WithError(err).Error("读取UDP消息失败")
			continue
		}

		var m dnsmessage.Message
		err = m.Unpack(message[:n])
		if err != nil {
			log.WithError(err).Error("解析DNS消息失败")
			continue
		}

		requestID := uuid.New().String()
		log.WithFields(log.Fields{
			"requestId": requestID,
			"client":    senderAddr.String(),
			"messageId": m.Header.ID,
			"questions": len(m.Questions),
		}).Debug("收到DNS查询请求")

		go s.handleDNSMessage(senderAddr, m, requestID)
	}
}

func (s *DnsServer) handleDNSMessage(senderAddr *net.UDPAddr, m dnsmessage.Message, requestID string) {
	startTime := time.Now()
	defer func() {
		log.WithFields(log.Fields{
			"requestId": requestID,
			"domain":    m.Questions[0].Name.String(),
			"elapsed":   time.Since(startTime).String(),
		}).Debug("DNS请求处理完成")
	}()

	// 创建带取消的 context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 将 requestID 添加到 context
	ctx = context.WithValue(ctx, client.RequestIDKey, requestID)

	queryQuestion := m.Questions[0]
	domainName := queryQuestion.Name.String()

	logger := log.WithFields(log.Fields{
		"requestId": requestID,
		"client":    senderAddr.String(),
		"domain":    domainName,
		"type":      queryQuestion.Type,
		"class":     queryQuestion.Class,
		"msgId":     m.Header.ID,
	})

	var resolver client.DNSResolver
	resolver = s.overseaResolver

	resolveStartTime := time.Now()
	if s.chinaDomainService.IsChinaDomain(ctx, domainName) {
		resolver = s.chinaResolver
		logger.Debug("使用国内DNS服务器解析")
	} else {
		logger.Debug("使用海外DNS服务器解析")
	}

	resp, err := resolver.Request(ctx, m)
	resolveElapsed := time.Since(resolveStartTime)
	if err != nil {
		logger.WithError(err).WithField("resolveTime", resolveElapsed.String()).Error("DNS解析失败")
		return
	}

	var respM dnsmessage.Message
	err = respM.Unpack(resp)
	if err != nil {
		logger.WithError(err).Error("解析DNS响应失败")
		return
	}

	logger.WithFields(log.Fields{
		"answers":     len(respM.Answers),
		"authorities": len(respM.Authorities),
		"additionals": len(respM.Additionals),
		"rcode":      respM.Header.RCode,
		"resolveTime": resolveElapsed.String(),
	}).Debug("DNS解析完成")

	_, err = s.listenConn.WriteToUDP(resp, senderAddr)
	if err != nil {
		logger.WithError(err).Error("发送DNS响应失败")
	}
}
