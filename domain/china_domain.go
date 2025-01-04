package domain

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ChinaDomainService 用于检测中国域名
type ChinaDomainService struct {
	pinyinService *PinyinDomainService
}

// NewChinaDomainService 创建一个新的中国域名检测服务
func NewChinaDomainService() *ChinaDomainService {
	return &ChinaDomainService{
		pinyinService: NewPinyinDomainService(),
	}
}

// extractMainDomain 提取主域名（二级域名）
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
				// 返回三级域名
				return parts[len(parts)-3]
			}
			return ""
		}
	}

	// 返回二级域名
	return parts[len(parts)-2]
}

// IsChinaDomain 检查是否为中国域名
func (s *ChinaDomainService) IsChinaDomain(ctx context.Context, domain string) bool {
	logger := log.WithField("domain", domain)

	// 移除末尾的点
	domain = strings.TrimSuffix(domain, ".")

	// 检查是否为中国顶级域名
	if strings.HasSuffix(domain, ".cn") || strings.HasSuffix(domain, ".中国") {
		logger.Debug("中国顶级域名")
		return true
	}

	// 提取主域名
	mainDomain := extractMainDomain(domain)
	if mainDomain == "" {
		return false
	}

	// 检查主域名是否为拼音
	isPinyin := s.pinyinService.IsPinyinDomain(mainDomain)
	logger.WithFields(log.Fields{
		"mainDomain": mainDomain,
		"isPinyin": isPinyin,
	}).Debug("拼音域名检查结果")

	return isPinyin
}

// Close 关闭服务
func (s *ChinaDomainService) Close() error {
	return nil
}
