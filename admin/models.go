package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite3 驱动程序
	log "github.com/sirupsen/logrus"
)

type DNSQuery struct {
	ID           int64                    `json:"id"`
	RequestID    string                   `json:"request_id"`
	Domain       string                   `json:"domain"`
	QueryType    string                   `json:"query_type"`
	ClientIP     string                   `json:"client_ip"`
	Server       string                   `json:"server"`       // 使用的上游服务器
	IsChinaDNS   bool                     `json:"is_china_dns"` // 是否使用中国 DNS
	ResponseCode int                      `json:"response_code"`
	AnswerCount  int                      `json:"answer_count"`
	TotalTime    int64                    `json:"total_time_ms"` // 总处理时间（毫秒）
	CreatedAt    time.Time                `json:"created_at"`
	Answers      []map[string]interface{} `json:"answers"`
}

type DomainCount struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

type QueryStats struct {
	TotalQueries      int64         `json:"total_queries"`
	AverageTimeMs     float64       `json:"average_time_ms"`
	ChinaDNSQueries   int64         `json:"china_dns_queries"`
	OverseaDNSQueries int64         `json:"oversea_dns_queries"`
	TopDomains        []DomainCount `json:"top_domains"`
}

type QueryLog struct {
	Timestamp string          `json:"timestamp"`
	Level     string          `json:"level"`
	Message   string          `json:"message"`
	Fields    map[string]any  `json:"fields"`
}

// 添加全局变量
var (
	adminServer *Server
)

// 添加设置服务器实例的函数
func SetAdminServer(server *Server) {
	adminServer = server
}

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// 创建 DNS 查询记录表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS dns_queries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			domain TEXT NOT NULL,
			query_type TEXT NOT NULL,
			client_ip TEXT NOT NULL,
			server TEXT NOT NULL,
			is_china_dns BOOLEAN NOT NULL,
			response_code INTEGER NOT NULL,
			answer_count INTEGER NOT NULL,
			total_time_ms INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			answers TEXT NOT NULL DEFAULT '[]'
		);
		CREATE INDEX IF NOT EXISTS idx_dns_queries_created_at ON dns_queries(created_at);
		CREATE INDEX IF NOT EXISTS idx_dns_queries_domain ON dns_queries(domain);

		CREATE TABLE IF NOT EXISTS dns_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			fields TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_dns_logs_timestamp ON dns_logs(timestamp);

		CREATE TABLE IF NOT EXISTS beian_cache (
			domain TEXT PRIMARY KEY,
			is_beian BOOLEAN NOT NULL,
			api_response TEXT NOT NULL DEFAULT '{}',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_beian_cache_updated_at ON beian_cache(updated_at);
	`)

	return db, err
}

func SaveDNSQuery(db *sql.DB, query *DNSQuery) error {
	answersJSON, err := json.Marshal(query.Answers)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO dns_queries (
			request_id, domain, query_type, client_ip, server,
			is_china_dns, response_code, answer_count, total_time_ms, created_at,
			answers
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		query.RequestID, query.Domain, query.QueryType, query.ClientIP,
		query.Server, query.IsChinaDNS, query.ResponseCode,
		query.AnswerCount, query.TotalTime, query.CreatedAt,
		string(answersJSON),
	)

	if err != nil {
		return err
	}

	// 如果保存成功且管理服务器已初始化，发送更新
	if adminServer != nil {
		// 获取最新统计数据
		endTime := time.Now()
		startTime := endTime.Add(-24 * time.Hour)
		stats, err := GetQueryStats(db, startTime, endTime)
		if err == nil {
			adminServer.broadcast <- map[string]interface{}{
				"type": "stats",
				"data": stats,
			}
		}

		// 获取最新查询记录
		queries, err := GetRecentQueries(db, 100)
		if err == nil {
			adminServer.broadcast <- map[string]interface{}{
				"type": "queries",
				"data": queries,
			}
		}
	}

	return nil
}

func GetQueryStats(db *sql.DB, startTime, endTime time.Time) ([]map[string]interface{}, error) {
	// 按分钟统计查询次数
	query := `
		WITH RECURSIVE
		time_slots(slot_time) AS (
			SELECT datetime(?, 'localtime')
			UNION ALL
			SELECT datetime(slot_time, '+1 minute')
			FROM time_slots
			WHERE slot_time < datetime(?, 'localtime')
		)
		SELECT 
			strftime('%Y-%m-%d %H:%M:00', slot_time) as time_slot,
			COALESCE(query_count, 0) as total,
			COALESCE(china_dns_count, 0) as china_dns,
			COALESCE(oversea_dns_count, 0) as oversea_dns
		FROM time_slots
		LEFT JOIN (
			SELECT 
				strftime('%Y-%m-%d %H:%M:00', created_at) as query_time,
				COUNT(*) as query_count,
				SUM(CASE WHEN is_china_dns = 1 THEN 1 ELSE 0 END) as china_dns_count,
				SUM(CASE WHEN is_china_dns = 0 THEN 1 ELSE 0 END) as oversea_dns_count
			FROM dns_queries
			WHERE created_at BETWEEN datetime(?, 'localtime') AND datetime(?, 'localtime')
				GROUP BY query_time
		) stats ON time_slot = query_time
		ORDER BY time_slot ASC
	`

	rows, err := db.Query(query,
		startTime.Format("2006-01-02 15:04:05"),
		endTime.Format("2006-01-02 15:04:05"),
		startTime.Format("2006-01-02 15:04:05"),
		endTime.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []map[string]interface{}
	for rows.Next() {
		var timeStr string
		var total, chinaDNS, overseaDNS int64
		if err := rows.Scan(&timeStr, &total, &chinaDNS, &overseaDNS); err != nil {
			return nil, err
		}

		t, err := time.ParseInLocation("2006-01-02 15:04:05", timeStr, time.Local)
		if err != nil {
			return nil, err
		}

		stats = append(stats, map[string]interface{}{
			"time": t.Format(time.RFC3339),
			"total": total,
			"china_dns": chinaDNS,
			"oversea_dns": overseaDNS,
		})
	}

	return stats, nil
}

func GetRecentQueries(db *sql.DB, limit int) ([]DNSQuery, error) {
	rows, err := db.Query(`
		SELECT id, request_id, domain, query_type, client_ip,
			   server, is_china_dns, response_code, answer_count,
			   total_time_ms, created_at, answers
		FROM dns_queries
		ORDER BY created_at DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queries []DNSQuery
	for rows.Next() {
		var q DNSQuery
		var answersJSON string
		err := rows.Scan(
			&q.ID, &q.RequestID, &q.Domain, &q.QueryType, &q.ClientIP,
			&q.Server, &q.IsChinaDNS, &q.ResponseCode, &q.AnswerCount,
			&q.TotalTime, &q.CreatedAt, &answersJSON,
		)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(answersJSON), &q.Answers)
		if err != nil {
			return nil, err
		}

		queries = append(queries, q)
	}

	return queries, nil
}

