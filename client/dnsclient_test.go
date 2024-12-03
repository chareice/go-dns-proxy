package client

import (
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSClient_Request(t *testing.T) {
	tests := []struct {
		name       string
		serverAddr string
		domain     string
		wantErr    bool
	}{
		{
			name:       "Test Google DNS",
			serverAddr: "8.8.8.8:53",
			domain:     "www.google.com.",
			wantErr:    false,
		},
		{
			name:       "Test AliDNS",
			serverAddr: "223.5.5.5:53",
			domain:     "www.baidu.com.",
			wantErr:    false,
		},
		{
			name:       "Invalid Server",
			serverAddr: "1.1.1.1:1",
			domain:     "www.google.com.",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewDNSClient(tt.serverAddr)

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
				t.Errorf("DNSClient.Request() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(resp) == 0 {
				t.Error("DNSClient.Request() returned empty response")
			}

			if !tt.wantErr {
				// 解析响应
				var respMsg dnsmessage.Message
				if err := respMsg.Unpack(resp); err != nil {
					t.Errorf("Failed to unpack response: %v", err)
					return
				}

				// 检查响应是否包含答案
				if len(respMsg.Answers) == 0 {
					t.Error("Response contains no answers")
				}
			}
		})
	}
} 