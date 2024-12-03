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
	ApiKey          string
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
		chinaDomainService: domain.NewChinaDomainService(options.ApiKey, db),
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

			go s.handleDNSMessage(remoteAddr, buffer[:n], uuid.New().String())
		}
	}
}

func (s *DnsServer) handleDNSMessage(senderAddr *net.UDPAddr, message []byte, requestID string) {
	startTime := time.Now()
	var m dnsmessage.Message
	err := m.Unpack(message)
	if err != nil {
		log.WithError(err).Error("解析DNS消息失败")
		return
	}

	logger := log.WithFields(log.Fields{
		"requestId": requestID,
		"client":    senderAddr.String(),
		"domain":    m.Questions[0].Name.String(),
		"type":      m.Questions[0].Type,
		"class":     m.Questions[0].Class,
		"msgId":     m.Header.ID,
	})

	defer func() {
		logger.WithField("elapsed", time.Since(startTime).String()).Debug("DNS请求处理完成")
	}()

	// 创建带取消的 context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 将 requestID 添加到 context
	ctx = context.WithValue(ctx, client.RequestIDKey, requestID)

	queryQuestion := m.Questions[0]
	domainName := queryQuestion.Name.String()

	var resolver client.DNSResolver
	var isChinaDNS bool
	resolver = s.overseaResolver

	resolveStartTime := time.Now()
	if s.chinaDomainService.IsChinaDomain(ctx, domainName) {
		resolver = s.chinaResolver
		isChinaDNS = true
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

	// 格式化响应内容
	var answers []map[string]interface{}
	for _, answer := range respM.Answers {
		answerMap := map[string]interface{}{
			"name": answer.Header.Name.String(),
			"type": answer.Header.Type.String(),
			"ttl":  answer.Header.TTL,
		}

		// 根据记录类型解析响应内容
		switch answer.Header.Type {
		case dnsmessage.TypeA:
			if a, ok := answer.Body.(*dnsmessage.AResource); ok {
				ip := net.IP(a.A[:])
				answerMap["data"] = ip.String()
			}
		case dnsmessage.TypeAAAA:
			if aaaa, ok := answer.Body.(*dnsmessage.AAAAResource); ok {
				ip := net.IP(aaaa.AAAA[:])
				answerMap["data"] = ip.String()
			}
		case dnsmessage.TypeCNAME:
			if cname, ok := answer.Body.(*dnsmessage.CNAMEResource); ok {
				answerMap["data"] = cname.CNAME.String()
			}
		case dnsmessage.TypeMX:
			if mx, ok := answer.Body.(*dnsmessage.MXResource); ok {
				answerMap["data"] = fmt.Sprintf("%d %s", mx.Pref, mx.MX.String())
			}
		case dnsmessage.TypeTXT:
			if txt, ok := answer.Body.(*dnsmessage.TXTResource); ok {
				answerMap["data"] = strings.Join(txt.TXT, " ")
			}
		default:
			answerMap["data"] = "unsupported record type"
		}
		answers = append(answers, answerMap)
	}

	// 记录查询信息到数据库
	query := &admin.DNSQuery{
		RequestID:    requestID,
		Domain:      domainName,
		QueryType:   queryQuestion.Type.String(),
		ClientIP:    senderAddr.IP.String(),
		Server:      resolver.String(),
		IsChinaDNS:  isChinaDNS,
		ResponseCode: int(respM.Header.RCode),
		AnswerCount: len(respM.Answers),
		TotalTime:   time.Since(startTime).Milliseconds(),
		CreatedAt:   startTime,
		Answers:     answers,
	}
	if err := admin.SaveDNSQuery(s.db, query); err != nil {
		logger.WithError(err).Error("保存查询记录失败")
	}

	logger.WithFields(log.Fields{
		"answers":     len(respM.Answers),
		"authorities": len(respM.Authorities),
		"additionals": len(respM.Additionals),
		"rcode":      respM.Header.RCode,
		"resolveTime": resolveElapsed.String(),
		"answerDetails": answers,
	}).Debug("DNS解析完成")

	_, err = s.listenConn.WriteToUDP(resp, senderAddr)
	if err != nil {
		logger.WithError(err).Error("发送DNS响应失败")
	}
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
