package domain

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestChinaDomainService_IsChinaDomain(t *testing.T) {
	// 创建临时缓存文件
	tmpfile, err := os.CreateTemp("", "beian_cache_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 解析查询参数
		domain := r.URL.Query().Get("domainName")
		if strings.Contains(domain, "beian") {
			// 模拟已备案域名
			w.Write([]byte(`{"StateCode": 1}`))
		} else {
			// 模拟未备案域名
			w.Write([]byte(`{"StateCode": 0}`))
		}
	}))
	defer mockServer.Close()

	service := NewChinaDomainService("test_api_key", tmpfile.Name(), 10)
	// 修改备案查询 URL 为 mock 服务器地址
	service.beianAPIURL = mockServer.URL

	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		{
			name:   "Test .cn domain",
				domain: "www.gov.cn.",
				want:   true,
		},
		{
			name:   "Test .com.cn domain",
			domain: "www.baidu.com.cn.",
			want:   true,
		},
		{
			name:   "Test .org.cn domain",
			domain: "www.example.org.cn.",
			want:   true,
		},
		{
			name:   "Test .net.cn domain",
			domain: "www.example.net.cn.",
			want:   true,
		},
		{
			name:   "Test .中国 domain",
			domain: "example.中国.",
			want:   true,
		},
		{
			name:   "Test beian domain",
			domain: "www.beian.cn.",
			want:   true,
		},
		{
			name:   "Test non-beian domain",
			domain: "www.google.com.",
			want:   false,
		},
		{
			name:   "Test another international domain",
			domain: "www.example.org.",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if got := service.IsChinaDomain(ctx, tt.domain); got != tt.want {
				t.Errorf("ChinaDomainService.IsChinaDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChinaDomainService_Cache(t *testing.T) {
	// 创建临时缓存文件
	tmpfile, err := os.CreateTemp("", "beian_cache_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domainName")
		if strings.Contains(domain, "beian") {
			w.Write([]byte(`{"StateCode": 1}`))
		} else {
			w.Write([]byte(`{"StateCode": 0}`))
		}
	}))
	defer mockServer.Close()

	service := NewChinaDomainService("test_api_key", tmpfile.Name(), 1)
	service.beianAPIURL = mockServer.URL

	// 测试缓存写入和读取
	domains := []struct {
		domain string
		want   bool
	}{
		{"beian.com", true},
		{"example.com", false},
	}

	for _, d := range domains {
		// 触发备案查询并缓存
		ctx := context.Background()
		result := service.isBeianDomain(ctx, fmt.Sprintf("www.%s.", d.domain))
		if result != d.want {
			t.Errorf("isBeianDomain(%s) = %v, want %v", d.domain, result, d.want)
		}

		// 验证缓存
		if cached, ok := service.cache[d.domain]; !ok || cached != d.want {
			t.Errorf("Cache for %s = %v, want %v", d.domain, cached, d.want)
		}
	}

	// 测试缓存持久化
	service2 := NewChinaDomainService("test_api_key", tmpfile.Name(), 1)
	for _, d := range domains {
		if cached, ok := service2.cache[d.domain]; !ok || cached != d.want {
			t.Errorf("Persisted cache for %s = %v, want %v", d.domain, cached, d.want)
		}
	}
} 