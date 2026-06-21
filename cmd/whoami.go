package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

func whoamiCommand() *cobra.Command {
	var jsonOut bool
	var showRaw bool

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Decode the stored Herringbone auth token",
		Long:  "Decode the stored Herringbone auth token locally. This command does not call /me or any other auth API route.",
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, err := secrets.LoadAuthToken()
			if err != nil {
				return fmt.Errorf("no stored auth token. Run: hbctl login -u <email> -p <password>")
			}

			decoded, err := decodeJWT(tok.AccessToken)
			if err != nil {
				return err
			}

			session, _ := secrets.LoadSession()

			if jsonOut {
				out := map[string]any{
					"stored_email": tok.Email,
					"auth_url":     tok.AuthURL,
					"login_path":   tok.LoginPath,
					"saved_at":     tok.SavedAt,
					"token_type":   tok.TokenType,
					"header":       decoded.Header,
					"claims":       decoded.Claims,
				}
				if session != nil {
					out["enterprise"] = session.Enterprise
					out["current_context"] = session.CurrentContextToken
				}
				if showRaw {
					out["token"] = tok.AccessToken
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			ui.FHeader(cmd.OutOrStdout(), "Herringbone token")

			rows := [][2]string{}
			if email := firstClaimString(decoded.Claims, "email", "preferred_username", "username", "upn"); email != "" {
				rows = append(rows, [2]string{"email", email})
			} else if strings.TrimSpace(tok.Email) != "" {
				rows = append(rows, [2]string{"email", tok.Email})
			}
			if typ := firstClaimString(decoded.Claims, "type", "token_type"); typ != "" {
				rows = append(rows, [2]string{"type", typ})
			}
			if sub := firstClaimString(decoded.Claims, "sub", "id", "user_id", "uid"); sub != "" {
				rows = append(rows, [2]string{"subject", sub})
			}
			if iss := firstClaimString(decoded.Claims, "iss", "issuer"); iss != "" {
				rows = append(rows, [2]string{"issuer", iss})
			}
			if aud := claimString(decoded.Claims["aud"]); aud != "" {
				rows = append(rows, [2]string{"audience", aud})
			}
			if exp := timeClaim(decoded.Claims, "exp"); !exp.IsZero() {
				rows = append(rows, [2]string{"expires", exp.Local().Format(time.RFC3339)})
				rows = append(rows, [2]string{"valid", tokenValidity(exp)})
			}
			if iat := timeClaim(decoded.Claims, "iat"); !iat.IsZero() {
				rows = append(rows, [2]string{"issued", iat.Local().Format(time.RFC3339)})
			}
			if session != nil {
				rows = append(rows, [2]string{"enterprise", mapBool(session.Enterprise, "true", "false")})
				if session.CurrentContextToken != nil && strings.TrimSpace(session.CurrentContextToken.ContextID) != "" {
					ctx := session.CurrentContextToken
					rows = append(rows, [2]string{"context", ctx.Slug}, [2]string{"context id", ctx.ContextID}, [2]string{"context role", ctx.Role})
				} else if session.Enterprise {
					rows = append(rows, [2]string{"context", "none selected"})
				}
			}
			if strings.TrimSpace(tok.AuthURL) != "" {
				rows = append(rows, [2]string{"login server", tok.AuthURL})
			}
			if strings.TrimSpace(tok.SavedAt) != "" {
				rows = append(rows, [2]string{"stored", tok.SavedAt})
			}

			if len(rows) == 0 {
				rows = append(rows, [2]string{"token", "decoded"})
			}
			ui.FKeyValues(cmd.OutOrStdout(), rows)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print decoded token claims as JSON")
	cmd.Flags().BoolVar(&showRaw, "show-token", false, "Include the raw token in --json output")
	_ = cmd.Flags().MarkHidden("show-token")
	return cmd
}

type decodedJWT struct {
	Header map[string]any
	Claims map[string]any
}

func decodeJWT(token string) (*decodedJWT, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("stored auth token is not a JWT")
	}

	headerBytes, err := decodeJWTPart(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}
	claimBytes, err := decodeJWTPart(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT claims: %w", err)
	}

	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to parse JWT header: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(claimBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return &decodedJWT{Header: header, Claims: claims}, nil
}

func decodeJWTPart(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func firstClaimString(claims map[string]any, keys ...string) string {
	for _, key := range keys {
		if v := claimString(claims[key]); v != "" {
			return v
		}
	}
	return ""
}

func claimString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := claimString(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(t, ", ")
	case float64:
		return fmt.Sprintf("%.0f", t)
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		return ""
	}
}

func scopesString(claims map[string]any) string {
	for _, key := range []string{"scopes", "scope", "scp", "permissions"} {
		v, ok := claims[key]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			return strings.Join(strings.Fields(t), ", ")
		case []any:
			parts := make([]string, 0, len(t))
			for _, item := range t {
				if s := claimString(item); s != "" {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, ", ")
		case []string:
			return strings.Join(t, ", ")
		}
	}
	return ""
}

func timeClaim(claims map[string]any, key string) time.Time {
	v, ok := claims[key]
	if !ok {
		return time.Time{}
	}
	switch t := v.(type) {
	case float64:
		return time.Unix(int64(t), 0)
	case int64:
		return time.Unix(t, 0)
	case json.Number:
		n, err := t.Int64()
		if err == nil {
			return time.Unix(n, 0)
		}
	case string:
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func tokenValidity(exp time.Time) string {
	now := time.Now()
	if now.After(exp) {
		return "expired"
	}
	return "valid for " + time.Until(exp).Round(time.Second).String()
}
