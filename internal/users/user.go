package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/credentials"
)

const (
	DefaultUsersPath = "/etc/vpnctl/users/users.json"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidUserName   = errors.New("invalid user name")
)

type User struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	UUID      string     `json:"uuid"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	IsRevoked bool       `json:"is_revoked,omitempty"`
}

func (u *User) IsActive() bool {
	if !u.Enabled || u.IsRevoked {
		return false
	}
	if u.ExpiresAt != nil && time.Now().After(*u.ExpiresAt) {
		return false
	}
	return true
}

type Store struct {
	path  string
	users []*User
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		path = DefaultUsersPath
	}

	s := &Store{
		path:  path,
		users: make([]*User, 0),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.users = make([]*User, 0)
			return nil
		}
		return fmt.Errorf("read users file %s: %w", s.path, err)
	}

	var userList []*User
	if err := json.Unmarshal(data, &userList); err != nil {
		return fmt.Errorf("unmarshal users json: %w", err)
	}

	s.users = userList
	return nil
}

func (s *Store) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir users dir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal users: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", s.path, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("write tmp users file: %w", err)
	}

	if err := os.Rename(tmpFile, s.path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("atomic rename users file: %w", err)
	}

	return nil
}

func (s *Store) List() []*User {
	out := make([]*User, len(s.users))
	copy(out, s.users)
	return out
}

func (s *Store) ListEnabled() []*User {
	var out []*User
	for _, u := range s.users {
		if u.IsActive() {
			out = append(out, u)
		}
	}
	return out
}

func (s *Store) Get(nameOrID string) (*User, error) {
	for _, u := range s.users {
		if strings.EqualFold(u.Name, nameOrID) || u.ID == nameOrID || u.UUID == nameOrID {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (s *Store) Add(name string, customUUID string) (*User, error) {
	return s.AddWithExpiry(name, customUUID, 0)
}

func (s *Store) AddWithExpiry(name string, customUUID string, duration time.Duration) (*User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidUserName
	}

	for _, u := range s.users {
		if strings.EqualFold(u.Name, name) {
			return nil, ErrUserAlreadyExists
		}
	}

	uUUID := customUUID
	if uUUID == "" {
		generated, err := credentials.NewUUID()
		if err != nil {
			return nil, fmt.Errorf("generate uuid for user %s: %w", name, err)
		}
		uUUID = generated
	}

	id, _ := credentials.NewShortID(8)
	user := &User{
		ID:        id,
		Name:      name,
		UUID:      uUUID,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	}

	if duration > 0 {
		exp := time.Now().UTC().Add(duration)
		user.ExpiresAt = &exp
	}

	s.users = append(s.users, user)
	if err := s.Save(); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Store) Revoke(nameOrID string) (*User, error) {
	u, err := s.Get(nameOrID)
	if err != nil {
		return nil, err
	}
	u.IsRevoked = true
	u.Enabled = false
	if err := s.Save(); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) Remove(nameOrID string) error {
	idx := -1
	for i, u := range s.users {
		if strings.EqualFold(u.Name, nameOrID) || u.ID == nameOrID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return ErrUserNotFound
	}

	s.users = append(s.users[:idx], s.users[idx+1:]...)
	return s.Save()
}

func (s *Store) SetEnabled(nameOrID string, enabled bool) (*User, error) {
	u, err := s.Get(nameOrID)
	if err != nil {
		return nil, err
	}

	u.Enabled = enabled
	if err := s.Save(); err != nil {
		return nil, err
	}

	return u, nil
}
