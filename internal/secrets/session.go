package secrets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const sessionFileName = "session.json"

type ContextToken struct {
	AccessToken string   `json:"access_token,omitempty"`
	TokenType   string   `json:"token_type,omitempty"`
	ContextID   string   `json:"context_id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	Role        string   `json:"role,omitempty"`
	OrgScopes   []string `json:"org_scopes,omitempty"`
	SavedAt     string   `json:"saved_at,omitempty"`
}

type Session struct {
	Enterprise          bool          `json:"enterprise"`
	AuthToken           *AuthToken    `json:"auth_token,omitempty"`
	CurrentContextToken *ContextToken `json:"current_context_token,omitempty"`
}

func sessionPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("HBCTL_SESSION_FILE")); v != "" {
		return filepath.Abs(v)
	}
	dir, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionFileName), nil
}

func SessionPath() (string, error) { return sessionPath() }

func LoadSession() (*Session, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func SaveSession(session *Session) error {
	if session == nil {
		return errors.New("session is empty")
	}
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	plain, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, plain, 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func SaveAuthSession(token *AuthToken) error {
	return SaveAuthSessionMode(token, false, nil)
}

func SaveAuthSessionMode(token *AuthToken, enterprise bool, context *ContextToken) error {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return errors.New("auth token is empty")
	}

	current := &Session{}
	if existing, err := LoadSession(); err == nil && existing != nil {
		current = existing
	}

	current.AuthToken = token
	current.Enterprise = enterprise
	if context != nil {
		if strings.TrimSpace(context.AccessToken) == "" {
			context.AccessToken = token.AccessToken
		}
		if strings.TrimSpace(context.TokenType) == "" {
			context.TokenType = token.TokenType
		}
		if strings.TrimSpace(context.SavedAt) == "" {
			context.SavedAt = time.Now().UTC().Format(time.RFC3339)
		}
		current.CurrentContextToken = context
	} else if !enterprise {
		current.CurrentContextToken = nil
	}

	return SaveSession(current)
}

func LoadAuthSession() (*AuthToken, error) {
	session, err := LoadSession()
	if err != nil {
		return nil, err
	}
	if session.AuthToken == nil || strings.TrimSpace(session.AuthToken.AccessToken) == "" {
		return nil, errors.New("no auth token in session")
	}
	return session.AuthToken, nil
}

func SaveCurrentContextSession(context *ContextToken) error {
	if context == nil || strings.TrimSpace(context.ContextID) == "" {
		return errors.New("context id is required")
	}
	session, err := LoadSession()
	if err != nil {
		return err
	}
	if session.AuthToken == nil || strings.TrimSpace(session.AuthToken.AccessToken) == "" {
		return errors.New("no auth token in session")
	}
	if strings.TrimSpace(context.AccessToken) == "" {
		context.AccessToken = session.AuthToken.AccessToken
	}
	if strings.TrimSpace(context.TokenType) == "" {
		context.TokenType = session.AuthToken.TokenType
	}
	if strings.TrimSpace(context.SavedAt) == "" {
		context.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}
	session.Enterprise = true
	session.CurrentContextToken = context
	return SaveSession(session)
}

func ClearAuthSession() error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
