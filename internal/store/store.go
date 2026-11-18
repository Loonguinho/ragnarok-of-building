// Package store keeps the saved builds in a local SQLite file.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite" // pure Go driver, no cgo
)

var (
	ErrNotFound = errors.New("build não encontrado")
	ErrTooMany  = errors.New("limite de builds atingido")
)

// MaxBuilds is how many builds can be kept.
const MaxBuilds = 500

type Store struct{ db *sql.DB }

// Build is a saved build: the encoded build code plus what the home screen
// shows without decoding it.
type Build struct {
	ID      int64
	Name    string
	Code    string
	Job     int
	BaseLv  int
	JobLv   int
	Updated time.Time
}

// Open opens (creating if needed) the database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: one writer; keeps things simple
	s := &Store{db: db}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS saved_builds (
	id         INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	code       TEXT NOT NULL,
	job        INTEGER NOT NULL,
	base_lv    INTEGER NOT NULL,
	job_lv     INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS saved_builds_updated ON saved_builds(updated_at);`); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SaveBuild creates (b.ID == 0) or updates a build.
func (s *Store) SaveBuild(b Build) (int64, error) {
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		b.Name = "Sem nome"
	}
	if utf8.RuneCountInString(b.Name) > 40 {
		b.Name = string([]rune(b.Name)[:40])
	}
	if b.Code == "" || len(b.Code) > 64<<10 {
		return 0, fmt.Errorf("código de build inválido")
	}
	now := time.Now().Unix()
	if b.ID != 0 {
		res, err := s.db.Exec(`UPDATE saved_builds SET name = ?, code = ?, job = ?, base_lv = ?, job_lv = ?, updated_at = ? WHERE id = ?`,
			b.Name, b.Code, b.Job, b.BaseLv, b.JobLv, now, b.ID)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, ErrNotFound
		}
		return b.ID, nil
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM saved_builds`).Scan(&count); err != nil {
		return 0, err
	}
	if count >= MaxBuilds {
		return 0, ErrTooMany
	}
	res, err := s.db.Exec(`INSERT INTO saved_builds(name, code, job, base_lv, job_lv, updated_at) VALUES(?, ?, ?, ?, ?, ?)`,
		b.Name, b.Code, b.Job, b.BaseLv, b.JobLv, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Builds lists the saved builds, most recent first.
func (s *Store) Builds() ([]Build, error) {
	rows, err := s.db.Query(`SELECT id, name, code, job, base_lv, job_lv, updated_at FROM saved_builds ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Build
	for rows.Next() {
		var b Build
		var ts int64
		if err := rows.Scan(&b.ID, &b.Name, &b.Code, &b.Job, &b.BaseLv, &b.JobLv, &ts); err != nil {
			return nil, err
		}
		b.Updated = time.Unix(ts, 0)
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBuild returns one build.
func (s *Store) GetBuild(id int64) (Build, error) {
	var b Build
	var ts int64
	err := s.db.QueryRow(`SELECT id, name, code, job, base_lv, job_lv, updated_at FROM saved_builds WHERE id = ?`, id).
		Scan(&b.ID, &b.Name, &b.Code, &b.Job, &b.BaseLv, &b.JobLv, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return Build{}, ErrNotFound
	}
	b.Updated = time.Unix(ts, 0)
	return b, err
}

// DeleteBuild removes a build.
func (s *Store) DeleteBuild(id int64) error {
	res, err := s.db.Exec(`DELETE FROM saved_builds WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
