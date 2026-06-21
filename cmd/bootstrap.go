package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

type bootstrapAttemptResult struct {
	Endpoint string
	Status   int
	Message  string
}

func bootstrapCommand() *cobra.Command {
	var email string
	var password string
	var bootstrapTokenFile string
	var bootstrapTokenInline string
	var serverURL string
	var registerPath string
	var claimPath string
	var loginPath string
	var enterprise bool
	var timeoutSeconds int

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Register the first Herringbone user",
		Long: "Register the first Herringbone user using the bootstrap token. " +
			"The created account is logged in and the returned user token is stored in the hbctl session file. " +
			"When --enterprise is supplied, hbctl also claims the enterprise platform org for that user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(email) == "" {
				return fmt.Errorf("--email is required")
			}
			if strings.TrimSpace(password) == "" {
				return fmt.Errorf("--password is required")
			}

			bootstrapToken, tokenSource, err := loadBootstrapTokenForCommand(bootstrapTokenFile, bootstrapTokenInline)
			if err != nil {
				return err
			}

			baseURL := resolveCommandServerURL(serverURL, "")
			client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}

			ui.FHeader(cmd.OutOrStdout(), "Herringbone bootstrap")
			ui.FKeyValues(cmd.OutOrStdout(), [][2]string{
				{"server", baseURL},
				{"email", email},
				{"bootstrap token", tokenSource},
				{"mode", mapBool(enterprise, "enterprise", "core/free")},
			})

			ui.FStep(cmd.OutOrStdout(), "Registering first user")
			registered, err := registerFirstUser(client, baseURL, registerPath, email, password, bootstrapToken)
			if err != nil {
				return err
			}
			if registered != nil && strings.TrimSpace(registered.Message) != "" {
				ui.FInfo(cmd.OutOrStdout(), "%s", registered.Message)
			}

			ui.FStep(cmd.OutOrStdout(), "Logging in and storing account token")
			result, err := loginToAuth(email, password, baseURL, loginPath, time.Duration(timeoutSeconds)*time.Second)
			if err != nil {
				return fmt.Errorf("registered user but login failed: %w", err)
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
				ui.FStep(cmd.OutOrStdout(), "Claiming enterprise platform org")
				claimed, err := claimEnterprisePlatform(client, result.AuthURL, claimPath, result.Token, bootstrapToken)
				if err != nil {
					return err
				}
				ui.FSuccess(cmd.OutOrStdout(), "Enterprise platform org claimed")
				if claimed != nil && strings.TrimSpace(claimed.ContextID) != "" {
					currentContext = &secrets.ContextToken{
						AccessToken: result.Token,
						TokenType:   result.TokenType,
						ContextID:   claimed.ContextID,
						Slug:        "platform",
						Name:        "Platform",
						Role:        claimed.Role,
						SavedAt:     time.Now().UTC().Format(time.RFC3339),
					}
				}
			}

			if err := secrets.SaveAuthTokenSession(authToken, enterprise, currentContext); err != nil {
				return fmt.Errorf("failed to store auth token: %w", err)
			}
			ui.FSuccess(cmd.OutOrStdout(), "Account token saved to hbctl session file")

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

	cmd.Flags().StringVar(&email, "email", "", "First user email")
	cmd.Flags().StringVarP(&email, "user", "u", "", "First user email alias")
	cmd.Flags().StringVarP(&password, "password", "p", "", "First user password")
	cmd.Flags().StringVar(&bootstrapTokenFile, "bootstrap-token-file", "", "Path to bootstrap token file; defaults to secrets/runtime/bootstrap_token when present")
	cmd.Flags().StringVar(&bootstrapTokenInline, "bootstrap-token", "", "Bootstrap token value; prefer --bootstrap-token-file for shell history safety")
	cmd.Flags().StringVar(&serverURL, "server", "", "Override saved Herringbone API server base URL")
	cmd.Flags().StringVar(&serverURL, "auth-url", "", "Deprecated alias for --server")
	cmd.Flags().StringVar(&registerPath, "register-path", "", "Override first-user registration path")
	cmd.Flags().StringVar(&loginPath, "login-path", "", "Override login path after registration")
	cmd.Flags().StringVar(&claimPath, "claim-path", "/herringbone/auth/enterprise/platform/claim", "Enterprise platform claim path")
	cmd.Flags().BoolVar(&enterprise, "enterprise", false, "Claim the enterprise platform org after registering and logging in")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 15, "Request timeout in seconds")
	return cmd
}

