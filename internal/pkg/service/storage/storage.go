package sqlite

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Storage struct {
	db *sql.DB
}

func New(databaseFile string) (*Storage, error) {

	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Storage{
		db: db,
	}, nil
}

func (s *Storage) DB() *sql.DB {
	return s.db
}

func (s *Storage) Close() error {
	if s == nil || s.db == nil {
		return nil
	}

	return s.db.Close()
}

// Query ...
func (s Storage) Query(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, sql, args...)
}
