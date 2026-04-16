package analytics

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const SessionWindow = 30 * time.Minute

type Session struct {
	ID         string    `json:"id"`
	LastActive time.Time `json:"last_active"`
	IsNew      bool      `json:"-"`
}

// LoadOrCreateSession reads or rotates the application session file at ~/.config/tc/.session.
func LoadOrCreateSession(now time.Time) (*Session, error) {
	path, err := SessionFilePath()
	if err != nil {
		return nil, err
	}
	return loadOrCreateSessionAt(path, now)
}

func loadOrCreateSessionAt(path string, now time.Time) (*Session, error) {
	s, err := readSession(path)
	if err != nil || s == nil || now.Sub(s.LastActive) >= SessionWindow {
		id, err := newSessionID()
		if err != nil {
			return nil, err
		}
		s = &Session{ID: id, IsNew: true}
	}
	s.LastActive = now
	if err := writeSession(path, s); err != nil {
		return nil, err
	}
	return s, nil
}

func readSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.ID == "" {
		return nil, nil
	}
	return &s, nil
}

func writeSession(path string, s *Session) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

func newSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
