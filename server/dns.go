package server

import (
	"context"
	"database/sql"
	"fmt"
	"go-dns-proxy/admin"
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
	listenConn         *net.UDPConn
	chinaResolver      client.DNSResolver
	overseaResolver    client.DNSResolver
	chinaDomainService *domain.ChinaDomainService
	db                 *sql.DB
	mu                 sync.RWMutex
	stopChan          chan struct{}
}

type NewServerOptions struct {
	ListenPort      int
	ChinaServerAddr string
	OverSeaServerAddr string
	DBPath          string
}

func NewDnsServer(options *NewServerOptions) (*DnsServer, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: options.ListenPort, IP: net.ParseIP("0.0.0.0")})
	if err != nil {
		return nil, err
	}

	db, err := admin.InitDB(options.DBPath)
	if err != nil {
		conn.Close()
		return nil, err
	}

	var chinaResolver, overseaResolver client.DNSResolver

	// 检查服务器类型
	chinaResolver = createResolver(options.ChinaServerAddr)
	overseaResolver = createResolver(options.OverSeaServerAddr)

	return &DnsServer{
		listenConn:         conn,
		chinaResolver:      chinaResolver,
		overseaResolver:    overseaResolver,
		chinaDomainService: domain.NewChinaDomainService(),
		db:                 db,
		stopChan:          make(chan struct{}),
	}, nil
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
		return client.NewUDPClient(addr)
	}
}

func (s *DnsServer) Start() {
	log.Info("DNS服务器启动")
	buffer := make([]byte, 512)

	for {
		select {
		case <-s.stopChan:
			return
		default:
			s.listenConn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, remoteAddr, err := s.listenConn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.WithError(err).Error("读取UDP数据失败")
				continue
			}

			go s.handleDNSQuery(remoteAddr, buffer[:n])
		}
	}
}

func (s *DnsServer) handleDNSQuery(senderAddr *net.UDPAddr, queryData []byte) {
	startTime := time.Now()
	requestID := uuid.New().String()
	logger := log.WithFields(log.Fields{
		"requestId": requestID,
		"clientIp": senderAddr.IP.String(),
	})

	// 解析 DNS 查询
	var queryMsg dnsmessage.Message
	if err := queryMsg.Unpack(queryData); err != nil {
		logger.WithError(err).Error("解析 DNS 查询失败")
		return
	}

	if len(queryMsg.Questions) == 0 {
		logger.Error("DNS 查询中没有问题")
		return
	}

	queryQuestion := queryMsg.Questions[0]
	domain := strings.TrimSuffix(queryQuestion.Name.String(), ".")
	logger = logger.WithFields(log.Fields{
		"domain": domain,
		"type":   queryQuestion.Type.String(),
	})

	// 判断是否使用中国 DNS
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ctx = context.WithValue(ctx, client.RequestIDKey, requestID)
	defer cancel()

	isChinaDNS := s.chinaDomainService.IsChinaDomain(ctx, domain)
	var resolver client.DNSResolver
	if isChinaDNS {
		resolver = s.chinaResolver
		logger.Debug("使用中国 DNS 服务器")
	} else {
		resolver = s.overseaResolver
		logger.Debug("使用海外 DNS 服务器")
	}

	// 发送查询
	respData, err := resolver.Request(ctx, queryMsg)
	if err != nil {
		logger.WithError(err).Error("DNS 查询失败")
		return
	}

	// 解析响应
	var respMsg dnsmessage.Message
	if err := respMsg.Unpack(respData); err != nil {
		logger.WithError(err).Error("解析 DNS 响应失败")
		return
	}

	// 发送响应
	if _, err := s.listenConn.WriteToUDP(respData, senderAddr); err != nil {
		logger.WithError(err).Error("发送 DNS 响应失败")
		return
	}

	// 提取 answers
	var answers []string
	for _, answer := range respMsg.Answers {
		switch answer.Body.(type) {
		case *dnsmessage.AResource:
			a := answer.Body.(*dnsmessage.AResource)
			ip := net.IP(a.A[:])
			answers = append(answers, ip.String())
		case *dnsmessage.AAAAResource:
			aaaa := answer.Body.(*dnsmessage.AAAAResource)
			ip := net.IP(aaaa.AAAA[:])
			answers = append(answers, ip.String())
		case *dnsmessage.CNAMEResource:
			cname := answer.Body.(*dnsmessage.CNAMEResource)
			answers = append(answers, cname.CNAME.String())
		case *dnsmessage.MXResource:
			mx := answer.Body.(*dnsmessage.MXResource)
			answers = append(answers, fmt.Sprintf("%d %s", mx.Pref, mx.MX.String()))
		case *dnsmessage.NSResource:
			ns := answer.Body.(*dnsmessage.NSResource)
			answers = append(answers, ns.NS.String())
		case *dnsmessage.PTRResource:
			ptr := answer.Body.(*dnsmessage.PTRResource)
			answers = append(answers, ptr.PTR.String())
		case *dnsmessage.TXTResource:
			txt := answer.Body.(*dnsmessage.TXTResource)
			answers = append(answers, strings.Join(txt.TXT, " "))
		}
	}

	// 保存查询记录
	dnsQuery := &admin.DNSQuery{
		RequestID:    requestID,
		Domain:      domain,
		QueryType:   queryQuestion.Type.String(),
		ClientIP:    senderAddr.IP.String(),
		Server:      resolver.String(),
		IsChinaDNS:  isChinaDNS,
		ResponseCode: int(respMsg.Header.RCode),
		AnswerCount: len(respMsg.Answers),
		TotalTimeMs: float64(time.Since(startTime).Microseconds()) / 1000.0, // 转换为毫秒的浮点数
		CreatedAt:   startTime,
		Answers:     answers,
	}
	if err := admin.SaveDNSQuery(s.db, dnsQuery); err != nil {
		logger.WithError(err).Error("保存查询记录失败")
	}

	logger.WithFields(log.Fields{
		"answers":     len(respMsg.Answers),
		"totalTimeMs": dnsQuery.TotalTimeMs,
		"isChinaDNS": isChinaDNS,
	}).Info("DNS 查询完成")
}

func (s *DnsServer) GetDB() *sql.DB {
	return s.db
}

func (s *DnsServer) Close() error {
	log.Info("正在关闭DNS服务器...")
	
	// 发送停止信号
	close(s.stopChan)

	// 关闭 UDP 连接
	if err := s.listenConn.Close(); err != nil {
		log.WithError(err).Error("关闭UDP连接失败")
	}

	// 关闭备案服务
	if err := s.chinaDomainService.Close(); err != nil {
		log.WithError(err).Error("关闭备案服务失败")
	}

	// 关闭数据库连接
	if err := s.db.Close(); err != nil {
		log.WithError(err).Error("关闭数据库连接失败")
	}

	log.Info("DNS服务器已关闭")
	return nil
}
