package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

type loginAttemptResult struct {
	Token     string
	TokenType string
	AuthURL   string
	Path      string
}

func loginCommand() *cobra.Command {
	var email string
	var password string
	var authURL string
	var loginPath string
	var timeoutSeconds int
	var enterprise bool
	var contextName string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Herringbone or store encrypted credentials and keys",
		Long: "Log in to the Herringbone auth service and store the returned token in the hbctl session file. " +
			"Subcommands are also available for storing MongoDB credentials, JWT secrets, and service signing keys.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" || password == "" {
				return fmt.Errorf("-u/--user and -p/--password are required")
			}

			ui.FHeader(cmd.OutOrStdout(), "Herringbone login")
			ui.FStep(cmd.OutOrStdout(), "Authenticating with auth service")

			result, err := loginToAuth(email, password, authURL, loginPath, time.Duration(timeoutSeconds)*time.Second)
			if err != nil {
				return err
			}

			savedAt := time.Now().UTC().Format(time.RFC3339)
			authToken := &secrets.AuthToken{
				Email:       email,
				AccessToken: result.Token,
				TokenType:   result.TokenType,
				AuthURL:     result.AuthURL,
				LoginPath:   result.Path,
				SavedAt:     savedAt,
			}

			var currentContext *secrets.ContextToken
			if enterprise {
				ui.FStep(cmd.OutOrStdout(), "Loading enterprise contexts")
				client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
				contexts, err := fetchEnterpriseContexts(client, result.AuthURL, result.Token)
				if err != nil {
					return fmt.Errorf("login succeeded but enterprise context lookup failed: %w", err)
				}
				ctx, ok := selectEnterpriseContext(contexts, contextName)
				if !ok {
					return fmt.Errorf("login succeeded but no enterprise contexts are available for this user")
				}
				currentContext = contextInfoToSessionToken(ctx, authToken)
			}

			if err := secrets.SaveAuthTokenSession(authToken, enterprise, currentContext); err != nil {
				return fmt.Errorf("failed to store auth token: %w", err)
			}

			ui.FSuccess(cmd.OutOrStdout(), "Auth token saved to hbctl session file")
			rows := [][2]string{
				{"email", email},
				{"auth url", result.AuthURL},
				{"login path", result.Path},
				{"enterprise", mapBool(enterprise, "true", "false")},
				{"saved at", savedAt},
			}
			if currentContext != nil {
				rows = append(rows, [2]string{"context", currentContext.Slug}, [2]string{"context id", currentContext.ContextID}, [2]string{"role", currentContext.Role})
			}
			ui.FKeyValues(cmd.OutOrStdout(), rows)
			return nil
		},
	}

	cmd.Flags().StringVarP(&email, "user", "u", "", "User email for auth login")
	cmd.Flags().StringVarP(&password, "password", "p", "", "User password for auth login")
	cmd.Flags().StringVar(&authURL, "auth-url", "", "Auth service base URL; defaults to saved hbctl server or http://localhost:8080")
	cmd.Flags().StringVar(&loginPath, "login-path", "", "Login path to call; defaults to trying proxied and direct login paths")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 10, "Auth login timeout in seconds")
	cmd.Flags().BoolVar(&enterprise, "enterprise", false, "Store this login as an enterprise session and select an enterprise context")
	cmd.Flags().StringVar(&contextName, "context", "", "Enterprise context slug or id to select; defaults to platform or the first available context")

	cmd.AddCommand(loginMongoCommand())
	cmd.AddCommand(loginJWTSecretCommand())
	cmd.AddCommand(loginServiceKeyCommand())
	return cmd
}

