package domain

import (
	"context"
	"database/sql"
	"fmt"
	"go-dns-proxy/admin"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
	// 创建临时数据库文件
	tmpDir, err := os.MkdirTemp("", "dns_test_*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := admin.InitDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	return db
}

func TestChinaDomainService_IsChinaDomain(t *testing.T) {
	db := setupTestDB(t)

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

	service := NewChinaDomainService("test_api_key", db)
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
	db := setupTestDB(t)

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

	service := NewChinaDomainService("test_api_key", db)
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
		if isBeian, found := admin.GetBeianCache(db, d.domain); !found || isBeian != d.want {
			t.Errorf("Cache for %s = %v, want %v", d.domain, isBeian, d.want)
		}
	}

	// 测试缓存持久化
	service2 := NewChinaDomainService("test_api_key", db)
	for _, d := range domains {
		if isBeian, found := admin.GetBeianCache(db, d.domain); !found || isBeian != d.want {
			t.Errorf("Persisted cache for %s = %v, want %v", d.domain, isBeian, d.want)
		}
	}
} 