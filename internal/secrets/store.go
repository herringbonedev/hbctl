package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/scrypt"
	"golang.org/x/term"
)

const (
	saltSize = 16
	keySize  = 32
)

var cachedPassphrase string
var baseDirOverride string

func SetBaseDir(dir string) {
	baseDirOverride = strings.TrimSpace(dir)
}

func HasBaseDirOverride() bool {
	return strings.TrimSpace(baseDirOverride) != ""
}

func BaseDir() (string, error) {
	if baseDirOverride != "" {
		return filepath.Abs(baseDirOverride)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hbctl"), nil
}

func secretsPath() (string, error) {
	dir, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "secrets.enc"), nil
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

func SaveJWTSecret(secret *JWTSecret) error {
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

	store.JWTSecret = secret

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

func LoadJWTSecret() (*JWTSecret, error) {
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

	if store.JWTSecret == nil {
		return nil, errors.New("no jwt secret stored")
	}

	return store.JWTSecret, nil
}

/* ---------- passphrase ---------- */

func getPassphrase(confirm bool) (string, error) {
	if cachedPassphrase != "" {
		return cachedPassphrase, nil
	}

	if v := os.Getenv("HBCTL_PASSPHRASE"); v != "" {
		cachedPassphrase = v
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

	cachedPassphrase = string(p1)
	return cachedPassphrase, nil
}

func SaveAuthToken(secret *AuthToken) error {
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

	store.AuthToken = secret

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

func LoadAuthToken() (*AuthToken, error) {
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

	if store.AuthToken == nil || strings.TrimSpace(store.AuthToken.AccessToken) == "" {
		return nil, errors.New("no auth token stored")
	}

	return store.AuthToken, nil
}

func SaveServiceKey(secret *ServiceKey) error {
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

	store.ServiceKey = secret

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

func LoadServiceKey() (*ServiceKey, error) {
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

	if store.ServiceKey == nil {
		return nil, errors.New("no service key stored")
	}

	return store.ServiceKey, nil
}

func EnsureMongoRootPassword() (string, error) {
	path, err := secretsPath()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	firstTime := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		firstTime = true
	}

	pass, err := getPassphrase(firstTime)
	if err != nil {
		return "", err
	}

	var store Store
	if !firstTime {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		plain, err := decrypt(data, pass)
		if err != nil {
			return "", errors.New("failed to decrypt secrets (wrong passphrase?)")
		}
		if err := json.Unmarshal(plain, &store); err != nil {
			return "", err
		}
	}

	if strings.TrimSpace(store.MongoRootPassword) != "" {
		return store.MongoRootPassword, nil
	}

	rootPass, err := randomSecret(32)
	if err != nil {
		return "", err
	}
	store.MongoRootPassword = rootPass

	plain, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return "", err
	}

	enc, err := encrypt(plain, pass)
	if err != nil {
		return "", err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, enc, 0600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}

	return rootPass, nil
}

func randomSecret(n int) (string, error) {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
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
