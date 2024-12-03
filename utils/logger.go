package utils

import (
	"log"
	"os"
	"sync"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

var (
	logger     *log.Logger
	logLevel   LogLevel
	loggerOnce sync.Once
)

func InitLogger(level LogLevel) {
	loggerOnce.Do(func() {
		logger = log.New(os.Stdout, "", log.LstdFlags)
		logLevel = level
	})
}

func SetLogLevel(level LogLevel) {
	logLevel = level
}

func Debug(format string, v ...interface{}) {
	if logLevel <= LogLevelDebug {
		logger.Printf("[DEBUG] "+format, v...)
	}
}

func Info(format string, v ...interface{}) {
	if logLevel <= LogLevelInfo {
		logger.Printf("[INFO] "+format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if logLevel <= LogLevelWarn {
		logger.Printf("[WARN] "+format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if logLevel <= LogLevelError {
		logger.Printf("[ERROR] "+format, v...)
	}
}

func LogLevelFromString(level string) LogLevel {
	switch level {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

func init() {
	InitLogger(LogLevelInfo) // 默认日志级别为 INFO
} 