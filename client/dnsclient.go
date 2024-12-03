package client

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/dns/dnsmessage"
)

type DNSClient struct {
	serverAddr string
}

type contextKey string

const (
	RequestIDKey contextKey = "requestID"
)

func NewDNSClient(serverAddr string) *DNSClient {
	log.WithField("server", serverAddr).Debug("创建DNS客户端")
	return &DNSClient{
		serverAddr: serverAddr,
	}
}

func (c *DNSClient) Request(ctx context.Context, m dnsmessage.Message) ([]byte, error) {
	// 从 context 获取 requestID
	requestID, _ := ctx.Value(RequestIDKey).(string)

	// 确保服务器地址包含端口
	host, port, err := net.SplitHostPort(c.serverAddr)
	if err != nil {
		host = c.serverAddr
			port = "53"
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
	logger.Debug("准备发送DNS请求")

	// 验证端口号
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum <= 0 || portNum > 65535 {
		logger.WithField("port", port).Error("无效的端口号")
		return nil, fmt.Errorf("无效的端口号: %s", port)
	}

	// 创建 UDP 连接
	dialer := net.Dialer{
		Timeout: 2 * time.Second,
	}
	logger.Debug("开始建立UDP连接")

	// 使用 context 控制连接超时
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(host, port))
	if err != nil {
		logger.WithError(err).Error("连接DNS服务器失败")
		return nil, fmt.Errorf("连接失败: %v", err)
	}
	defer conn.Close()
	logger.Debug("UDP连接已建立")

	// 设置读写超时
	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(2 * time.Second))
	}

	// 打包 DNS 消息
	packed, err := m.Pack()
	if err != nil {
		logger.WithError(err).Error("打包DNS消息失败")
		return nil, fmt.Errorf("打包DNS消息失败: %v", err)
	}
	logger.WithField("messageSize", len(packed)).Debug("DNS消息打包完成")

	// 发送请求
	startTime := time.Now()
	_, err = conn.Write(packed)
	if err != nil {
		logger.WithError(err).Error("发送DNS请求失败")
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	logger.Debug("DNS请求已发送，等待响应")

	// 读取响应
	response := make([]byte, 512)
	n, err := conn.Read(response)
	if err != nil {
		logger.WithError(err).Error("读取DNS响应失败")
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	elapsed := time.Since(startTime)
	logger.WithField("responseSize", n).WithField("elapsed", elapsed.String()).Debug("收到DNS响应")

	// 解析响应以记录日志
	var respMsg dnsmessage.Message
	if err := respMsg.Unpack(response[:n]); err == nil {
		logger.WithFields(log.Fields{
			"answers":     len(respMsg.Answers),
			"authorities": len(respMsg.Authorities),
			"additionals": len(respMsg.Additionals),
			"rcode":      respMsg.Header.RCode,
			"truncated":  respMsg.Header.Truncated,
		}).Debug("DNS响应解析完成")
	}

	return response[:n], nil
} 