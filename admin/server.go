package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Server struct {
	db            *sql.DB
	router        *gin.Engine
	wsClients     map[*websocket.Conn]bool
	wsClientMutex sync.RWMutex
	broadcast     chan interface{}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewServer(db *sql.DB) *Server {
	s := &Server{
		db:        db,
		router:    gin.Default(),
		wsClients: make(map[*websocket.Conn]bool),
		broadcast: make(chan interface{}, 256),
	}
	s.setupRoutes()
	go s.handleBroadcast()
	return s
}

func (s *Server) setupRoutes() {
	s.router.LoadHTMLGlob("admin/templates/*")
	s.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	s.router.GET("/ws", s.handleWebSocket)
}

func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.WithError(err).Error("WebSocket 升级失败")
		return
	}

	conn.SetReadLimit(512 * 1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	s.wsClientMutex.Lock()
	s.wsClients[conn] = true
	s.wsClientMutex.Unlock()

	s.sendInitialData(conn)
	go s.startPing(conn)

	go func() {
		defer func() {
			s.wsClientMutex.Lock()
			delete(s.wsClients, conn)
			s.wsClientMutex.Unlock()
			conn.Close()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logrus.WithError(err).Error("WebSocket 读取错误")
				}
				return
			}

			s.handleWSMessage(conn, message)
		}
	}()
}

func (s *Server) handleWSMessage(conn *websocket.Conn, message []byte) {
	var msg struct {
		Type    string          `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(message, &msg); err != nil {
		logrus.WithError(err).Error("解析 WebSocket 消息失败")
		return
	}

	switch msg.Type {
	case "get_beian_cache":
		s.handleGetBeianCache(conn)
	case "get_stats":
		if start, ok := msg.Payload["start"].(string); ok {
			if end, ok := msg.Payload["end"].(string); ok {
				startTime, _ := time.Parse(time.RFC3339, start)
				endTime, _ := time.Parse(time.RFC3339, end)
				s.handleGetStats(conn, startTime, endTime)
			}
		}
	case "get_query_logs":
		if requestID, ok := msg.Payload["request_id"].(string); ok {
			s.handleGetQueryLogs(conn, requestID)
		}
	case "set_log_level":
		if level, ok := msg.Payload["level"].(string); ok {
			s.handleSetLogLevel(conn, level)
		}
	}
}

func (s *Server) handleGetBeianCache(conn *websocket.Conn) {
	rows, err := s.db.Query(`
		SELECT domain, is_beian, api_response, updated_at
		FROM beian_cache
		ORDER BY updated_at DESC
		LIMIT 100
	`)
	if err != nil {
		logrus.WithError(err).Error("查询备案缓存失败")
		return
	}
	defer rows.Close()

	type BeianCacheItem struct {
		Domain      string    `json:"domain"`
		IsBeian    bool      `json:"is_beian"`
		APIResponse string    `json:"api_response"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	var cacheItems []BeianCacheItem
	for rows.Next() {
		var item BeianCacheItem
		if err := rows.Scan(&item.Domain, &item.IsBeian, &item.APIResponse, &item.UpdatedAt); err != nil {
			logrus.WithError(err).Error("扫描备案缓存记录失败")
			continue
		}
		cacheItems = append(cacheItems, item)
	}

	s.sendWSMessage(conn, "beian_cache", cacheItems)
}

func (s *Server) handleGetStats(conn *websocket.Conn, startTime, endTime time.Time) {
	stats, err := GetQueryStats(s.db, startTime, endTime)
	if err != nil {
		logrus.WithError(err).Error("获取查询统计失败")
		return
	}
	s.sendWSMessage(conn, "stats", stats)
}

func (s *Server) handleGetQueryLogs(conn *websocket.Conn, requestID string) {
	rows, err := s.db.Query(`
		SELECT timestamp, level, message, fields
		FROM dns_logs
		WHERE fields LIKE ?
		ORDER BY timestamp ASC`,
		"%\"requestId\":\""+requestID+"\"%")
	if err != nil {
		logrus.WithError(err).Error("查询日志失败")
		return
	}
	defer rows.Close()

	type LogEntry struct {
		Timestamp time.Time         `json:"timestamp"`
		Level     string           `json:"level"`
		Message   string           `json:"message"`
		Fields    json.RawMessage  `json:"fields"`
	}

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		var fieldsStr string
		if err := rows.Scan(&log.Timestamp, &log.Level, &log.Message, &fieldsStr); err != nil {
			logrus.WithError(err).Error("扫描日志记录失败")
			continue
		}
		log.Fields = json.RawMessage(fieldsStr)
		logs = append(logs, log)
	}

	s.sendWSMessage(conn, "query_logs", logs)
}

func (s *Server) handleSetLogLevel(conn *websocket.Conn, level string) {
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		s.sendWSMessage(conn, "error", map[string]string{
			"message": "无效的日志级别",
		})
		return
	}

	logrus.SetLevel(parsedLevel)
	s.sendWSMessage(conn, "log_level", map[string]string{
		"level": parsedLevel.String(),
	})
}

func (s *Server) startPing(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) sendInitialData(conn *websocket.Conn) {
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)
	s.handleGetStats(conn, startTime, endTime)
	s.handleGetBeianCache(conn)

	queries, err := GetRecentQueries(s.db, 100)
	if err == nil {
		s.sendWSMessage(conn, "queries", queries)
	}

	s.sendWSMessage(conn, "log_level", map[string]string{
		"level": logrus.GetLevel().String(),
	})
}

func (s *Server) handleBroadcast() {
	for data := range s.broadcast {
		s.wsClientMutex.RLock()
		for conn := range s.wsClients {
			err := conn.WriteJSON(data)
			if err != nil {
				logrus.WithError(err).Error("WebSocket 发送失败")
				s.wsClientMutex.Lock()
				delete(s.wsClients, conn)
				s.wsClientMutex.Unlock()
				conn.Close()
			}
		}
		s.wsClientMutex.RUnlock()
	}
}

func (s *Server) sendWSMessage(conn *websocket.Conn, msgType string, data interface{}) {
	msg := map[string]interface{}{
		"type": msgType,
		"data": data,
	}
	err := conn.WriteJSON(msg)
	if err != nil {
		logrus.WithError(err).Error("WebSocket 发送失败")
	}
}

func (s *Server) Start(addr string) error {
	return s.router.Run(addr)
} 