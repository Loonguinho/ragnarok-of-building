// Package store keeps accounts, sessions and saved builds in SQLite.
package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite" // pure Go driver, no cgo
)

var (
	ErrTaken       = errors.New("esse ID já existe")
	ErrBadLogin    = errors.New("ID ou senha incorretos")
	ErrNotFound    = errors.New("build não encontrado")
	ErrInvalidName = errors.New("o ID deve ter de 4 a 23 letras, números ou _")
	ErrWeakPass    = errors.New("a senha deve ter pelo menos 6 caracteres")
	ErrTooMany     = errors.New("limite de builds atingido")
)

// MaxBuilds is how many builds one account can keep.
const MaxBuilds = 60

// SessionTTL is how long a login lasts.
const SessionTTL = 30 * 24 * time.Hour

type Store struct{ db *sql.DB }

type User struct {
	ID   int64
	Name string
}

// Build is a saved build: the encoded build code plus what the character
// select screen shows without decoding it.
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
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: one writer; keeps things simple
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
	id         INTEGER PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE COLLATE NOCASE,
	pass_hash  BLOB NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS builds (
	id         INTEGER PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	code       TEXT NOT NULL,
	job        INTEGER NOT NULL,
	base_lv    INTEGER NOT NULL,
	job_lv     INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS builds_user ON builds(user_id, updated_at);`)
	return err
}

func validName(n string) bool {
	if len(n) < 4 || len(n) > 23 {
		return false
	}
	for _, r := range n {
		if !(r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

// Register creates an account.
func (s *Store) Register(name, pass string) (User, error) {
	name = strings.TrimSpace(name)
	if !validName(name) {
		return User{}, ErrInvalidName
	}
	if utf8.RuneCountInString(pass) < 6 || len(pass) > 72 { // bcrypt reads at most 72 bytes
		return User{}, ErrWeakPass
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	res, err := s.db.Exec(`INSERT INTO users(name, pass_hash, created_at) VALUES(?, ?, ?)`, name, hash, time.Now().Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return User{}, ErrTaken
		}
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return User{ID: id, Name: name}, nil
}

// Login checks a name and password.
func (s *Store) Login(name, pass string) (User, error) {
	var u User
	var hash []byte
	err := s.db.QueryRow(`SELECT id, name, pass_hash FROM users WHERE name = ?`, strings.TrimSpace(name)).Scan(&u.ID, &u.Name, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(pass)) // same timing as a wrong password
		return User{}, ErrBadLogin
	}
	if err != nil {
		return User{}, err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(pass)) != nil {
		return User{}, ErrBadLogin
	}
	return u, nil
}

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not a real password"), bcrypt.DefaultCost)

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// NewSession returns a random session token for the user (only its hash is
// stored).
func (s *Store) NewSession(userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now()
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now.Unix()); err != nil {
		return "", err
	}
	_, err := s.db.Exec(`INSERT INTO sessions(token_hash, user_id, expires_at) VALUES(?, ?, ?)`,
		tokenHash(token), userID, now.Add(SessionTTL).Unix())
	return token, err
}

// UserBySession returns the session's user, or ok=false.
func (s *Store) UserBySession(token string) (User, bool) {
	if token == "" {
		return User{}, false
	}
	var u User
	err := s.db.QueryRow(`SELECT u.id, u.name FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash(token), time.Now().Unix()).Scan(&u.ID, &u.Name)
	return u, err == nil
}

// EndSession logs a session out.
func (s *Store) EndSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
	return err
}

// SaveBuild creates (b.ID == 0) or updates one of the user's builds.
func (s *Store) SaveBuild(userID int64, b Build) (int64, error) {
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
		res, err := s.db.Exec(`UPDATE builds SET name = ?, code = ?, job = ?, base_lv = ?, job_lv = ?, updated_at = ?
			WHERE id = ? AND user_id = ?`, b.Name, b.Code, b.Job, b.BaseLv, b.JobLv, now, b.ID, userID)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, ErrNotFound
		}
		return b.ID, nil
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM builds WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return 0, err
	}
	if count >= MaxBuilds {
		return 0, ErrTooMany
	}
	res, err := s.db.Exec(`INSERT INTO builds(user_id, name, code, job, base_lv, job_lv, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		userID, b.Name, b.Code, b.Job, b.BaseLv, b.JobLv, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Builds lists the user's builds, most recent first.
func (s *Store) Builds(userID int64) ([]Build, error) {
	rows, err := s.db.Query(`SELECT id, name, code, job, base_lv, job_lv, updated_at FROM builds
		WHERE user_id = ? ORDER BY updated_at DESC, id DESC`, userID)
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

// GetBuild returns one of the user's builds.
func (s *Store) GetBuild(userID, id int64) (Build, error) {
	var b Build
	var ts int64
	err := s.db.QueryRow(`SELECT id, name, code, job, base_lv, job_lv, updated_at FROM builds WHERE id = ? AND user_id = ?`,
		id, userID).Scan(&b.ID, &b.Name, &b.Code, &b.Job, &b.BaseLv, &b.JobLv, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return Build{}, ErrNotFound
	}
	b.Updated = time.Unix(ts, 0)
	return b, err
}

// DeleteBuild removes one of the user's builds.
func (s *Store) DeleteBuild(userID, id int64) error {
	res, err := s.db.Exec(`DELETE FROM builds WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
