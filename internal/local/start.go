package local

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/docker"
	hbmongo "github.com/herringbonedev/hbctl/internal/mongo"
	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/units"
)

type StartOptions struct {
    Project     string
    Element     string
    Unit        string
    All         bool
    RecvType    string
}

func Start(opts StartOptions) error {
	fmt.Println("[hbctl] Decrypting secrets...")

	sec, err := secrets.LoadMongo()
	if err != nil {
		return fmt.Errorf("failed to load MongoDB secret: %w", err)
	}

	jwt, err := secrets.LoadJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to load JWT secret: %w", err)
	}

	svcKeys, err := secrets.LoadServiceKey()
	if err != nil {
		return fmt.Errorf("failed to load service keys: %w", err)
	}

	// base env for all services (no secrets)
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

		for _, e := range units.AllElements {

			if e.Name == "logingestion-receiver" {
				env["RECEIVER_TYPE"] = "UDP"
			}

			env["SERVICE_JWT_PUBLIC_KEY"] = svcKeys.PubSvcKey

			if e.Name == "herringbone-auth" {
				env["JWT_SECRET"] = jwt.JWTSecret
				env["SERVICE_JWT_PRIVATE_KEY"] = svcKeys.PrivSvcKey
			}

			if err := startElement(opts.Project, env, e.Name); err != nil {
				return err
			}

			delete(env, "JWT_SECRET")
			delete(env, "SERVICE_JWT_PRIVATE_KEY")
			delete(env, "SERVICE_JWT_PUBLIC_KEY")
		}

		return nil
	}

	if opts.Unit != "" {
		if err := ensureDatabase(opts.Project, sec); err != nil {
			return err
		}

		elements := units.UnitElements[opts.Unit]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}

		for _, el := range elements {

			if el == "herringbone-auth" {
				env["JWT_SECRET"] = jwt.JWTSecret
				env["SERVICE_JWT_PRIVATE_KEY"] = svcKeys.PrivSvcKey
				env["SERVICE_JWT_PUBLIC_KEY"] = svcKeys.PubSvcKey
			}

			if err := startElement(opts.Project, env, el); err != nil {
				return err
			}

			delete(env, "JWT_SECRET")
			delete(env, "SERVICE_JWT_PRIVATE_KEY")
			delete(env, "SERVICE_JWT_PUBLIC_KEY")
		}

		return nil
	}

	if opts.Element != "" {
		if opts.Element == "logingestion-receiver" && opts.RecvType == "" {
			return fmt.Errorf("error: --type required for receiver")
		}

		if err := ensureDatabase(opts.Project, sec); err != nil {
			return err
		}

		if opts.RecvType != "" {
			env["RECEIVER_TYPE"] = strings.ToUpper(opts.RecvType)
		}

		if opts.Element == "herringbone-auth" {
			env["JWT_SECRET"] = jwt.JWTSecret
			env["SERVICE_JWT_PRIVATE_KEY"] = svcKeys.PrivSvcKey
			env["SERVICE_JWT_PUBLIC_KEY"] = svcKeys.PubSvcKey
		}

		return startElement(opts.Project, env, opts.Element)
	}

	return fmt.Errorf("error: specify --element, --unit, or --all")
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

func randomPassword(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
