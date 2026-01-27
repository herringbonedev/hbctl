package local

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/docker"
	hbmongo "github.com/herringbonedev/hbctl/internal/mongo"
	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/units"
)

type StartOptions struct {
    Project        string
    Element        string
    Unit           string
    All            bool
    RecvType       string
    NoTokenCreate  bool
}

func secretsDirForProject() (string, error) {
	files := ComposeFilesForElement("herringbone-auth")
	if len(files) == 0 {
		return "", fmt.Errorf("no compose files found")
	}
	composeFile := files[0]
	base := filepath.Dir(composeFile)
	return filepath.Join(base, "secrets", "runtime"), nil
}

func Start(opts StartOptions) error {
	fmt.Println("[hbctl] Decrypting secrets...")

	sec, err := secrets.LoadMongo()
	if err != nil {
		return fmt.Errorf("failed to load MongoDB secret: %w", err)
	}

	jwtSecret, err := secrets.LoadJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to load JWT secret: %w", err)
	}

	svcKeys, err := secrets.LoadServiceKey()
	if err != nil {
		return fmt.Errorf("failed to load service keys: %w", err)
	}

	secretsDir, err := secretsDirForProject()
	if err != nil {
		return err
	}

	if !opts.NoTokenCreate {
		if err := prepareAuthSecrets(secretsDir, jwtSecret.JWTSecret, svcKeys.PrivSvcKey, svcKeys.PubSvcKey); err != nil {
			return err
		}
	} else {
		fmt.Println("[hbctl] Skipping runtime secret generation (--no-token-create)")
	}

	env := map[string]string{
		"MONGO_ROOT_PASS": "",
		"MONGO_HOST":      sec.Host,
		"MONGO_PORT":      fmt.Sprintf("%d", sec.Port),
		"MONGO_USER":      sec.User,
		"MONGO_PASS":      sec.Password,
		"DB_NAME":         sec.Database,
		"AUTH_DB":         sec.AuthSource,
		"RECEIVER_TYPE":   "",
		"MATCHER_API":     "",
	}

	if opts.All {
		fmt.Println("[hbctl] Starting full Herringbone stack...")

		if err := ensureDatabase(opts.Project, sec); err != nil {
			return err
		}

		if err := startElement(opts.Project, env, "herringbone-auth"); err != nil {
			return err
		}

		if err := waitHTTP("http://localhost:7001/health", 45*time.Second); err != nil {
			_ = waitHTTP("http://localhost:7001/docs", 5*time.Second)
		}

		if !opts.NoTokenCreate {

			if err := ensureAdminToken(secretsDir, jwtSecret.JWTSecret); err != nil {
				return err
			}

			if err := bootstrapServices(secretsDir); err != nil {
				return err
			}

		} else {
			fmt.Println("[hbctl] Skipping service token bootstrap (--no-token-create)")
		}

		for _, e := range units.AllElements {
			if e.Name == "herringbone-auth" {
				continue
			}
			if e.Name == "logingestion-receiver" {
				env["RECEIVER_TYPE"] = "UDP"
			}
			if err := startElement(opts.Project, env, e.Name); err != nil {
				return err
			}
		}

		return nil
	}

	if opts.Unit != "" {

		elements := units.UnitElements[opts.Unit]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}

		for _, el := range elements {
			if err := startElement(opts.Project, env, el); err != nil {
				return err
			}
		}

		return nil
	}

	if opts.Element != "" {
		if opts.Element == "logingestion-receiver" && opts.RecvType == "" {
			return fmt.Errorf("error: --type required for receiver")
		}

		if opts.RecvType != "" {
			env["RECEIVER_TYPE"] = strings.ToUpper(opts.RecvType)
		}

		return startElement(opts.Project, env, opts.Element)
	}

	return fmt.Errorf("error: specify --element, --unit, or --all")
}

func bootstrapServices(secretsDir string) error {
	authURL := "http://localhost:7001"

	adminToken, err := loadAdminToken(secretsDir)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, svc := range BootstrapServices {
		svcID := fmt.Sprintf("%s-%s", svc.ID, uuidString())

		createBody := map[string]any{
			"service_id":   svcID,
			"service_name": svc.Name,
			"scopes":       svc.Scopes,
		}
		
		if err := postJSON(client, authURL+"/herringbone/auth/services/register", adminToken, createBody, nil); err != nil {
			return fmt.Errorf("create service %s failed: %w", svc.Name, err)
		}

		tokenResp := struct {
			AccessToken string `json:"access_token"`
		}{}

		if err := postJSON(client, authURL+"/herringbone/auth/service-token", adminToken, map[string]any{
			"service": svc.Name,
			"scopes":  svc.Scopes,
		}, &tokenResp); err != nil {
			return fmt.Errorf("token mint failed for %s: %w", svc.Name, err)
		}

		filename := strings.ReplaceAll(svc.Name, "-", "_") + "_service_token"
		tokenPath := filepath.Join(secretsDir, filename)

		if err := os.WriteFile(tokenPath, []byte(strings.TrimSpace(tokenResp.AccessToken)), 0444); err != nil {
			return err
		}
	}

	return nil
}

