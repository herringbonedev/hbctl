package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/scrypt"
	"golang.org/x/term"
)

const (
	saltSize = 16
	keySize  = 32
)

func secretsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hbctl", "secrets.enc"), nil
}

func SaveMongo(secret *MongoSecret) error {
	path, err := secretsPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	firstTime := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		firstTime = true
	}

	pass, err := getPassphrase(firstTime)
	if err != nil {
		return err
	}

	var store Store

	if !firstTime {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		plain, err := decrypt(data, pass)
		if err != nil {
			return errors.New("failed to decrypt secrets (wrong passphrase?)")
		}
		_ = json.Unmarshal(plain, &store)
	}

	store.MongoDB = secret

	plain, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	enc, err := encrypt(plain, pass)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, enc, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadMongo() (*MongoSecret, error) {
	path, err := secretsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	pass, err := getPassphrase(false)
	if err != nil {
		return nil, err
	}

	plain, err := decrypt(data, pass)
	if err != nil {
		return nil, errors.New("failed to decrypt secrets (wrong passphrase?)")
	}

	var store Store
	if err := json.Unmarshal(plain, &store); err != nil {
		return nil, err
	}

	if store.MongoDB == nil {
		return nil, errors.New("no mongodb secret stored")
	}

	return store.MongoDB, nil
}

/* ---------- passphrase ---------- */

func getPassphrase(confirm bool) (string, error) {
	if v := os.Getenv("HBCTL_PASSPHRASE"); v != "" {
		return v, nil
	}

	fmt.Print("Enter hbctl passphrase: ")
	p1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	if len(p1) == 0 {
		return "", errors.New("empty passphrase not allowed")
	}

	if confirm {
		fmt.Print("Confirm hbctl passphrase: ")
		p2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		if string(p1) != string(p2) {
			return "", errors.New("passphrases do not match")
		}
	}

	return string(p1), nil
}

/* ---------- crypto ---------- */

func encrypt(data []byte, pass string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	key, err := scrypt.Key([]byte(pass), salt, 1<<15, 8, 1, keySize)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)

	out := append(salt, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func decrypt(data []byte, pass string) ([]byte, error) {
	if len(data) < saltSize {
		return nil, errors.New("invalid encrypted file")
	}

	salt := data[:saltSize]
	rest := data[saltSize:]

	key, err := scrypt.Key([]byte(pass), salt, 1<<15, 8, 1, keySize)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(rest) < gcm.NonceSize() {
		return nil, errors.New("invalid encrypted payload")
	}

	nonce := rest[:gcm.NonceSize()]
	ciphertext := rest[gcm.NonceSize():]

	return gcm.Open(nil, nonce, ciphertext, nil)
}
