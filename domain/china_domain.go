package domain

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-dns-proxy/client"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// RequestIDKey 是上下文中请求ID的键
const RequestIDKey = client.RequestIDKey

type ChinaDomainService struct {
	apiKey      string
	db          *sql.DB
	beianAPIURL string
}

// extractMainDomain 从域名中提取主域名
func extractMainDomain(domain string) string {
	// 移除末尾的点
	domain = strings.TrimSuffix(domain, ".")
	parts := strings.Split(domain, ".")
	
	if len(parts) < 2 {
		return ""
	}

	// 处理特殊的二级域名后缀
	if len(parts) >= 3 {
		tld := parts[len(parts)-1]          // 顶级域名
		sld := parts[len(parts)-2]          // 二级域名
		specialSuffixes := map[string]bool{
			"com.cn": true,
			"net.cn": true,
			"org.cn": true,
			"gov.cn": true,
			"edu.cn": true,
		}
		
		if specialSuffixes[sld+"."+tld] {
			if len(parts) >= 3 {
				// 返回三级域名 + 特殊后缀
				return parts[len(parts)-3] + "." + sld + "." + tld
			}
			return sld + "." + tld
		}
	}

	// 处理普通域名
	if len(parts) >= 2 {
		// 返回二级域名 + 顶级域名
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}

	return domain
}

func NewChinaDomainService(apiKey string, db *sql.DB) *ChinaDomainService {
	return &ChinaDomainService{
		apiKey:      apiKey,
		db:          db,
		beianAPIURL: "https://apidata.chinaz.com/CallAPI/Domain",
	}
}

func (s *ChinaDomainService) IsChinaDomain(ctx context.Context, domain string) bool {
	requestID, _ := ctx.Value(RequestIDKey).(string)
	logger := log.WithFields(log.Fields{
		"domain": domain,
		"requestId": requestID,
	})

	logger.Debug("域名解析")
	parts := strings.Split(domain, ".")
	logger = logger.WithField("parts", parts)

	// 移除末尾的点
	domain = strings.TrimSuffix(domain, ".")

	// 检查是否为中国顶级域名
	if strings.HasSuffix(domain, ".cn") || strings.HasSuffix(domain, ".中国") {
		return true
	}

	// 提取主域名
	mainDomain := extractMainDomain(domain)
	if mainDomain == "" {
		return false
	}

	logger = logger.WithFields(log.Fields{
		"mainDomain": mainDomain,
		"sld": parts[len(parts)-2],
		"tld": parts[len(parts)-1],
	})
	logger.Debug("检查主域名备案状态")

	// 检查缓存
	var isBeian bool
	err := s.db.QueryRowContext(ctx, "SELECT is_beian FROM beian_cache WHERE domain = ? AND updated_at > datetime('now', '-24 hours')", mainDomain).Scan(&isBeian)
	if err == nil {
		logger.WithFields(log.Fields{
			"domain": mainDomain,
			"isBeian": isBeian,
			"source": "cache",
		}).Debug("从缓存获取备案状态")
		return isBeian
	}

	// 如果没有 API Key，只根据域名后缀判断
	if s.apiKey == "" {
		return false
	}

	logger.Debug("开始查询备案接口")
	// 构建 API URL
	url := fmt.Sprintf("%s?key=%s&domainName=%s", s.beianAPIURL, s.apiKey, mainDomain)
	logger = logger.WithFields(log.Fields{
		"domain": mainDomain,
		"url": url,
	})
	logger.Debug("备案查询请求")

	// 发送 HTTP 请求
	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.WithError(err).Error("创建备案查询请求失败")
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.WithError(err).Error("备案查询请求失败")
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Error("读取备案查询响应失败")
		return false
	}

	// 解析响应
	var result struct {
		StateCode int    `json:"StateCode"`
		Reason    string `json:"Reason"`
		Result    any    `json:"Result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		logger.WithError(err).Error("解析备案查询响应失败")
		return false
	}

	// 判断是否备案
	isBeian = result.StateCode == 1 && result.Result != nil
	logger.WithFields(log.Fields{
		"domain": mainDomain,
		"isBeian": isBeian,
		"elapsed": time.Since(startTime),
		"response": string(body),
	}).Debug("备案查询完成")

	// 更新缓存
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO beian_cache (domain, is_beian, api_response, updated_at) 
		VALUES (?, ?, ?, datetime('now')) 
		ON CONFLICT(domain) DO UPDATE SET 
		is_beian = excluded.is_beian, 
		api_response = excluded.api_response,
		updated_at = excluded.updated_at`,
		mainDomain, isBeian, string(body))
	if err != nil {
		logger.WithError(err).Error("更新备案缓存失败")
	}

	return isBeian
}

func (s *ChinaDomainService) Close() error {
	return nil
}
