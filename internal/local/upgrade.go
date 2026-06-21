package local

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/herringbonedev/hbctl/internal/units"
)

type UpgradeOptions struct {
	Project       string
	Element       string
	Unit          string
	All           bool
	Pull          bool
	ForceRecreate bool
	Enterprise    bool
	DryRun        bool
}

func Upgrade(opts UpgradeOptions) error {
	var env map[string]string
	if opts.DryRun {
		env = blankLifecycleEnv(opts.Enterprise)
	} else {
		var err error
		env, err = mongoLifecycleEnv(opts.Enterprise)
		if err != nil {
			return err
		}
	}

	printUpgradeHeader(opts)

	if opts.Element != "" {
		element := ElementForMode(opts.Element, opts.Enterprise)
		if element == "logingestion-receiver" {
			return fmt.Errorf("logingestion-receiver is managed separately; upgrade/recreate receivers with hbctl receiver commands")
		}
		if IsEnterpriseElement(element) && !opts.Enterprise {
			return fmt.Errorf("%s is an enterprise service; pass --enterprise to upgrade it", element)
		}
		return upgradeElement(opts.Project, env, element, opts.Pull, opts.ForceRecreate, opts.DryRun, true)
	}

	if opts.Unit != "" {
		elements := units.UnitElements[strings.TrimSpace(opts.Unit)]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		if strings.TrimSpace(opts.Unit) == "auth" {
			elements = []string{AuthElementForMode(opts.Enterprise)}
		}
		ui.Section("Upgrade target")
		ui.KeyValues([][2]string{{"unit", opts.Unit}, {"elements", fmt.Sprintf("%d", len(elements))}})
		for _, element := range elements {
			element = CanonicalElementName(element)
			if element == "logingestion-receiver" {
				ui.Skip("logingestion-receiver: use hbctl receiver commands")
				continue
			}
			if IsEnterpriseElement(element) && !opts.Enterprise {
				ui.Skip("%s: enterprise service requires --enterprise", element)
				continue
			}
			if err := upgradeElement(opts.Project, env, element, opts.Pull, opts.ForceRecreate, opts.DryRun, false); err != nil {
				return err
			}
		}
		ui.Success("Upgrade complete for unit %s", opts.Unit)
		return nil
	}

	if opts.All {
		plan := []string{
			"MongoDB is protected and will not be force-recreated by hbctl upgrade --all.",
			"Existing compose files are discovered from the current directory.",
			"Missing optional elements are skipped.",
			"Required core elements must exist: proxy, mongo, auth.",
			"Common MongoDB seed data from init-mongo.js is replayed before services are refreshed.",
		}
		if opts.Enterprise {
			plan = append(plan, "Enterprise platform/org seed data is ensured after common init-mongo.js replay.")
		} else {
			plan = append(plan, "Core/free mode skips enterprise platform/org seed data but still runs common init-mongo.js.")
		}
		plan = append(plan,
			"Each service is pulled and recreated one at a time with --no-deps.",
			"Receivers are managed separately with hbctl receiver commands and are not upgraded by --all.",
			"No docker compose down and no volume removal are used.",
		)
		ui.Plan("Full-stack upgrade safety plan", plan)

		if err := validateCoreComposeFiles(); err != nil {
			return err
		}

		if err := runUpgradeMongoSeedPlan(opts.Project, env, opts.Enterprise, opts.DryRun); err != nil {
			return err
		}

		for _, element := range fullStackUpgradeElements(opts.Enterprise) {
			if err := upgradeElement(opts.Project, env, element, opts.Pull, opts.ForceRecreate, opts.DryRun, false); err != nil {
				return err
			}
		}
		ui.Success("Full-stack upgrade complete")
		return nil
	}

	return fmt.Errorf("specify --element, --unit, or --all")
}

