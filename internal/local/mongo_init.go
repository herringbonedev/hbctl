package local

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
)

type MongoInitOptions struct {
	Project    string
	ScriptPath string
	Enterprise bool
	DryRun     bool
}

func RunMongoInit(opts MongoInitOptions) error {
	ui.Header("Herringbone MongoDB init")

	scriptPath, err := findMongoInitScript(opts.ScriptPath)
	if err != nil {
		return err
	}

	ui.KeyValues([][2]string{
		{"project", opts.Project},
		{"script", scriptPath},
		{"enterprise", ui.Bool(opts.Enterprise)},
		{"dry run", ui.Bool(opts.DryRun)},
	})

	if opts.DryRun {
		ui.Command("ensure MongoDB is reachable without recreating or removing volumes")
		ui.Command("replay %s inside the running MongoDB container", scriptPath)
		if opts.Enterprise {
			ui.Command("ensure enterprise platform/org seed data")
		}
		return nil
	}

	sec, err := secrets.LoadMongo()
	if err != nil {
		return fmt.Errorf("failed to load MongoDB secret: %w", err)
	}
	if strings.TrimSpace(sec.Database) == "" {
		sec.Database = "herringbone"
	}
	if strings.TrimSpace(sec.AuthSource) == "" {
		sec.AuthSource = sec.Database
	}

	if err := ensureCoreDatabase(opts.Project, sec); err != nil {
		return err
	}

	ui.Section("MongoDB seed data")
	ui.Step("Replaying %s inside the running MongoDB container", scriptPath)
	if err := runMongoInitScriptFileInContainer(opts.Project, scriptPath); err != nil {
		return err
	}
	ui.Success("MongoDB init script replay complete")

	if opts.Enterprise {
		if err := ensureEnterpriseMongoSeedData(opts.Project, sec); err != nil {
			return err
		}
	} else {
		ui.Skip("Enterprise platform/org seed data: core/free mode")
	}

	return nil
}

func findMongoInitScript(explicitPath string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(explicitPath) != "" {
		candidates = append(candidates, strings.TrimSpace(explicitPath))
	} else {
		candidates = append(candidates,
			"init-mongo.js",
			filepath.Join("docker", "init-mongo.js"),
		)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			abs, absErr := filepath.Abs(candidate)
			if absErr == nil {
				return abs, nil
			}
			return candidate, nil
		}
	}

	if strings.TrimSpace(explicitPath) != "" {
		return "", fmt.Errorf("Mongo init script not found: %s", explicitPath)
	}
	return "", fmt.Errorf("Mongo init script not found. Run from the docker directory, repo root, or pass --file <path>")
}

func runMongoInitScriptFileInContainer(project string, scriptPath string) error {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("Mongo init script is empty: %s", scriptPath)
	}
	return runMongoJavaScriptInContainer(project, string(data))
}