func defaultAuthURL() string {
	if v := strings.TrimSpace(os.Getenv("HBCTL_AUTH_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("HBCTL_SERVER_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if cfg, err := secrets.LoadServerConfig(); err == nil && strings.TrimSpace(cfg.BaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	}
	return "http://localhost:8080"
}

func loginToAuth(email, password, authURL, loginPath string, timeout time.Duration) (*loginAttemptResult, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	baseURLs := loginBaseURLs(authURL)
	paths := loginPaths(loginPath)
	payloads := []map[string]string{
		{"email": email, "password": password},
		{"username": email, "password": password},
		{"user": email, "password": password},
	}

	client := &http.Client{Timeout: timeout}
	var lastErr error
	for _, base := range baseURLs {
		for _, path := range paths {
			endpoint, err := joinURL(base, path)
			if err != nil {
				lastErr = err
				continue
			}

			for _, payload := range payloads {
				token, tokenType, retry, err := postLogin(client, endpoint, payload)
				if err == nil {
					return &loginAttemptResult{
						Token:     token,
						TokenType: tokenType,
						AuthURL:   strings.TrimRight(base, "/"),
						Path:      path,
					}, nil
				}

				lastErr = err
				if !retry {
					return nil, err
				}
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("auth login failed: %w", lastErr)
	}
	return nil, errors.New("auth login failed: no login endpoints were attempted")
}

func loginBaseURLs(primary string) []string {
	primary = strings.TrimRight(strings.TrimSpace(primary), "/")
	if primary == "" {
		primary = "http://localhost:8080"
	}

	out := []string{primary}
	if primary == "http://localhost:8080" || primary == "http://127.0.0.1:8080" {
		out = append(out, "http://localhost:7001")
	}
	return dedupeStrings(out)
}

func loginPaths(explicit string) []string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		if !strings.HasPrefix(explicit, "/") {
			explicit = "/" + explicit
		}
		return []string{explicit}
	}
	return []string{
		"/herringbone/auth/login",
		"/login",
	}
}

func joinURL(base, path string) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", errors.New("auth url is empty")
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid auth url: %s", base)
	}
	basePath := strings.TrimRight(u.Path, "/")
	path = "/" + strings.TrimLeft(path, "/")
	u.Path = basePath + path
	return u.String(), nil
}

func postLogin(client *http.Client, endpoint string, payload map[string]string) (string, string, bool, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", false, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", true, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}

		// Wrong credentials should fail loudly. Missing routes or schema mismatch can
		// continue to the next supported auth deployment shape.
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", "", false, fmt.Errorf("auth login rejected credentials: http %d: %s", resp.StatusCode, message)
		}

		return "", "", true, fmt.Errorf("%s returned http %d: %s", endpoint, resp.StatusCode, message)
	}

	token, tokenType, err := tokenFromLoginResponse(data)
	if err != nil {
		return "", "", true, fmt.Errorf("%s returned no usable token: %w", endpoint, err)
	}

	return token, tokenType, false, nil
}

func tokenFromLoginResponse(data []byte) (string, string, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", "", err
	}

	tokenKeys := map[string]bool{
		"access_token": true,
		"accessToken":  true,
		"auth_token":   true,
		"bearer_token": true,
		"id_token":     true,
		"jwt":          true,
		"token":        true,
	}

	token, tokenType := findToken(raw, tokenKeys)
	if strings.TrimSpace(token) == "" {
		return "", "", errors.New("expected one of access_token, accessToken, auth_token, bearer_token, id_token, jwt, or token")
	}
	if strings.TrimSpace(tokenType) == "" {
		tokenType = "bearer"
	}
	return strings.TrimSpace(token), strings.TrimSpace(tokenType), nil
}

