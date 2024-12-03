package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/dns/dnsmessage"
)

type DOHClient struct {
	serverAddr string
	client     *http.Client
}

func NewDOHClient(serverAddr string) *DOHClient {
	return &DOHClient{
		serverAddr: serverAddr,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *DOHClient) Request(ctx context.Context, m dnsmessage.Message) ([]byte, error) {
	startTime := time.Now()
	requestID, _ := ctx.Value(RequestIDKey).(string)
	logger := log.WithFields(log.Fields{
		"requestId": requestID,
		"server":    c.serverAddr,
		"type":      m.Questions[0].Type,
		"domain":    m.Questions[0].Name.String(),
		"messageId": m.Header.ID,
	})

	logger.Debug("准备发送DOH请求")

	// 打包 DNS 消息
	packStartTime := time.Now()
	dnsMessage, err := m.Pack()
	if err != nil {
		logger.WithError(err).Error("打包DNS消息失败")
		return nil, fmt.Errorf("打包DNS消息失败: %v", err)
	}
	logger.WithField("packTime", time.Since(packStartTime).String()).Debug("DNS消息打包完成")

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.serverAddr, bytes.NewReader(dnsMessage))
	if err != nil {
		logger.WithError(err).Error("创建HTTP请求失败")
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	// 发送请求
	httpStartTime := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		logger.WithError(err).WithField("httpTime", time.Since(httpStartTime).String()).Error("发送DOH请求失败")
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		logger.WithFields(log.Fields{
			"statusCode": resp.StatusCode,
			"httpTime":   time.Since(httpStartTime).String(),
		}).Error("DOH请求返回非200状态码")
		return nil, fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	// 读取响应
	readStartTime := time.Now()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).WithField("readTime", time.Since(readStartTime).String()).Error("读取DOH响应失败")
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应以记录日志
	var respMsg dnsmessage.Message
	if err := respMsg.Unpack(body); err == nil {
		logger.WithFields(log.Fields{
			"answers":     len(respMsg.Answers),
			"authorities": len(respMsg.Authorities),
			"additionals": len(respMsg.Additionals),
			"rcode":      respMsg.Header.RCode,
			"httpTime":   time.Since(httpStartTime).String(),
			"totalTime":  time.Since(startTime).String(),
			"bodySize":   len(body),
		}).Debug("DOH响应解析完成")
	}

	return body, nil
}