func runUpgradeMongoSeedPlan(project string, env map[string]string, enterprise bool, dryRun bool) error {
	ui.Section("MongoDB seed/migration data")

	if dryRun {
		ui.Command("ensure MongoDB is reachable without recreating or removing volumes")
		ui.Command("replay init-mongo.js inside the running MongoDB container")
		if enterprise {
			ui.Command("ensure enterprise platform/org seed data")
		} else {
			ui.Skip("Enterprise platform/org seed data: core/free mode")
		}
		return nil
	}

	sec := &secrets.MongoSecret{
		Host:       env["MONGO_HOST"],
		Port:       atoiDefault(env["MONGO_PORT"], 27017),
		User:       env["MONGO_USER"],
		Password:   env["MONGO_PASS"],
		Database:   env["DB_NAME"],
		AuthSource: env["AUTH_DB"],
	}
	if strings.TrimSpace(sec.Database) == "" {
		sec.Database = "herringbone"
	}
	if strings.TrimSpace(sec.AuthSource) == "" {
		sec.AuthSource = sec.Database
	}

	if err := ensureCoreDatabase(project, sec); err != nil {
		return err
	}
	if err := ensureCommonMongoSeedData(project); err != nil {
		return err
	}
	if enterprise {
		return ensureEnterpriseMongoSeedData(project, sec)
	}
	ui.Skip("Enterprise platform/org seed data: core/free mode")
	return nil
}

func atoiDefault(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	var out int
	if _, err := fmt.Sscanf(value, "%d", &out); err != nil || out <= 0 {
		return fallback
	}
	return out
}

func printUpgradeHeader(opts UpgradeOptions) {
	ui.Header("Herringbone safe upgrade")
	ui.KeyValues([][2]string{
		{"project", opts.Project},
		{"pull images", ui.Bool(opts.Pull)},
		{"force recreate", ui.Bool(opts.ForceRecreate)},
		{"enterprise env", ui.Bool(opts.Enterprise)},
		{"dry run", ui.Bool(opts.DryRun)},
	})
}

func fullStackUpgradeElements(enterprise bool) []string {
	elements := []string{
		"herringbone-proxy",
		AuthElementForMode(enterprise),
		"herringbone-logs",
		"herringbone-search",
		"parser-cardset",
		"parser-extractor",
		"detectionengine-detector",
		"detectionengine-matcher",
		"detectionengine-ruleset",
		"incidents-incidentset",
		"incidents-correlator",
		"incidents-orchestrator",
		"operations-center",
	}
	if enterprise {
		elements = append(elements, "fingerprint-scoreset", "fingerprint-identifier", "ollama", "fingerprint-tuner", "parser-enrichment")
	}
	return elements
}

func upgradeElement(project string, env map[string]string, element string, pull bool, forceRecreate bool, dryRun bool, explicit bool) error {
	element = CanonicalElementName(element)
	if IsEnterpriseElement(element) && !strings.EqualFold(strings.TrimSpace(env["HB_ENTERPRISE"]), "true") {
		if explicit {
			return fmt.Errorf("%s is an enterprise service; pass --enterprise to upgrade it", element)
		}
		ui.Skip("%s: enterprise service requires --enterprise", element)
		return nil
	}
	if element == "mongodb" {
		return fmt.Errorf("mongodb is protected. hbctl will not recreate MongoDB during upgrade; back up the database and upgrade MongoDB manually if needed")
	}

	composeFiles := ComposeFilesForElement(element)
	start, reason, err := shouldStartElement(element, composeFiles)
	if err != nil {
		return err
	}
	if !start {
		ui.Skip("%s: %s", element, reason)
		return nil
	}

	service, err := resolveComposeServiceName(composeFiles, element)
	if err != nil {
		return err
	}

	ui.Section(element)
	if elementRequiresMongoDiscovery(element) && !dryRun {
		if err := ensureMongoServiceDiscovery(project, env); err != nil {
			return err
		}
	}
	composeArgs := []string{"-p", project}
	composeArgs = append(composeArgs, composeFiles...)

	if pull {
		ui.Step("Pulling latest image for %s", service)
		pullArgs := append([]string{}, composeArgs...)
		pullArgs = append(pullArgs, "pull", service)
		if err := runComposeMaybe(dryRun, env, pullArgs...); err != nil {
			return err
		}
	}

	ui.Step("Recreating without tearing down dependencies")
	upArgs := append([]string{}, composeArgs...)
	upArgs = append(upArgs, "up", "-d", "--no-deps")
	if forceRecreate {
		upArgs = append(upArgs, "--force-recreate")
	}
	upArgs = append(upArgs, service)
	if err := runComposeMaybe(dryRun, env, upArgs...); err != nil {
		return err
	}
	ui.Success("%s upgraded", element)
	return nil
}

func runComposeMaybe(dryRun bool, env map[string]string, args ...string) error {
	if dryRun {
		ui.Command("docker compose %s", strings.Join(args, " "))
		return nil
	}
	return docker.ComposeWithEnv(env, args...)
}
