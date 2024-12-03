package domain

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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
			log.Printf("创建缓存文件失败: %v", err)
		}
		file.Close()
	}

	fileBytes, err := ioutil.ReadFile(s.cacheFile)
	if err != nil {
		log.Printf("读取缓存文件失败: %v", err)
		return
	}

	// 只有当文件不为空时才尝试解析
	if len(fileBytes) > 0 {
		err = json.Unmarshal(fileBytes, &s.cache)
		if err != nil {
			log.Printf("解析缓存文件失败: %v", err)
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
				log.Printf("序列化缓存失败: %v", err)
				continue
			}

			// 原子写入：先写入临时文件，再重命名
			tmpFile := s.cacheFile + ".tmp"
			err = ioutil.WriteFile(tmpFile, data, 0644)
			if err != nil {
				log.Printf("写入临时缓存文件失败: %v", err)
				continue
			}

			err = os.Rename(tmpFile, s.cacheFile)
			if err != nil {
				log.Printf("重命名缓存文件失败: %v", err)
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
		}
	}()
}

// IsChinaDomain 判断域名是否为国内域名
func (s *ChinaDomainService) IsChinaDomain(domain string) bool {
	// 移除末尾的点
	domain = strings.TrimSuffix(domain, ".")
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	// 检查是否是直接的中国域名
	domainSuffix := parts[len(parts)-1]
	if domainSuffix == "cn" || domainSuffix == "中国" {
		return true
	}

	// 检查是否是二级中国域名
	if len(parts) >= 3 {
		secondLevelDomain := parts[len(parts)-2]
		thirdLevelDomain := parts[len(parts)-3]
		if strings.HasSuffix(domain, ".com.cn") || strings.HasSuffix(domain, ".net.cn") || strings.HasSuffix(domain, ".org.cn") {
			return true
		}
		if secondLevelDomain == "com" || secondLevelDomain == "net" || secondLevelDomain == "org" {
			return s.isBeianDomain(fmt.Sprintf("%s.%s", thirdLevelDomain, secondLevelDomain))
		}
	}

	return false
}

func (s *ChinaDomainService) isBeianDomain(domain string) bool {
	// 检查缓存
	s.cacheMutex.RLock()
	if val, ok := s.cache[domain]; ok {
		s.cacheMutex.RUnlock()
		return val
	}
	s.cacheMutex.RUnlock()

	// 创建带超时的客户端
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 构建请求URL
	reqURL := fmt.Sprintf("%s?key=%s&domainName=%s", s.beianAPIURL, s.apiKey, domain)
	resp, err := client.Get(reqURL)
	if err != nil {
		log.Printf("请求备案鉴定接口失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应失败: %v", err)
		return false
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("API响应状态码错误: %d, body: %s", resp.StatusCode, string(body))
		return false
	}

	result := gjson.GetBytes(body, "StateCode")
	if !result.Exists() {
		log.Printf("解析响应失败: %s", string(body))
		return false
	}

	code := result.Int()
	isBeian := code == 1

	// 更新缓存
	s.writeCacheChan <- cacheResult{Domain: domain, Result: isBeian}

	return isBeian
}
