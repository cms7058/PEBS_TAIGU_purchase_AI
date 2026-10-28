package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func openStorage(dir string) (*sql.DB, error) {
	path := filepath.Join(dir, "training.sqlite")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS schema_version(version INTEGER PRIMARY KEY);
INSERT OR IGNORE INTO schema_version VALUES(1);
CREATE TABLE IF NOT EXISTS classroom(id INTEGER PRIMARY KEY CHECK(id=1), payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS training_attempts(id TEXT PRIMARY KEY, learner TEXT NOT NULL, module TEXT NOT NULL, number INTEGER NOT NULL, status TEXT NOT NULL, revision INTEGER NOT NULL, payload BLOB NOT NULL, UNIQUE(learner,module,number));
CREATE INDEX IF NOT EXISTS training_owner ON training_attempts(learner,module,number DESC);
CREATE TABLE IF NOT EXISTS training_events(id INTEGER PRIMARY KEY, attempt TEXT NOT NULL REFERENCES training_attempts(id), at TEXT NOT NULL, kind TEXT NOT NULL, payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS exam_config(id INTEGER PRIMARY KEY CHECK(id=1), payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS exam_attempts(id TEXT PRIMARY KEY, learner TEXT NOT NULL UNIQUE, payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS review_sessions(attempt TEXT PRIMARY KEY REFERENCES training_attempts(id), payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS consultations(id TEXT PRIMARY KEY, attempt TEXT NOT NULL REFERENCES training_attempts(id), learner TEXT NOT NULL, module TEXT NOT NULL, payload BLOB NOT NULL);
CREATE INDEX IF NOT EXISTS consultation_owner ON consultations(learner,module);
CREATE TABLE IF NOT EXISTS ai_usage(id INTEGER PRIMARY KEY, learner TEXT NOT NULL, at TEXT NOT NULL, mode TEXT NOT NULL, tokens INTEGER NOT NULL);
`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s *Server) loadState() ([]byte, error) {
	var b []byte
	err := s.db.QueryRow("SELECT payload FROM classroom WHERE id=1").Scan(&b)
	if err == sql.ErrNoRows {
		b, err = os.ReadFile(filepath.Join(s.cfg.DataDir, "classroom.json"))
	}
	return b, err
}

func (s *Server) persist(st State) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO classroom(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", b)
	return err
}

func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Server) saveAttempt(a *TrainingAttempt, kind string, detail any) error {
	payload, err := json.Marshal(a)
	if err != nil {
		return err
	}
	extra, err := json.Marshal(map[string]any{"detail": detail, "activeSeconds": a.ActiveSeconds, "elapsedSeconds": a.UpdatedAt.Sub(a.StartedAt).Seconds()})
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO training_attempts(id,learner,module,number,status,revision,payload) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,revision=excluded.revision,payload=excluded.payload`, a.ID, a.Learner, a.Module, a.Number, a.Status, a.Revision, payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO training_events(attempt,at,kind,payload) VALUES(?,?,?,?)", a.ID, a.UpdatedAt.Format(timeFormat), kind, extra)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) getAttempt(id string) (*TrainingAttempt, error) {
	var b []byte
	if err := s.db.QueryRow("SELECT payload FROM training_attempts WHERE id=?", id).Scan(&b); err != nil {
		return nil, err
	}
	var a TrainingAttempt
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("decode attempt: %w", err)
	}
	return &a, nil
}
func (s *Server) latestAttempt(learner, module string) (*TrainingAttempt, error) {
	var id string
	err := s.db.QueryRow("SELECT id FROM training_attempts WHERE learner=? AND module=? ORDER BY number DESC LIMIT 1", learner, module).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.getAttempt(id)
}
