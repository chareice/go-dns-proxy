package admin

import (
	"database/sql"
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

type DBHook struct {
	db *sql.DB
}

func NewDBHook(db *sql.DB) *DBHook {
	return &DBHook{db: db}
}

func (hook *DBHook) Levels() []log.Level {
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
		log.DebugLevel,
	}
}

func (hook *DBHook) Fire(entry *log.Entry) error {
	fieldsBytes, err := json.Marshal(entry.Data)
	if err != nil {
		return err
	}

	_, err = hook.db.Exec(`
		INSERT INTO dns_logs (timestamp, level, message, fields)
		VALUES (?, ?, ?, ?)`,
		entry.Time, entry.Level.String(), entry.Message, string(fieldsBytes),
	)
	return err
} 