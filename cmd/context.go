package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

type enterpriseContextInfo struct {
	ContextID string   `json:"context_id"`
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Role      string   `json:"role"`
	OrgScopes []string `json:"org_scopes,omitempty"`
	Status    string   `json:"status,omitempty"`
}

func contextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Show or select the enterprise context stored in the hbctl session",
		Long:  "Manage the current enterprise context stored in ~/.hbctl/session.json. This does not unlock secrets.enc.",
	}
	cmd.AddCommand(contextShowCommand())
	cmd.AddCommand(contextListCommand())
	cmd.AddCommand(contextSetCommand())
	cmd.AddCommand(contextClearCommand())
	return cmd
}

func contextShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current session context",
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := secrets.LoadSession()
			if err != nil {
				return fmt.Errorf("no hbctl session found. Run: hbctl login -u <email> -p <password>")
			}
			ui.FHeader(cmd.OutOrStdout(), "Herringbone context")
			rows := [][2]string{{"enterprise", mapBool(session.Enterprise, "true", "false")}}
			if session.CurrentContextToken != nil && strings.TrimSpace(session.CurrentContextToken.ContextID) != "" {
				ctx := session.CurrentContextToken
				rows = append(rows,
					[2]string{"context id", ctx.ContextID},
					[2]string{"slug", ctx.Slug},
					[2]string{"name", ctx.Name},
					[2]string{"role", ctx.Role},
				)
				rows = append(rows, [2]string{"saved at", ctx.SavedAt})
			} else {
				rows = append(rows, [2]string{"current context", "none"})
			}
			ui.FKeyValues(cmd.OutOrStdout(), rows)
			return nil
		},
	}
}

func contextListCommand() *cobra.Command {
	var serverURL string
	var jsonOut bool
	var timeoutSeconds int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List enterprise contexts available to the logged-in user",
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, err := secrets.LoadAuthToken()
			if err != nil {
				return fmt.Errorf("no stored auth token. Run: hbctl login -u <email> -p <password>")
			}
			baseURL := resolveCommandServerURL(serverURL, tok.AuthURL)
			client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
			contexts, err := fetchEnterpriseContexts(client, baseURL, tok.AccessToken)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"contexts": contexts})
			}
			ui.FHeader(cmd.OutOrStdout(), "Herringbone contexts")
			rows := [][]string{}
			for _, ctx := range contexts {
				rows = append(rows, []string{ctx.Slug, ctx.Name, ctx.Role, ctx.ContextID})
			}
			if len(rows) == 0 {
				ui.FInfo(cmd.OutOrStdout(), "No enterprise contexts found for this user")
				return nil
			}
			ui.FTable(cmd.OutOrStdout(), []string{"SLUG", "NAME", "ROLE", "CONTEXT ID"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "server", "", "Override saved Herringbone API server base URL")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print contexts as JSON")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 10, "Request timeout in seconds")
	return cmd
}

func contextSetCommand() *cobra.Command {
	var serverURL string
	var timeoutSeconds int
	cmd := &cobra.Command{
		Use:   "set <slug-or-context-id>",
		Short: "Set the current enterprise context for the session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, err := secrets.LoadAuthToken()
			if err != nil {
				return fmt.Errorf("no stored auth token. Run: hbctl login -u <email> -p <password>")
			}
			baseURL := resolveCommandServerURL(serverURL, tok.AuthURL)
			client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
			contexts, err := fetchEnterpriseContexts(client, baseURL, tok.AccessToken)
			if err != nil {
				return err
			}
			ctx, ok := findEnterpriseContext(contexts, args[0])
			if !ok {
				return fmt.Errorf("enterprise context %q not found", args[0])
			}
			contextToken := contextInfoToSessionToken(ctx, tok)
			if err := secrets.SaveCurrentContextSession(contextToken); err != nil {
				return fmt.Errorf("failed to save current context: %w", err)
			}
			ui.FHeader(cmd.OutOrStdout(), "Herringbone context")
			ui.FSuccess(cmd.OutOrStdout(), "Current enterprise context saved")
			kv := [][2]string{{"slug", ctx.Slug}, {"name", ctx.Name}, {"role", ctx.Role}, {"context id", ctx.ContextID}}
			ui.FKeyValues(cmd.OutOrStdout(), kv)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "server", "", "Override saved Herringbone API server base URL")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 10, "Request timeout in seconds")
	return cmd
}

func contextClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the current enterprise context from the session",
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := secrets.LoadSession()
			if err != nil {
				return fmt.Errorf("no hbctl session found")
			}
			session.CurrentContextToken = nil
			if err := secrets.SaveSession(session); err != nil {
				return err
			}
			ui.FSuccess(cmd.OutOrStdout(), "Current enterprise context cleared")
			return nil
		},
	}
}

func fetchEnterpriseContexts(client *http.Client, baseURL string, token string) ([]enterpriseContextInfo, error) {
	endpoint, err := joinURL(baseURL, "/herringbone/auth/enterprise/me")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("enterprise context lookup failed: http %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var body struct {
		Contexts       []enterpriseContextInfo `json:"contexts"`
		DefaultContext *enterpriseContextInfo  `json:"default_context"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return body.Contexts, nil
}

func selectEnterpriseContext(contexts []enterpriseContextInfo, preferred string) (*enterpriseContextInfo, bool) {
	if len(contexts) == 0 {
		return nil, false
	}
	if ctx, ok := findEnterpriseContext(contexts, preferred); ok {
		return ctx, true
	}
	if ctx, ok := findEnterpriseContext(contexts, "platform"); ok {
		return ctx, true
	}
	return &contexts[0], true
}

func findEnterpriseContext(contexts []enterpriseContextInfo, wanted string) (*enterpriseContextInfo, bool) {
	wanted = strings.TrimSpace(strings.ToLower(wanted))
	if wanted == "" {
		return nil, false
	}
	for i := range contexts {
		ctx := &contexts[i]
		if strings.ToLower(strings.TrimSpace(ctx.Slug)) == wanted || strings.ToLower(strings.TrimSpace(ctx.ContextID)) == wanted {
			return ctx, true
		}
	}
	return nil, false
}

func contextInfoToSessionToken(ctx *enterpriseContextInfo, tok *secrets.AuthToken) *secrets.ContextToken {
	if ctx == nil || tok == nil {
		return nil
	}
	return &secrets.ContextToken{
		AccessToken: tok.AccessToken,
		TokenType:   tok.TokenType,
		ContextID:   ctx.ContextID,
		Name:        ctx.Name,
		Slug:        ctx.Slug,
		Role:        ctx.Role,
		OrgScopes:   ctx.OrgScopes,
		SavedAt:     time.Now().UTC().Format(time.RFC3339),
	}
}
