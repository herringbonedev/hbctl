package local

import (
	"fmt"
	"path/filepath"

	"github.com/herringbonedev/hbctl/internal/secrets"
)

func blankLifecycleEnv(enterprise bool) map[string]string {
	env := map[string]string{
		"MONGO_ROOT_PASS":                 "",
		"MONGO_HOST":                      "",
		"MONGO_PORT":                      "",
		"MONGO_USER":                      "",
		"MONGO_PASS":                      "",
		"DB_NAME":                         "",
		"AUTH_DB":                         "",
		"RECEIVER_TYPE":                   "",
		"MATCHER_API":                     "",
		"HB_ENTERPRISE":                   fmt.Sprintf("%t", enterprise),
		"COMPOSE_IGNORE_ORPHANS":          "true",
		"COMPOSE_PROGRESS":                "plain",
		"COMPOSE_PROFILES":                "ops",
		"FINGERPRINT_IDENTIFIER_REPLICAS": "1",
		"LOGINGESTION_RECEIVER_REPLICAS":  "1",
	}

	if secrets.HasBaseDirOverride() {
		if base, err := secrets.BaseDir(); err == nil {
			env["HBCTL_SECRETS_DIR"] = base
			env["HB_SECRETS_DIR"] = base
			env["HERRINGBONE_SECRETS_DIR"] = base
			env["RUNTIME_SECRETS_DIR"] = filepath.Join(base, "runtime")
		}
	}

	return env
}

func mongoLifecycleEnv(enterprise bool) (map[string]string, error) {
	sec, err := secrets.LoadMongo()
	if err != nil {
		return nil, fmt.Errorf("failed to load MongoDB secret: %w", err)
	}

	env := blankLifecycleEnv(enterprise)
	env["MONGO_HOST"] = sec.Host
	env["MONGO_PORT"] = fmt.Sprintf("%d", sec.Port)
	env["MONGO_USER"] = sec.User
	env["MONGO_PASS"] = sec.Password
	env["DB_NAME"] = sec.Database
	env["AUTH_DB"] = sec.AuthSource
	return env, nil
}
