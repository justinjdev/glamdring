package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/justin/glamdring/pkg/api"
)

// SessionMeta holds index metadata for one session.
type SessionMeta struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

// Store manages JSONL session files and the index.
type Store struct {
	dir string
}

// Open creates the directory if absent and returns a Store.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// NewSession generates a new session with a UUID and default metadata.
// No file is written until the first call to AppendMessages.
func (s *Store) NewSession() (SessionMeta, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return SessionMeta{}, err
	}
	return SessionMeta{
		ID:        id.String(),
		Title:     "New Session",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// AppendMessages appends messages to the session JSONL file, creating it if needed.
func (s *Store) AppendMessages(id string, msgs []api.RequestMessage) error {
	f, err := os.OpenFile(s.sessionPath(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, msg := range msgs {
		if err := enc.Encode(msg); err != nil {
			return err
		}
	}
	return nil
}

// LoadMessages reads the session JSONL file and returns all messages.
// Returns an empty (non-nil) slice if the file does not exist.
func (s *Store) LoadMessages(id string) ([]api.RequestMessage, error) {
	f, err := os.Open(s.sessionPath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []api.RequestMessage{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var msgs []api.RequestMessage
	scanner := bufio.NewScanner(f)
	// Increase buffer size to handle large messages
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg api.RequestMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if msgs == nil {
		return []api.RequestMessage{}, nil
	}
	return msgs, nil
}

// CloseSession updates the index with final metadata for the session.
func (s *Store) CloseSession(id string, firstUserContent string, messageCount int) error {
	sessions, err := s.readIndex()
	if err != nil {
		return err
	}

	title := truncateTitle(firstUserContent)
	if title == "" {
		title = "New Session"
	}

	now := time.Now()
	found := false
	for i := range sessions {
		if sessions[i].ID == id {
			sessions[i].UpdatedAt = now
			sessions[i].MessageCount = messageCount
			sessions[i].Title = title
			found = true
			break
		}
	}
	if !found {
		sessions = append(sessions, SessionMeta{
			ID:           id,
			Title:        title,
			CreatedAt:    now,
			UpdatedAt:    now,
			MessageCount: messageCount,
		})
	}

	return s.writeIndex(sessions)
}

// ListSessions returns all sessions sorted by UpdatedAt descending.
// Returns an empty slice (not an error) if the index does not exist.
func (s *Store) ListSessions() ([]SessionMeta, error) {
	sessions, err := s.readIndex()
	if err != nil {
		return nil, err
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

// DeleteSession removes the session file and its index entry.
func (s *Store) DeleteSession(id string) error {
	if err := os.Remove(s.sessionPath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	sessions, err := s.readIndex()
	if err != nil {
		return err
	}

	filtered := sessions[:0]
	for _, sess := range sessions {
		if sess.ID != id {
			filtered = append(filtered, sess)
		}
	}

	return s.writeIndex(filtered)
}

// RebuildIndex scans all JSONL files in the directory and rebuilds the index.
// Returns the number of sessions found.
func (s *Store) RebuildIndex() (int, error) {
	pattern := filepath.Join(s.dir, "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return 0, err
	}

	var sessions []SessionMeta
	for _, fpath := range files {
		base := filepath.Base(fpath)
		id := strings.TrimSuffix(base, ".jsonl")

		info, err := os.Stat(fpath)
		if err != nil {
			continue
		}
		mtime := info.ModTime()

		// Load first message for title and count total lines
		title, count, err := scanSessionFile(fpath)
		if err != nil {
			continue
		}

		sessions = append(sessions, SessionMeta{
			ID:           id,
			Title:        title,
			CreatedAt:    mtime,
			UpdatedAt:    mtime,
			MessageCount: count,
		})
	}

	if sessions == nil {
		sessions = []SessionMeta{}
	}

	if err := s.writeIndex(sessions); err != nil {
		return 0, err
	}
	return len(sessions), nil
}

// indexPath returns the path to the index file.
func (s *Store) indexPath() string {
	return filepath.Join(s.dir, "index.json")
}

// sessionPath returns the path to a session's JSONL file.
func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.dir, id+".jsonl")
}

// readIndex reads and parses the index file. Returns an empty slice if not found.
func (s *Store) readIndex() ([]SessionMeta, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []SessionMeta{}, nil
		}
		return nil, err
	}

	var sessions []SessionMeta
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}
	if sessions == nil {
		return []SessionMeta{}, nil
	}
	return sessions, nil
}

// writeIndex atomically writes the session index to disk.
func (s *Store) writeIndex(sessions []SessionMeta) error {
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := filepath.Join(s.dir, ".index.tmp")
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.indexPath())
}

// truncateTitle returns the first 60 runes of s, trimmed of whitespace.
func truncateTitle(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= 60 {
		return s
	}
	runes := []rune(s)
	return string(runes[:60])
}

// scanSessionFile reads a JSONL file and returns the title (from first user message)
// and the total message count.
func scanSessionFile(fpath string) (string, int, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	title := "New Session"
	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		count++
		if count == 1 {
			// Attempt to extract title from first user message
			var msg api.RequestMessage
			if err := json.Unmarshal([]byte(line), &msg); err == nil {
				if msg.Role == "user" {
					if text, ok := msg.Content.(string); ok {
						t := truncateTitle(text)
						if t != "" {
							title = t
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", 0, err
	}
	return title, count, nil
}
