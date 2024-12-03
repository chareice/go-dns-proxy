package server

import (
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestCreateResolver(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "Test UDP DNS",
			addr:    "8.8.8.8",
			wantErr: false,
		},
		{
			name:    "Test UDP DNS with port",
			addr:    "8.8.8.8:53",
			wantErr: false,
		},
		{
			name:    "Test DOH",
			addr:    "https://1.1.1.1/dns-query",
			wantErr: false,
		},
		{
			name:    "Test DOT",
			addr:    "tls://1.1.1.1:853",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := createResolver(tt.addr)
			if resolver == nil {
				t.Error("createResolver() returned nil")
				return
			}

			// 测试解析器是否可用
			var msg dnsmessage.Message
			msg.Header.ID = 1234
			msg.Header.RecursionDesired = true
			msg.Questions = []dnsmessage.Question{
				{
					Name:  dnsmessage.MustNewName("www.google.com."),
					Type:  dnsmessage.TypeA,
					Class: dnsmessage.ClassINET,
				},
			}

			resp, err := resolver.Request(msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolver.Request() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(resp) == 0 {
				t.Error("resolver.Request() returned empty response")
			}
		})
	}
}

func TestDnsServer_Integration(t *testing.T) {
	// 创建临时缓存文件
	tmpfile, err := os.CreateTemp("", "beian_cache_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// 创建服务器
	server := NewDnsServer(&NewServerOptions{
		ListenPort:         15353, // 使用非标准端口避免冲突
		BeianCacheFile:     tmpfile.Name(),
		BeianCacheInterval: 1,
		ChinaServerAddr:    "114.114.114.114:53",
		OverSeaServerAddr:  "8.8.8.8:53",
		ApiKey:             "test_api_key",
	})

	// 启动服务器
	go server.Start()

	// 等待服务器启动
	time.Sleep(time.Second)

	// 创建测试客户端
	conn, err := net.Dial("udp", "127.0.0.1:15353")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// 构建测试查询
	var msg dnsmessage.Message
	msg.Header.ID = 1234
	msg.Header.RecursionDesired = true
	msg.Questions = []dnsmessage.Question{
		{
			Name:  dnsmessage.MustNewName("www.baidu.com."),
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		},
	}

	// 发送查询
	packed, err := msg.Pack()
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.Write(packed)
	if err != nil {
		t.Fatal(err)
	}

	// 接收响应
	response := make([]byte, 512)
	n, err := conn.Read(response)
	if err != nil {
		t.Fatal(err)
	}

	// 解析响应
	var respMsg dnsmessage.Message
	err = respMsg.Unpack(response[:n])
	if err != nil {
		t.Fatal(err)
	}

	// 验证响应
	if len(respMsg.Answers) == 0 {
		t.Error("No answers in response")
	}
} 