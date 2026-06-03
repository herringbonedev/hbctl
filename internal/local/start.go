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
	Project         string
	Element         string
	Unit            string
	All             bool
	RecvType        string
	NoTokenCreate   bool
	BootstrapTokens bool
	Enterprise      bool
}

type requestOptions struct {
	OrgID string
}

func secretsDirForProject() (string, error) {
	files := ComposeFilesForElement("herringbone-auth")
	if len(files) == 0 {
		return "", fmt.Errorf("no compose files found")
	}
	composeFile := files[1]
	base := filepath.Dir(composeFile)
	return filepath.Join(base, "secrets", "runtime"), nil
}

func Start(opts StartOptions) error {
	opts.Element = CanonicalElementName(strings.TrimSpace(opts.Element))
	opts.Unit = strings.TrimSpace(opts.Unit)

	fmt.Println("[hbctl] Decrypting MongoDB secrets...")

	sec, err := secrets.LoadMongo()
	if err != nil {
		return fmt.Errorf("failed to load MongoDB secret: %w", err)
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
		"HB_ENTERPRISE":   fmt.Sprintf("%t", opts.Enterprise),
	}

	runtimeSecretsRequired := startNeedsRuntimeSecrets(opts)
	tokenBootstrapRequired := startNeedsTokenBootstrap(opts)

	if opts.NoTokenCreate {
		if tokenBootstrapRequired {
			fmt.Println("[hbctl] Skipping token bootstrap (--no-token-create)")
		}
		tokenBootstrapRequired = false
	}

	var secretsDir string
	var jwtSecret *secrets.JWTSecret
	var svcKeys *secrets.ServiceKey

	if runtimeSecretsRequired || tokenBootstrapRequired {
		fmt.Println("[hbctl] Preparing auth runtime secrets...")

		jwtSecret, err = secrets.LoadJWTSecret()
		if err != nil {
			return fmt.Errorf("failed to load JWT secret: %w", err)
		}

		svcKeys, err = secrets.LoadServiceKey()
		if err != nil {
			return fmt.Errorf("failed to load service keys: %w", err)
		}

		secretsDir, err = secretsDirForProject()
		if err != nil {
			return err
		}

		if err := prepareAuthSecrets(secretsDir, jwtSecret.JWTSecret, svcKeys.PrivSvcKey, svcKeys.PubSvcKey); err != nil {
			return err
		}
	} else if opts.NoTokenCreate {
		fmt.Println("[hbctl] --no-token-create ignored: element/unit starts do not create tokens by default")
	}

	if opts.All {
		fmt.Println("[hbctl] Starting full Herringbone stack...")

		if err := startElement(opts.Project, env, "herringbone-proxy"); err != nil {
			return err
		}

		if err := ensureDatabase(opts.Project, sec); err != nil {
			return err
		}

		if err := startElement(opts.Project, env, "herringbone-auth-e"); err != nil {
			return err
		}

		if err := waitHTTP("http://localhost:8080/health", 45*time.Second); err != nil {
			_ = waitHTTP("http://localhost:8080/docs", 5*time.Second)
		}

		if tokenBootstrapRequired {
			if err := ensureAdminToken(secretsDir, jwtSecret.JWTSecret); err != nil {
				return err
			}

			if err := bootstrapServices(secretsDir); err != nil {
				return err
			}
		} else {
			fmt.Println("[hbctl] Service token bootstrap not requested")
		}

		for _, e := range units.AllElements {
			element := CanonicalElementName(e.Name)
			if element == "herringbone-auth-e" || element == "logingestion-receiver" {
				continue
			}
			if err := startElement(opts.Project, env, element); err != nil {
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
			if err := startElement(opts.Project, env, CanonicalElementName(el)); err != nil {
				return err
			}
		}

		if tokenBootstrapRequired {
			if secretsDir == "" || jwtSecret == nil {
				return fmt.Errorf("token bootstrap requested but auth runtime secrets were not prepared")
			}
			if err := waitHTTP("http://localhost:8080/health", 45*time.Second); err != nil {
				return err
			}
			if err := ensureAdminToken(secretsDir, jwtSecret.JWTSecret); err != nil {
				return err
			}
			if err := bootstrapServices(secretsDir); err != nil {
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

		if err := startElement(opts.Project, env, opts.Element); err != nil {
			return err
		}

		if tokenBootstrapRequired {
			if secretsDir == "" || jwtSecret == nil {
				return fmt.Errorf("token bootstrap requested but auth runtime secrets were not prepared")
			}
			if err := waitHTTP("http://localhost:8080/health", 45*time.Second); err != nil {
				return err
			}
			if err := ensureAdminToken(secretsDir, jwtSecret.JWTSecret); err != nil {
				return err
			}
			if err := bootstrapServices(secretsDir); err != nil {
				return err
			}
		}

		return nil
	}

	return fmt.Errorf("error: specify --element, --unit, or --all")
}

func startNeedsRuntimeSecrets(opts StartOptions) bool {
	if opts.All || opts.BootstrapTokens {
		return true
	}
	if CanonicalElementName(opts.Element) == "herringbone-auth-e" {
		return true
	}
	for _, element := range units.UnitElements[opts.Unit] {
		if CanonicalElementName(element) == "herringbone-auth-e" {
			return true
		}
	}
	return false
}

func startNeedsTokenBootstrap(opts StartOptions) bool {
	if opts.BootstrapTokens {
		return true
	}
	return opts.All
}

func bootstrapServices(secretsDir string) error {
	authURL := "http://localhost:8080"

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

		if err := postJSON(
			client,
			authURL+"/herringbone/auth/services/internal/register",
			adminToken,
			createBody,
			nil,
			requestOptions{},
		); err != nil {
			return fmt.Errorf("create internal service %s failed: %w", svc.Name, err)
		}

		tokenResp := struct {
			AccessToken string `json:"access_token"`
		}{}

		if err := postJSON(
			client,
			authURL+"/herringbone/auth/services/internal/token",
			adminToken,
			map[string]any{
				"service": svc.Name,
				"scopes":  svc.Scopes,
			},
			&tokenResp,
			requestOptions{},
		); err != nil {
			return fmt.Errorf("internal token mint failed for %s: %w", svc.Name, err)
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
		"typ":   "user",
		"scope": []string{"*"},
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

	bootstrapPath := filepath.Join(secretsDir, "bootstrap_token")

	if _, err := os.Stat(bootstrapPath); os.IsNotExist(err) {
		token, err := generateBootstrapToken(32)
		if err != nil {
			return fmt.Errorf("failed generating bootstrap token: %w", err)
		}

		if err := os.WriteFile(bootstrapPath, []byte(token), 0444); err != nil {
			return fmt.Errorf("failed writing bootstrap_token: %w", err)
		}
		if err := os.Chmod(bootstrapPath, 0444); err != nil {
			return err
		}

		fmt.Println("[hbctl] Generated bootstrap token")
	} else {
		fmt.Println("[hbctl] Bootstrap token already exists")
	}

	return nil
}

func requiredStartElement(element string) bool {
	switch CanonicalElementName(strings.TrimSpace(element)) {
	case "herringbone-auth-e", "herringbone-proxy", "mongodb":
		return true
	default:
		return false
	}
}

func composeFileArgs(composeArgs []string) ([]string, error) {
	files := []string{}
	for i := 0; i < len(composeArgs); i++ {
		if composeArgs[i] != "-f" {
			continue
		}

		if i+1 >= len(composeArgs) {
			return nil, fmt.Errorf("missing compose file after -f")
		}

		files = append(files, composeArgs[i+1])
		i++
	}
	return files, nil
}

func shouldStartElement(element string, composeArgs []string) (bool, string, error) {
	files, err := composeFileArgs(composeArgs)
	if err != nil {
		return false, "", err
	}

	if len(files) == 0 {
		if requiredStartElement(element) {
			return false, "", fmt.Errorf("required element %q has no compose file mapping", element)
		}
		return false, "no compose file mapping", nil
	}

	hasElementCompose := false
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		if file != ComposeMongo || CanonicalElementName(element) == "mongodb" {
			hasElementCompose = true
		}

		if _, err := os.Stat(file); err != nil {
			if file == ComposeMongo || requiredStartElement(element) {
				return false, "", fmt.Errorf("required compose file missing for %q: %s", element, file)
			}
			return false, fmt.Sprintf("compose file missing: %s", file), nil
		}
	}

	if !hasElementCompose {
		if requiredStartElement(element) {
			return false, "", fmt.Errorf("required element %q has no element compose file mapping", element)
		}
		return false, "no element compose file mapping", nil
	}

	return true, "", nil
}

func startElement(project string, env map[string]string, element string) error {
	composeArgs := ComposeFilesForElement(element)
	start, reason, err := shouldStartElement(element, composeArgs)
	if err != nil {
		return err
	}
	if !start {
		fmt.Printf("[hbctl] Skipping %s: %s\n", element, reason)
		return nil
	}

	args := []string{"-p", project}
	args = append(args, composeArgs...)
	element = CanonicalElementName(element)
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
	if _, err := os.Stat(ComposeMongo); err != nil {
		return fmt.Errorf("required mongodb compose file missing: %s", ComposeMongo)
	}
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

func postJSON(client *http.Client, url, token string, body any, out any, opts requestOptions) error {
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

	if opts.OrgID != "" {
		req.Header.Set("X-Herringbone-Org", opts.OrgID)
	}

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

func getJSON(client *http.Client, url, token string, out any, opts requestOptions) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	if opts.OrgID != "" {
		req.Header.Set("X-Herringbone-Org", opts.OrgID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		d, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(d))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func maybeResolvePlatformOrgID(client *http.Client, authURL, token string) (string, error) {
	type enterpriseMeResponse struct {
		Contexts []struct {
			ContextID string `json:"context_id"`
			Slug      string `json:"slug"`
		} `json:"contexts"`
	}

	var resp enterpriseMeResponse

	err := getJSON(client, authURL+"/herringbone/auth/enterprise/me", token, &resp, requestOptions{})
	if err != nil {
		msg := err.Error()

		if strings.Contains(msg, "404") {
			return "", nil
		}
		if strings.Contains(msg, "X-Herringbone-Org header required") {
			return "", nil
		}
		if strings.Contains(msg, "default context not allowed") {
			return "", nil
		}
		if strings.Contains(msg, "invalid user id") {
			return "", nil
		}
		if strings.Contains(msg, "user identity required") {
			return "", nil
		}

		return "", err
	}

	for _, ctx := range resp.Contexts {
		if ctx.Slug == "platform" && ctx.ContextID != "" {
			return ctx.ContextID, nil
		}
	}

	return "", nil
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

func generateBootstrapToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomPassword(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