func ensureAdminToken(secretsDir, jwtSecret string) error {
	path := filepath.Join(secretsDir, "admin_token")

	if b, err := os.ReadFile(path); err == nil {
		if strings.TrimSpace(string(b)) != "" {
			return nil
		}
	}

	tok, err := mintAdminJWT(jwtSecret)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(tok), 0444); err != nil {
		return fmt.Errorf("failed writing admin_token: %w", err)
	}

	return nil
}

func mintAdminJWT(secret string) (string, error) {
	now := time.Now().UTC()

	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}

	payload := map[string]any{
		"sub":   "hbctl-bootstrap",
		"email": "hbctl@local",
		"role":  "admin",
		"typ":   "user",
		"iat":   now.Unix(),
		"exp":   now.Add(24 * time.Hour).Unix(),
	}

	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	enc := base64.RawURLEncoding
	h := enc.EncodeToString(hb)
	p := enc.EncodeToString(pb)
	msg := h + "." + p

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(msg))
	sig := enc.EncodeToString(mac.Sum(nil))

	return msg + "." + sig, nil
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", url, nil)
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for %s", url)
}

func prepareAuthSecrets(secretsDir, jwtSecret, svcPriv, svcPub string) error {
	fmt.Println("[hbctl] Preparing runtime secrets...")

	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return err
	}

	files := map[string]string{
		"jwt_secret":              jwtSecret,
		"service_jwt_private_key": svcPriv,
		"service_jwt_public_key":  svcPub,
	}

	for name, value := range files {
		path := filepath.Join(secretsDir, name)

		if err := os.WriteFile(path, []byte(value), 0444); err != nil {
			return fmt.Errorf("failed writing %s: %w", name, err)
		}
		if err := os.Chmod(path, 0444); err != nil {
			return err
		}
	}

	return nil
}

func startElement(project string, env map[string]string, element string) error {
	args := []string{"-p", project}
	args = append(args, ComposeFilesForElement(element)...)
	args = append(args, "up", "-d", "--no-recreate", element)

	fmt.Println("[hbctl] Starting", element, "...")
	return docker.ComposeWithEnv(env, args...)
}

func ensureDatabase(project string, sec *secrets.MongoSecret) error {
	rootPass := randomPassword(24)

	rootURI := fmt.Sprintf(
		"mongodb://root:%s@localhost:%d/admin",
		rootPass, sec.Port,
	)

	appURI := fmt.Sprintf(
		"mongodb://%s:%s@localhost:%d/%s?authSource=%s",
		sec.User, sec.Password, sec.Port, sec.Database, sec.AuthSource,
	)

	fmt.Println("[hbctl] Checking MongoDB app user...")
	if hbmongo.CanConnect(appURI) {
		fmt.Println("[hbctl] MongoDB already initialized.")
		return nil
	}

	env := map[string]string{
		"MONGO_ROOT_PASS": rootPass,
	}

	fmt.Println("[hbctl] Ensuring MongoDB is running...")
	if err := docker.ComposeWithEnv(env,
		"-p", project,
		"-f", ComposeMongo,
		"up", "-d", "mongodb",
	); err != nil {
		return err
	}

	fmt.Println("[hbctl] Waiting for MongoDB root auth...")
	if err := hbmongo.WaitForConnect(rootURI, 60*time.Second); err != nil {
		return fmt.Errorf("mongo root not ready: %w", err)
	}

	fmt.Println("[hbctl] Bootstrapping MongoDB user...")
	if err := hbmongo.EnsureUser(
		"localhost",
		sec.Port,
		rootPass,
		sec.User,
		sec.Password,
		sec.Database,
	); err != nil {
		return fmt.Errorf("failed to bootstrap MongoDB user: %w", err)
	}

	fmt.Println("[hbctl] MongoDB ready.")
	return nil
}

func postJSON(client *http.Client, url, token string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		d, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(d))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func loadAdminToken(secretsDir string) (string, error) {
	path := filepath.Join(secretsDir, "admin_token")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("admin token missing: %w", err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("admin token empty: %s", path)
	}
	return tok, nil
}

func uuidString() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func randomPassword(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
