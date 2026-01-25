package config

import (
	"os"
	"path/filepath"
)

func HbctlDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hbctl"), nil
}
