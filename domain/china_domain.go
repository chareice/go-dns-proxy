package domain

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type cacheResult struct {
	Domain string
	Result bool
}

type ChinaDomainService struct {
	apiKey         string
	cacheFile      string
	cache          map[string]bool
	writeCacheChan chan cacheResult
}

func NewChinaDomainService(apiKey string, beianCacheFile string) *ChinaDomainService {
	service := &ChinaDomainService{
		apiKey:         apiKey,
		cacheFile:      beianCacheFile,
		writeCacheChan: make(chan cacheResult),
	}

	service.initCache()

	return service
}

func (s *ChinaDomainService) initCache() {
	fileBytes, err := ioutil.ReadFile(s.cacheFile)

	if err != nil {
		log.Println(err)
	}

	err = json.Unmarshal(fileBytes, &s.cache)
	if err != nil {
		log.Println(err)
	}

	if s.cache == nil {
		s.cache = make(map[string]bool)
	}

	go func() {
		f, err := os.OpenFile(s.cacheFile, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Panic(err)
		}

		defer f.Close()

		for {
			time.Sleep(1 * time.Minute)

			fmt.Printf("当前cache: %v\n", s.cache)
			data, err := json.Marshal(&s.cache)

			if err != nil {
				log.Panic("序列化失败")
			}

			_, err = f.WriteAt(data, 0)

			if err != nil {
				log.Println("写入缓存文件失败")
			}
		}
	}()

	go func() {
		for {
			cr := <-s.writeCacheChan
			s.cache[cr.Domain] = cr.Result
		}
	}()
}

// IsChinaDomain 判断域名是否为国内域名
func (s *ChinaDomainService) IsChinaDomain(domain string) bool {
	parts := strings.Split(domain, ".")
	domainSuffix := parts[len(parts)-2]

	if domainSuffix == "cn" {
		return true
	}

	if domainSuffix == "com" || domainSuffix == "net" {
		return s.isBeianDomain(domain)
	}

	return false
}

func (s *ChinaDomainService) isBeianDomain(domain string) bool {
	parts := strings.Split(domain, ".")
	domainSuffix := parts[len(parts)-2]
	domainGroup := parts[len(parts)-3]

	queryDomain := fmt.Sprintf("%s.%s", domainGroup, domainSuffix)

	if val, ok := s.cache[queryDomain]; ok {
		return val
	}

	resp, err := http.Get(fmt.Sprintf("https://apidata.chinaz.com/CallAPI/Domain?key=%s&domainName=%s", s.apiKey, queryDomain))

	if err != nil {
		log.Printf("请求备案鉴定接口失败, %v", err)
		return false
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("获取响应Body失败, %v", err)
		return false
	}

	result := gjson.GetBytes(body, "StateCode")

	code := result.Int()

	if code == 1 || code == 0 {
		cacheRes := false
		if code == 1 {
			cacheRes = true
		}
		s.writeCacheChan <- cacheResult{Domain: queryDomain, Result: cacheRes}
		return cacheRes
	}

	log.Printf("请求结果：%v\n", string(body))
	return false
}
