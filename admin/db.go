package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

// 添加全局变量
var (
	adminServer *Server
)

// 添加设置服务器实例的函数
func SetAdminServer(server *Server) {
	adminServer = server
}

func InitDB(dbPath string) (*sql.DB, error) {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %v", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}

	// 创建表
	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("创建表失败: %v", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	// 设置 SQLite 参数以减少锁定问题
	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA busy_timeout=5000;
	`); err != nil {
		return fmt.Errorf("设置 SQLite 参数失败: %v", err)
	}

	_, err := db.Exec(`
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
			total_time_ms REAL NOT NULL,
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
	return err
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
		query.AnswerCount, query.TotalTimeMs, query.CreatedAt,
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
			adminServer.broadcast <- stats
		}

		// 获取最新查询记录
		queries, err := GetRecentQueries(db, "", 20)
		if err == nil {
			adminServer.broadcast <- map[string]interface{}{
				"type": "queries",
				"data": map[string]interface{}{
					"data":        queries,
					"next_cursor": "",
				},
			}
		}
	}

	return nil
}

func GetQueryStats(db *sql.DB, startTime, endTime time.Time) (*QueryStats, error) {
	var stats QueryStats
	var avgTime sql.NullFloat64

	// 获取总查询数和平均响应时间
	err := db.QueryRow(`
		SELECT 
			COALESCE(COUNT(*), 0) as total_queries,
			COALESCE(AVG(total_time_ms), 0) as avg_time,
			COALESCE(SUM(CASE WHEN is_china_dns = 1 THEN 1 ELSE 0 END), 0) as china_dns_queries,
			COALESCE(SUM(CASE WHEN is_china_dns = 0 THEN 1 ELSE 0 END), 0) as oversea_dns_queries
		FROM dns_queries
		WHERE created_at BETWEEN ? AND ?`,
		startTime, endTime,
	).Scan(&stats.TotalQueries, &avgTime, &stats.ChinaDNSQueries, &stats.OverseaDNSQueries)

	if err != nil {
		return nil, err
	}

	stats.AverageTimeMs = avgTime.Float64

	// 获取热门域名
	rows, err := db.Query(`
		SELECT domain, COUNT(*) as count
		FROM dns_queries
		WHERE created_at BETWEEN ? AND ?
		GROUP BY domain
		ORDER BY count DESC
		LIMIT 10`,
		startTime, endTime,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats.TopDomains = make([]DomainCount, 0)
	for rows.Next() {
		var dc DomainCount
		if err := rows.Scan(&dc.Domain, &dc.Count); err != nil {
			return nil, err
		}
		stats.TopDomains = append(stats.TopDomains, dc)
	}

	// 获取热门客户端
	rows, err = db.Query(`
		SELECT client_ip, COUNT(*) as count
		FROM dns_queries
		WHERE created_at BETWEEN ? AND ?
		GROUP BY client_ip
		ORDER BY count DESC
		LIMIT 10`,
		startTime, endTime,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats.TopClients = make([]ClientCount, 0)
	for rows.Next() {
		var cc ClientCount
		if err := rows.Scan(&cc.ClientIP, &cc.Count); err != nil {
			return nil, err
		}
		stats.TopClients = append(stats.TopClients, cc)
	}

	return &stats, nil
}

func GetRecentQueries(db *sql.DB, cursor string, limit int) ([]DNSQuery, error) {
	var rows *sql.Rows
	var err error

	if cursor == "" {
		rows, err = db.Query(`
			SELECT id, request_id, domain, query_type, client_ip,
				   server, is_china_dns, response_code, answer_count,
				   total_time_ms, created_at, answers
			FROM dns_queries
			ORDER BY created_at DESC, id DESC
			LIMIT ?`,
			limit,
		)
	} else {
		// 解析游标
		var cursorTime time.Time
		var cursorID int64
		_, err := fmt.Sscanf(cursor, "%s_%d", &cursorTime, &cursorID)
		if err != nil {
			return nil, fmt.Errorf("无效的游标格式: %v", err)
		}

		rows, err = db.Query(`
			SELECT id, request_id, domain, query_type, client_ip,
				   server, is_china_dns, response_code, answer_count,
				   total_time_ms, created_at, answers
			FROM dns_queries
			WHERE (created_at, id) < (?, ?)
			ORDER BY created_at DESC, id DESC
			LIMIT ?`,
			cursorTime, cursorID, limit,
		)
	}

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
			&q.TotalTimeMs, &q.CreatedAt, &answersJSON,
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

// ... rest of the file ... 