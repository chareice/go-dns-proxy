package client

import (
	"os"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestDOTClient_Request(t *testing.T) {
	// 如果设置了 SKIP_NETWORK_TESTS 环境变量，跳过网络测试
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network tests")
	}

	tests := []struct {
		name       string
		serverAddr string
		domain     string
		wantErr    bool
	}{
		{
			name:       "Test Cloudflare DOT",
			serverAddr: "1.1.1.1:853",
			domain:     "www.google.com.",
			wantErr:    false,
		},
		{
			name:       "Test Google DOT",
			serverAddr: "8.8.8.8:853",
			domain:     "www.google.com.",
			wantErr:    false,
		},
		{
			name:       "Test Invalid Port",
			serverAddr: "1.1.1.1:1",
			domain:     "www.google.com.",
			wantErr:    true,
		},
		{
			name:       "Test Invalid Server",
			serverAddr: "invalid.example.com:853",
			domain:     "www.google.com.",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置测试超时
			done := make(chan bool)
			go func() {
				c := NewDOTClient(tt.serverAddr)

				// 构建 DNS 查询消息
				var msg dnsmessage.Message
				msg.Header.ID = 1234
				msg.Header.RecursionDesired = true
				msg.Questions = []dnsmessage.Question{
					{
						Name:  dnsmessage.MustNewName(tt.domain),
						Type:  dnsmessage.TypeA,
						Class: dnsmessage.ClassINET,
					},
				}

				resp, err := c.Request(msg)
				if (err != nil) != tt.wantErr {
					t.Errorf("DOTClient.Request() error = %v, wantErr %v", err, tt.wantErr)
					done <- true
					return
				}

				if !tt.wantErr {
					if len(resp) == 0 {
						t.Error("DOTClient.Request() returned empty response")
						done <- true
						return
					}

					// 解析响应
					var respMsg dnsmessage.Message
					if err := respMsg.Unpack(resp); err != nil {
						t.Errorf("Failed to unpack response: %v", err)
						done <- true
						return
					}

					// 检查响应是否包含答案
					if len(respMsg.Answers) == 0 {
						t.Error("Response contains no answers")
					}
				}
				done <- true
			}()

			// 等待测试完成或超时
			select {
			case <-done:
				// 测试正常完成
			case <-time.After(5 * time.Second):
				t.Error("Test timeout")
			}
		})
	}
} 