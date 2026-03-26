package memory

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Conversation struct {
	ID        int64
	Role      string
	Content   string
	CreatedAt time.Time
}

type SkillRun struct {
	ID           int64
	SkillName    string
	Input        string
	Output       string
	Success      bool
	ErrorMessage string
	DurationMs   int64
	CreatedAt    time.Time
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS kv_memory (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS skill_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_name TEXT NOT NULL,
			input TEXT,
			output TEXT,
			success BOOLEAN NOT NULL,
			error_message TEXT,
			duration_ms INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Conversation methods

func (s *Store) SaveMessage(role, content string) error {
	_, err := s.db.Exec(
		"INSERT INTO conversations (role, content) VALUES (?, ?)",
		role, content,
	)
	return err
}

func (s *Store) LoadHistory(limit int) ([]Conversation, error) {
	rows, err := s.db.Query(
		"SELECT id, role, content, created_at FROM conversations ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Role, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	// Reverse to chronological order
	for i, j := 0, len(convs)-1; i < j; i, j = i+1, j-1 {
		convs[i], convs[j] = convs[j], convs[i]
	}
	return convs, rows.Err()
}

// Key-value memory methods

func (s *Store) Store(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO kv_memory (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		key, value,
	)
	return err
}

func (s *Store) Recall(key string) ([]struct{ Key, Value string }, error) {
	rows, err := s.db.Query(
		"SELECT key, value FROM kv_memory WHERE key LIKE ?",
		"%"+key+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct{ Key, Value string }
	for rows.Next() {
		var kv struct{ Key, Value string }
		if err := rows.Scan(&kv.Key, &kv.Value); err != nil {
			return nil, err
		}
		results = append(results, kv)
	}
	return results, rows.Err()
}

// Skill run logging

func (s *Store) LogSkillRun(skillName, input, output string, success bool, errMsg string, durationMs int64) error {
	_, err := s.db.Exec(
		`INSERT INTO skill_runs (skill_name, input, output, success, error_message, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		skillName, input, output, success, errMsg, durationMs,
	)
	return err
}

func (s *Store) GetSkillRuns(skillName string, limit int) ([]SkillRun, error) {
	rows, err := s.db.Query(
		`SELECT id, skill_name, input, output, success, error_message, duration_ms, created_at
		 FROM skill_runs WHERE skill_name = ? ORDER BY id DESC LIMIT ?`,
		skillName, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []SkillRun
	for rows.Next() {
		var r SkillRun
		if err := rows.Scan(&r.ID, &r.SkillName, &r.Input, &r.Output, &r.Success, &r.ErrorMessage, &r.DurationMs, &r.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
