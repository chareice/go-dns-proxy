package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/dns/dnsmessage"
)

type DOTClient struct {
	serverAddr string
}

func NewDOTClient(serverAddr string) *DOTClient {
	log.WithField("server", serverAddr).Debug("创建DOT客户端")
	return &DOTClient{
		serverAddr: serverAddr,
	}
}

func (c *DOTClient) Request(ctx context.Context, m dnsmessage.Message) ([]byte, error) {
	startTime := time.Now()
	requestID, _ := ctx.Value(RequestIDKey).(string)

	// 确保服务器地址包含端口，默认为 853
	host, port, err := net.SplitHostPort(c.serverAddr)
	if err != nil {
		host = c.serverAddr
		port = "853"
	}

	logger := log.WithFields(log.Fields{
		"requestId":  requestID,
		"server":     fmt.Sprintf("%s:%s", host, port),
		"type":       m.Questions[0].Type,
		"domain":     m.Questions[0].Name.String(),
		"messageId":  m.Header.ID,
		"recursion":  m.Header.RecursionDesired,
		"questions":  len(m.Questions),
	})
	logger.Debug("准备发送DOT请求")

	// 验证端口号
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum <= 0 || portNum > 65535 {
		logger.WithField("port", port).Error("无效的端口号")
		return nil, fmt.Errorf("无效的端口号: %s", port)
	}

	// 设置拨号超时
	dialer := &net.Dialer{
		Timeout: 3 * time.Second,
	}

	logger.Debug("开始建立TLS连接")
	tlsStartTime := time.Now()

	// 建立 TLS 连接
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         host,
		InsecureSkipVerify: true, // 仅用于测试
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port), tlsConfig)
	if err != nil {
		logger.WithError(err).WithField("tlsTime", time.Since(tlsStartTime).String()).Error("TLS连接失败")
		return nil, fmt.Errorf("TLS连接失败: %v", err)
	}
	defer conn.Close()

	connState := conn.ConnectionState()
	logger.WithFields(log.Fields{
		"version":           connState.Version,
		"handshakeComplete": connState.HandshakeComplete,
		"cipherSuite":       connState.CipherSuite,
		"tlsTime":          time.Since(tlsStartTime).String(),
	}).Debug("TLS连接已建立")

	// 设置读写超时
	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(3 * time.Second))
	}

	// 打包 DNS 消息
	packStartTime := time.Now()
	dnsMessage, err := m.Pack()
	if err != nil {
		logger.WithError(err).Error("打包DNS消息失败")
		return nil, fmt.Errorf("打包DNS消息失败: %v", err)
	}
	logger.WithField("packTime", time.Since(packStartTime).String()).Debug("DNS消息打包完成")

	// 添加两字节的长度前缀
	length := uint16(len(dnsMessage))
	prefixedMessage := make([]byte, 2+len(dnsMessage))
	prefixedMessage[0] = byte(length >> 8)
	prefixedMessage[1] = byte(length)
	copy(prefixedMessage[2:], dnsMessage)

	// 发送请求
	writeStartTime := time.Now()
	_, err = conn.Write(prefixedMessage)
	if err != nil {
		logger.WithError(err).WithField("writeTime", time.Since(writeStartTime).String()).Error("发送DOT请求失败")
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	logger.WithFields(log.Fields{
		"messageSize": len(prefixedMessage),
		"writeTime":   time.Since(writeStartTime).String(),
	}).Debug("DOT请求已发送，等待响应")

	// 读取响应
	readStartTime := time.Now()
	lengthBytes := make([]byte, 2)
	_, err = io.ReadFull(conn, lengthBytes)
	if err != nil {
		logger.WithError(err).WithField("readTime", time.Since(readStartTime).String()).Error("读取DOT响应长度失败")
		return nil, fmt.Errorf("读取响应长度失败: %v", err)
	}
	responseLength := int(lengthBytes[0])<<8 | int(lengthBytes[1])

	// 验证响应长度
	if responseLength <= 0 || responseLength > 65535 {
		logger.WithField("length", responseLength).Error("无效的DOT响应长度")
		return nil, fmt.Errorf("无效的响应长度: %d", responseLength)
	}

	// 读取响应内容
	response := make([]byte, responseLength)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		logger.WithError(err).WithField("readTime", time.Since(readStartTime).String()).Error("读取DOT响应内容失败")
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应以记录日志
	var respMsg dnsmessage.Message
	if err := respMsg.Unpack(response); err == nil {
		logger.WithFields(log.Fields{
			"answers":     len(respMsg.Answers),
			"authorities": len(respMsg.Authorities),
			"additionals": len(respMsg.Additionals),
			"rcode":      respMsg.Header.RCode,
			"truncated":  respMsg.Header.Truncated,
			"tlsTime":    time.Since(tlsStartTime).String(),
			"readTime":   time.Since(readStartTime).String(),
			"totalTime":  time.Since(startTime).String(),
			"bodySize":   len(response),
		}).Debug("DOT响应解析完成")
	}

	return response, nil
}

func (c *DOTClient) String() string {
	return "tls://" + c.serverAddr
} 