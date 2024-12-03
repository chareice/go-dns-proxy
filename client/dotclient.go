package client

import (
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

func (c *DOTClient) Request(m dnsmessage.Message) ([]byte, error) {
	// 确保服务器地址包含端口，默认为 853
	host, port, err := net.SplitHostPort(c.serverAddr)
	if err != nil {
		host = c.serverAddr
		port = "853"
	}

	logger := log.WithFields(log.Fields{
		"server": fmt.Sprintf("%s:%s", host, port),
		"type":   m.Questions[0].Type,
		"domain": m.Questions[0].Name.String(),
	})
	logger.Debug("发送DOT请求")

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

	// 建立 TLS 连接
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port), &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: host,
		// 跳过证书验证，用于测试
		InsecureSkipVerify: true,
	})
	if err != nil {
		logger.WithError(err).Error("TLS连接失败")
		return nil, fmt.Errorf("TLS连接失败: %v", err)
	}
	defer conn.Close()

	logger.Debug("TLS连接已建立")

	// 设置读写超时
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	// 打包 DNS 消息
	dnsMessage, err := m.Pack()
	if err != nil {
		logger.WithError(err).Error("打包DNS消息失败")
		return nil, fmt.Errorf("打包DNS消息失败: %v", err)
	}

	// 添加两字节的长度前缀
	length := uint16(len(dnsMessage))
	prefixedMessage := make([]byte, 2+len(dnsMessage))
	prefixedMessage[0] = byte(length >> 8)
	prefixedMessage[1] = byte(length)
	copy(prefixedMessage[2:], dnsMessage)

	// 发送请求
	_, err = conn.Write(prefixedMessage)
	if err != nil {
		logger.WithError(err).Error("发送DOT请求失败")
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}

	logger.Debug("请求已发送，等待响应")

	// 读取响应长度
	lengthBytes := make([]byte, 2)
	_, err = io.ReadFull(conn, lengthBytes)
	if err != nil {
		logger.WithError(err).Error("读取DOT响应长度失败")
		return nil, fmt.Errorf("读取响应长度失败: %v", err)
	}
	responseLength := int(lengthBytes[0])<<8 | int(lengthBytes[1])

	// 验证响应长度
	if responseLength <= 0 || responseLength > 65535 {
		logger.WithField("length", responseLength).Error("无效的DOT响应长度")
		return nil, fmt.Errorf("无效的响应长度: %d", responseLength)
	}

	logger.WithField("length", responseLength).Debug("收到响应长度")

	// 读取响应
	response := make([]byte, responseLength)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		logger.WithError(err).Error("读取DOT响应内容失败")
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应以记录日志
	var respMsg dnsmessage.Message
	if err := respMsg.Unpack(response); err == nil {
		logger.WithField("answers", len(respMsg.Answers)).Debug("收到DOT响应")
	}

	return response, nil
} 