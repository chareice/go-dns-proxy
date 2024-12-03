package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"go-dns-proxy/client"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

type cacheResult struct {
	Domain string
	Result bool
}

type ChinaDomainService struct {
	apiKey         string
	cacheFile      string
	cache          map[string]bool
	cacheMutex     sync.RWMutex
	writeCacheChan chan cacheResult
	cacheInterval  int
	beianAPIURL    string
}

func NewChinaDomainService(apiKey string, beianCacheFile string, cacheInterval int) *ChinaDomainService {
	service := &ChinaDomainService{
		apiKey:         apiKey,
		cacheFile:      beianCacheFile,
		cache:          make(map[string]bool),
		writeCacheChan: make(chan cacheResult),
		cacheInterval:  cacheInterval,
		beianAPIURL:    "https://apidata.chinaz.com/CallAPI/Domain",
	}

	service.initCache()

	return service
}

func (s *ChinaDomainService) initCache() {
	// 确保缓存文件存在
	if _, err := os.Stat(s.cacheFile); os.IsNotExist(err) {
		file, err := os.Create(s.cacheFile)
		if err != nil {
			log.WithError(err).Error("创建缓存文件失败")
		}
		file.Close()
	}

	fileBytes, err := os.ReadFile(s.cacheFile)
	if err != nil {
		log.WithError(err).Error("读取缓存文件失败")
		return
	}

	// 只有当文件不为空时才尝试解析
	if len(fileBytes) > 0 {
		err = json.Unmarshal(fileBytes, &s.cache)
		if err != nil {
			log.WithError(err).Error("解析缓存文件失败")
		}
	}

	// 启动缓存写入协程
	go func() {
		ticker := time.NewTicker(time.Duration(s.cacheInterval) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.cacheMutex.RLock()
			data, err := json.Marshal(s.cache)
			s.cacheMutex.RUnlock()

			if err != nil {
				log.WithError(err).Error("序列化缓存失败")
				continue
			}

			// 原子写入：先写入临时文件，再重命名
			tmpFile := s.cacheFile + ".tmp"
			err = os.WriteFile(tmpFile, data, 0644)
			if err != nil {
				log.WithError(err).Error("写入临时缓存文件失败")
				continue
			}

			err = os.Rename(tmpFile, s.cacheFile)
			if err != nil {
				log.WithError(err).Error("重命名缓存文件失败")
				_ = os.Remove(tmpFile)
			}
		}
	}()

	// 启动缓存更新协程
	go func() {
		for cr := range s.writeCacheChan {
			s.cacheMutex.Lock()
			s.cache[cr.Domain] = cr.Result
			s.cacheMutex.Unlock()
			log.WithFields(log.Fields{
				"domain": cr.Domain,
				"isBeian": cr.Result,
			}).Debug("更新域名备案缓存")
		}
	}()
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

	// 检查是否是直接的中国域名
	domainSuffix := parts[len(parts)-1]
	if domainSuffix == "cn" || domainSuffix == "中国" {
		logger.Debug("直接匹配中国域名后缀")
		return true
	}

	// 检查是否是二级中国域名
	if len(parts) >= 3 {
		secondLevelDomain := parts[len(parts)-2]
		thirdLevelDomain := parts[len(parts)-3]
		if strings.HasSuffix(domain, ".com.cn") || strings.HasSuffix(domain, ".net.cn") || strings.HasSuffix(domain, ".org.cn") {
			logger.Debug("匹配二级中国域名后缀")
			return true
		}
		if secondLevelDomain == "com" || secondLevelDomain == "net" || secondLevelDomain == "org" {
			checkDomain := fmt.Sprintf("%s.%s", thirdLevelDomain, secondLevelDomain)
			logger.WithField("checkDomain", checkDomain).Debug("检查域名备案状态")
			return s.isBeianDomain(ctx, checkDomain)
		}
	}

	logger.Debug("非中国域名")
	return false
}

func (s *ChinaDomainService) isBeianDomain(ctx context.Context, domain string) bool {
	requestID, _ := ctx.Value(client.RequestIDKey).(string)
	logger := log.WithFields(log.Fields{
		"requestId": requestID,
		"domain":    domain,
	})

	// 检查缓存
	s.cacheMutex.RLock()
	if val, ok := s.cache[domain]; ok {
		s.cacheMutex.RUnlock()
		logger.WithField("cached", val).Debug("命中备案缓存")
		return val
	}
	s.cacheMutex.RUnlock()

	logger.Debug("开始查询备案接口")
	startTime := time.Now()

	// 创建带超时的客户端
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 构建请求URL
	reqURL := fmt.Sprintf("%s?key=%s&domainName=%s", s.beianAPIURL, s.apiKey, domain)
	
	// 创建带 context 的请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		logger.WithError(err).Error("创建HTTP请求失败")
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Error("请求备案鉴定接口失败")
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Error("读取响应失败")
		return false
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		logger.WithFields(log.Fields{
			"statusCode": resp.StatusCode,
			"body":      string(body),
		}).Error("API响应状态码错误")
		return false
	}

	result := gjson.GetBytes(body, "StateCode")
	if !result.Exists() {
		logger.WithField("body", string(body)).Error("解析响应失败")
		return false
	}

	code := result.Int()
	isBeian := code == 1

	logger.WithFields(log.Fields{
		"isBeian": isBeian,
		"elapsed": time.Since(startTime).String(),
	}).Debug("备案查询完成")

	// 更新缓存
	s.writeCacheChan <- cacheResult{Domain: domain, Result: isBeian}

	return isBeian
}
