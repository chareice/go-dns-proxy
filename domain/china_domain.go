package domain

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-dns-proxy/admin"
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
	service := &ChinaDomainService{
		apiKey:      apiKey,
		db:          db,
		beianAPIURL: "https://apidata.chinaz.com/CallAPI/Domain",
	}

	// 启动定期清理过期缓存的协程
	go service.cleanCacheRoutine()

	return service
}

func (s *ChinaDomainService) cleanCacheRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := admin.CleanOldBeianCache(s.db, 24*time.Hour); err != nil {
			log.WithError(err).Error("清理过期备案缓存失败")
		}
	}
}

func (s *ChinaDomainService) IsChinaDomain(ctx context.Context, domain string) bool {
	requestID, _ := ctx.Value(client.RequestIDKey).(string)
	logger := log.WithFields(log.Fields{
		"requestId": requestID,
		"domain":    domain,
	})
	
	// 移除末尾的点
	domain = strings.TrimSuffix(domain, ".")
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		logger.Debug("域名格式无效")
		return false
	}

	logger.WithField("parts", parts).Debug("域名解析")

	// 检查是否是直接的中国域名
	domainSuffix := parts[len(parts)-1]
	if domainSuffix == "cn" || domainSuffix == "中国" {
		logger.Debug("直接匹配中国域名后缀")
		return true
	}

	// 检查是否是二级中国域名
	if len(parts) >= 2 {
		tld := parts[len(parts)-1]          // 顶级域名，如 "com"
		sld := parts[len(parts)-2]          // 二级域名，如 "taobao"
		mainDomain := fmt.Sprintf("%s.%s", sld, tld)

		if strings.HasSuffix(domain, ".com.cn") || strings.HasSuffix(domain, ".net.cn") || strings.HasSuffix(domain, ".org.cn") {
			logger.Debug("匹配二级中国域名后缀")
			return true
		}

		if tld == "com" || tld == "net" || tld == "org" {
			logger.WithFields(log.Fields{
				"mainDomain": mainDomain,
				"sld": sld,
				"tld": tld,
			}).Debug("检查主域名备案状态")
			return s.isBeianDomain(ctx, mainDomain)
		}
	}

	logger.Debug("非中国域名")
	return false
}

func (s *ChinaDomainService) isBeianDomain(ctx context.Context, domain string) bool {
	// 提取主域名
	mainDomain := extractMainDomain(domain)
	if mainDomain == "" {
		return false
	}

	// 检查缓存
	if isBeian, _, found := admin.GetBeianCache(s.db, mainDomain); found {
		return isBeian
	}

	logger := log.WithFields(log.Fields{
		"domain":    mainDomain,
		"requestId": ctx.Value(RequestIDKey),
	})

	logger.Debug("开始查询备案接口")

	// 构建请求URL
	url := fmt.Sprintf("%s?key=%s&domainName=%s", s.beianAPIURL, s.apiKey, mainDomain)
	logger.WithField("url", url).Debug("备案查询请求")

	// 发送请求
	startTime := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		logger.WithError(err).Error("备案查询请求失败")
		return false
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Error("读取备案查询响应失败")
		return false
	}

	// 解析响应
	var result struct {
		StateCode int         `json:"StateCode"`
		Reason    string      `json:"Reason"`
		Result    interface{} `json:"Result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		logger.WithError(err).Error("解析备案查询响应失败")
		return false
	}

	// 判断是否备案
	isBeian := result.StateCode == 1 && result.Result != nil
	logger.WithFields(log.Fields{
		"elapsed":   time.Since(startTime),
		"isBeian":   isBeian,
		"response":  string(body),
	}).Debug("备案查询完成")

	// 保存到缓存
	if err := admin.SaveBeianCache(s.db, mainDomain, isBeian, string(body)); err != nil {
		logger.WithError(err).Error("保存备案缓存失败")
	}

	return isBeian
}

func (s *ChinaDomainService) Close() error {
	return nil
}