func findToken(value any, tokenKeys map[string]bool) (string, string) {
	switch v := value.(type) {
	case map[string]any:
		tokenType := ""
		for key, child := range v {
			if strings.EqualFold(key, "token_type") || strings.EqualFold(key, "tokenType") {
				if s, ok := child.(string); ok {
					tokenType = s
				}
			}
		}
		for key, child := range v {
			if tokenKeys[key] {
				if s, ok := child.(string); ok && strings.TrimSpace(s) != "" {
					return s, tokenType
				}
			}
		}
		for _, child := range v {
			token, nestedType := findToken(child, tokenKeys)
			if token != "" {
				if tokenType == "" {
					tokenType = nestedType
				}
				return token, tokenType
			}
		}
	case []any:
		for _, child := range v {
			token, tokenType := findToken(child, tokenKeys)
			if token != "" {
				return token, tokenType
			}
		}
	}
	return "", ""
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func loginMongoCommand() *cobra.Command {
	var user string
	var password string
	var host string
	var database string
	var port int
	var authSource string
	var replicaSet string

	cmd := &cobra.Command{
		Use:   "mongodb",
		Short: "Store MongoDB credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if user == "" || password == "" || host == "" {
				return fmt.Errorf("--user, --password, and --host are required")
			}
			err := secrets.SaveMongo(&secrets.MongoSecret{
				User:       user,
				Password:   password,
				Host:       host,
				Port:       port,
				Database:   database,
				AuthSource: authSource,
				ReplicaSet: replicaSet,
			})
			if err != nil {
				return err
			}
			ui.FSuccess(cmd.OutOrStdout(), "MongoDB credentials saved")
			return nil
		},
	}

	cmd.Flags().StringVar(&user, "user", "", "MongoDB username")
	cmd.Flags().StringVar(&password, "password", "", "MongoDB password")
	cmd.Flags().StringVar(&host, "host", "", "MongoDB host")
	cmd.Flags().StringVar(&database, "database", "herringbone", "MongoDB database")
	cmd.Flags().IntVar(&port, "port", 27017, "MongoDB port")
	cmd.Flags().StringVar(&authSource, "auth-source", "admin", "MongoDB auth source")
	cmd.Flags().StringVar(&replicaSet, "replica-set", "", "MongoDB replica set")
	return cmd
}

func loginJWTSecretCommand() *cobra.Command {
	var jwtSecret string

	cmd := &cobra.Command{
		Use:   "jwtsecret",
		Short: "Store the JWT secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jwtSecret == "" {
				return fmt.Errorf("--secret is required")
			}
			if err := secrets.SaveJWTSecret(&secrets.JWTSecret{JWTSecret: jwtSecret}); err != nil {
				return err
			}
			ui.FSuccess(cmd.OutOrStdout(), "JWT secret saved")
			return nil
		},
	}

	cmd.Flags().StringVar(&jwtSecret, "secret", "", "JWT secret value")
	return cmd
}

func loginServiceKeyCommand() *cobra.Command {
	var publicPath string
	var privatePath string
	var publicInline string
	var privateInline string
	var generate bool
	var bits int

	cmd := &cobra.Command{
		Use:   "servicekey",
		Short: "Store or generate service signing keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			var publicKey string
			var privateKey string

			if generate {
				generatedPublic, generatedPrivate, err := secrets.GenerateServiceKeyPair(bits)
				if err != nil {
					return err
				}
				publicKey = generatedPublic
				privateKey = generatedPrivate
			} else {
				if publicInline != "" {
					publicKey = publicInline
				} else if publicPath != "" {
					data, err := os.ReadFile(publicPath)
					if err != nil {
						return err
					}
					publicKey = string(data)
				}

				if privateInline != "" {
					privateKey = privateInline
				} else if privatePath != "" {
					data, err := os.ReadFile(privatePath)
					if err != nil {
						return err
					}
					privateKey = string(data)
				}
			}

			if publicKey == "" || privateKey == "" {
				return fmt.Errorf("provide both public and private keys or use --generate")
			}

			if err := secrets.SaveServiceKey(&secrets.ServiceKey{
				PubSvcKey:  publicKey,
				PrivSvcKey: privateKey,
			}); err != nil {
				return err
			}
			if generate {
				ui.FSuccess(cmd.OutOrStdout(), "Service keypair generated and saved")
			} else {
				ui.FSuccess(cmd.OutOrStdout(), "Service keypair saved")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&publicPath, "public", "", "Path to the public key PEM file")
	cmd.Flags().StringVar(&privatePath, "private", "", "Path to the private key PEM file")
	cmd.Flags().StringVar(&publicInline, "public-key", "", "Inline public key PEM")
	cmd.Flags().StringVar(&privateInline, "private-key", "", "Inline private key PEM")
	cmd.Flags().BoolVar(&generate, "generate", false, "Generate a new RSA keypair")
	cmd.Flags().IntVar(&bits, "bits", 4096, "RSA key size when generating")
	return cmd
}
