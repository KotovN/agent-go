package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage interface using SQLite database
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(path string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			value TEXT NOT NULL,
			metadata TEXT,
			source TEXT,
			scope TEXT,
			type TEXT,
			score REAL,
			expires_at DATETIME,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

// Save stores an item in SQLite storage
func (s *SQLiteStorage) Save(ctx context.Context, item *StorageItem) error {
	metadata, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memories (value, metadata, source, scope, type, score, expires_at, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, item.Value, string(metadata), item.Source, item.Scope, item.Type, item.Score, item.ExpiresAt, item.Timestamp)

	if err != nil {
		return fmt.Errorf("failed to insert memory: %w", err)
	}

	return nil
}

// Search finds memories matching the query
func (s *SQLiteStorage) Search(ctx context.Context, query string, maxResults int) ([]*StorageItem, error) {
	// Simple text search using LIKE
	rows, err := s.db.QueryContext(ctx, `
		SELECT value, metadata, source, scope, type, score, expires_at, timestamp
		FROM memories 
		WHERE value LIKE ? 
		ORDER BY timestamp DESC 
		LIMIT ?
	`, "%"+query+"%", maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}
	defer rows.Close()

	var results []*StorageItem
	for rows.Next() {
		var item StorageItem
		var metadataStr string
		err := rows.Scan(&item.Value, &metadataStr, &item.Source, &item.Scope, &item.Type, &item.Score, &item.ExpiresAt, &item.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		err = json.Unmarshal([]byte(metadataStr), &item.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		results = append(results, &item)
	}

	return results, nil
}

// Reset clears all stored items
func (s *SQLiteStorage) Reset(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM memories")
	if err != nil {
		return fmt.Errorf("failed to clear memories: %w", err)
	}
	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}