func loadBootstrapTokenForCommand(path string, inline string) (string, string, error) {
	if token := strings.TrimSpace(inline); token != "" {
		return token, "inline", nil
	}

	candidates := []string{}
	if strings.TrimSpace(path) != "" {
		candidates = append(candidates, strings.TrimSpace(path))
	} else {
		candidates = append(candidates, "secrets/runtime/bootstrap_token")
		if base, err := secrets.BaseDir(); err == nil && strings.TrimSpace(base) != "" {
			candidates = append(candidates, base+string(os.PathSeparator)+"runtime"+string(os.PathSeparator)+"bootstrap_token")
		}
	}

	var lastErr error
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		if token := strings.TrimSpace(string(data)); token != "" {
			return token, candidate, nil
		}
		lastErr = fmt.Errorf("%s is empty", candidate)
	}

	if lastErr != nil {
		return "", "", fmt.Errorf("bootstrap token not found. Use --bootstrap-token-file <path>: %w", lastErr)
	}
	return "", "", fmt.Errorf("bootstrap token not found. Use --bootstrap-token-file <path>")
}

func registerFirstUser(client *http.Client, baseURL string, explicitPath string, email string, password string, bootstrapToken string) (*bootstrapAttemptResult, error) {
	paths := registrationPaths(explicitPath)
	payloads := []map[string]any{
		{"email": email, "password": password},
		{"email": email, "password": password, "bootstrap_token": bootstrapToken},
		{"user": map[string]any{"email": email, "password": password}},
		{"username": email, "email": email, "password": password},
		{"username": email, "email": email, "password": password, "bootstrap_token": bootstrapToken},
	}

	var lastErr error
	for _, base := range loginBaseURLs(baseURL) {
		for _, path := range paths {
			endpoint, err := joinURL(base, path)
			if err != nil {
				lastErr = err
				continue
			}
			for _, payload := range payloads {
				status, body, retry, err := postBootstrapJSON(client, endpoint, bootstrapToken, payload)
				if err == nil {
					return &bootstrapAttemptResult{Endpoint: endpoint, Status: status, Message: responseMessage(body)}, nil
				}
				if status == http.StatusConflict {
					return &bootstrapAttemptResult{Endpoint: endpoint, Status: status, Message: "first user already exists; continuing with login"}, nil
				}
				lastErr = err
				if !retry {
					return nil, err
				}
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("first-user registration failed: %w", lastErr)
	}
	return nil, errors.New("first-user registration failed: no registration endpoints were attempted")
}

func registrationPaths(explicit string) []string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		if !strings.HasPrefix(explicit, "/") {
			explicit = "/" + explicit
		}
		return []string{explicit}
	}
	return []string{
		"/herringbone/auth/bootstrap/register",
		"/herringbone/auth/bootstrap/admin",
		"/herringbone/auth/bootstrap",
		"/herringbone/auth/register",
		"/herringbone/auth/users/bootstrap",
		"/bootstrap/register",
		"/bootstrap",
		"/register",
	}
}

func postBootstrapJSON(client *http.Client, endpoint string, bootstrapToken string, payload any) (int, []byte, bool, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, false, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", bootstrapToken)
	req.Header.Set("x-bootstrap-token", bootstrapToken)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, true, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, data, false, nil
	}

	message := strings.TrimSpace(string(data))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return resp.StatusCode, data, false, fmt.Errorf("%s rejected bootstrap token: http %d: %s", endpoint, resp.StatusCode, message)
	}

	if resp.StatusCode == http.StatusConflict {
		return resp.StatusCode, data, false, fmt.Errorf("%s returned conflict: %s", endpoint, message)
	}

	return resp.StatusCode, data, true, fmt.Errorf("%s returned http %d: %s", endpoint, resp.StatusCode, message)
}

func responseMessage(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	for _, key := range []string{"message", "detail", "status"} {
		if s, ok := raw[key].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

type platformClaimResult struct {
	OK        bool   `json:"ok"`
	ContextID string `json:"context_id"`
	Role      string `json:"role"`
}

func claimEnterprisePlatform(client *http.Client, baseURL string, claimPath string, accessToken string, bootstrapToken string) (*platformClaimResult, error) {
	claimPath = strings.TrimSpace(claimPath)
	if claimPath == "" {
		claimPath = "/herringbone/auth/enterprise/platform/claim"
	}
	endpoint, err := joinURL(baseURL, claimPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("X-Bootstrap-Token", bootstrapToken)
	req.Header.Set("x-bootstrap-token", bootstrapToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result platformClaimResult
		_ = json.Unmarshal(data, &result)
		return &result, nil
	}
	return nil, fmt.Errorf("platform claim failed: %s returned http %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(data)))
}
