package admin

import (
	"time"

	_ "modernc.org/sqlite" // SQLite 驱动程序
)

type DNSQuery struct {
	ID           int64     `json:"id"`
	RequestID    string    `json:"request_id"`
	Domain       string    `json:"domain"`
	QueryType    string    `json:"query_type"`
	ClientIP     string    `json:"client_ip"`
	Server       string    `json:"server"`
	IsChinaDNS   bool      `json:"is_china_dns"`
	ResponseCode int       `json:"response_code"`
	AnswerCount  int       `json:"answer_count"`
	TotalTimeMs  float64   `json:"total_time_ms"`
	CreatedAt    time.Time `json:"created_at"`
	Answers      []string  `json:"answers"`
}

type QueryStats struct {
	TotalQueries      int64         `json:"total_queries"`
	AverageTimeMs     float64       `json:"average_time_ms"`
	ChinaDNSQueries   int64         `json:"china_dns_queries"`
	OverseaDNSQueries int64         `json:"oversea_dns_queries"`
	TopDomains        []DomainCount `json:"top_domains"`
	TopClients        []ClientCount `json:"top_clients"`
}

type DomainCount struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

type ClientCount struct {
	ClientIP string `json:"client_ip"`
	Count    int64  `json:"count"`
} 