func GetQueryLogs(db *sql.DB, requestID string) ([]QueryLog, error) {
	rows, err := db.Query(`
		SELECT timestamp, level, message, fields
		FROM dns_logs
		WHERE fields LIKE ?
		ORDER BY timestamp ASC`,
		fmt.Sprintf("%%\"requestId\":\"%s\"%%", requestID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []QueryLog
	for rows.Next() {
		var log QueryLog
		var fieldsJSON string
		err := rows.Scan(&log.Timestamp, &log.Level, &log.Message, &fieldsJSON)
		if err != nil {
			return nil, err
		}

		// 解析 JSON 字段
		err = json.Unmarshal([]byte(fieldsJSON), &log.Fields)
		if err != nil {
			return nil, err
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// 添加备案缓存相关函数
func GetBeianCache(db *sql.DB, domain string) (bool, string, bool) {
	var isBeian bool
	var apiResponse string
	err := db.QueryRow(`
		SELECT is_beian, api_response
		FROM beian_cache
		WHERE domain = ?`,
		domain,
	).Scan(&isBeian, &apiResponse)

	if err == sql.ErrNoRows {
		return false, "{}", false
	}
	if err != nil {
		log.WithError(err).Error("查询备案缓存失败")
		return false, "{}", false
	}

	return isBeian, apiResponse, true
}

func SaveBeianCache(db *sql.DB, domain string, isBeian bool, apiResponse string) error {
	_, err := db.Exec(`
		INSERT INTO beian_cache (domain, is_beian, api_response, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(domain) DO UPDATE SET
			is_beian = excluded.is_beian,
			api_response = excluded.api_response,
			updated_at = CURRENT_TIMESTAMP`,
		domain, isBeian, apiResponse,
	)
	return err
}

func CleanOldBeianCache(db *sql.DB, maxAge time.Duration) error {
	_, err := db.Exec(`
		DELETE FROM beian_cache
		WHERE updated_at < datetime('now', ?)`,
		fmt.Sprintf("-%d seconds", int(maxAge.Seconds())),
	)
	return err
} 