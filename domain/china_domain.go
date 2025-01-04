package domain

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// ChinaDomainService 用于检测中国域名
type ChinaDomainService struct {
	pinyinService *PinyinDomainService
	chinaDomains map[string]bool
	mu           sync.RWMutex
}

// NewChinaDomainService 创建一个新的中国域名检测服务
func NewChinaDomainService() *ChinaDomainService {
	return &ChinaDomainService{
		pinyinService: NewPinyinDomainService(),
		chinaDomains: make(map[string]bool),
	}
}

// LoadChinaDomainList 从文件加载中国域名列表
func (s *ChinaDomainService) LoadChinaDomainList(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "server=") {
			// 解析格式：server=/example.com/114.114.114.114
			parts := strings.Split(line, "/")
			if len(parts) >= 2 {
				domain := strings.TrimPrefix(parts[1], ".")
				s.chinaDomains[domain] = true
			}
		}
	}

	log.WithField("count", len(s.chinaDomains)).Info("已加载中国域名列表")
	return scanner.Err()
}

// DownloadAndLoadChinaDomainList 下载并加载中国域名列表
func (s *ChinaDomainService) DownloadAndLoadChinaDomainList(url string, dataDir string) error {
	// 确保目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	// 本地文件路径
	localFile := filepath.Join(dataDir, "china_domains.txt")

	// 如果本地文件存在，直接加载
	if _, err := os.Stat(localFile); err == nil {
		log.Info("使用本地中国域名列表")
		return s.LoadChinaDomainList(localFile)
	}

	// 本地文件不存在时才下载
	log.WithField("url", url).Info("本地文件不存在，开始下载中国域名列表")

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 下载文件
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 创建临时文件
	tmpFile := localFile + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer out.Close()

	// 复制内容到临时文件
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tmpFile)
		return err
	}

	// 关闭临时文件
	out.Close()

	// 重命名临时文件
	if err := os.Rename(tmpFile, localFile); err != nil {
		os.Remove(tmpFile)
		return err
	}

	log.Info("中国域名列表下载完成")
	return s.LoadChinaDomainList(localFile)
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

// isDomainInList 检查域名是否在中国域名列表中
func (s *ChinaDomainService) isDomainInList(domain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 检查完整域名
	if s.chinaDomains[domain] {
		return true
	}

	// 检查父域名
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		parentDomain := strings.Join(parts[i:], ".")
		if s.chinaDomains[parentDomain] {
			return true
		}
	}

	return false
}

// IsChinaDomain 检查是否为中国域名
func (s *ChinaDomainService) IsChinaDomain(ctx context.Context, domain string) bool {
	logger := log.WithField("domain", domain)

	// 移除末尾的点
	domain = strings.TrimSuffix(domain, ".")

	// 检查是否在中国域名列表中
	if s.isDomainInList(domain) {
		logger.Debug("域名在中国域名列表中")
		return true
	}

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
