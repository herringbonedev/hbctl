package local

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/herringbonedev/hbctl/internal/units"
)

type RestartOptions struct {
	Project    string
	Element    string
	Unit       string
	All        bool
	Enterprise bool
}

func Restart(opts RestartOptions) error {
	env := blankLifecycleEnv(opts.Enterprise)
	if restartTargetNeedsMongo(opts) {
		var err error
		env, err = mongoLifecycleEnv(opts.Enterprise)
		if err != nil {
			return err
		}
	}
	ui.Header("Herringbone restart")

	if opts.Element != "" {
		element := ElementForMode(opts.Element, opts.Enterprise)
		if element == "logingestion-receiver" {
			return fmt.Errorf("logingestion-receiver is managed separately; use hbctl receiver restart instead")
		}
		if IsEnterpriseElement(element) && !opts.Enterprise {
			return fmt.Errorf("%s is an enterprise service; pass --enterprise to restart it", element)
		}
		return restartElement(opts.Project, env, element)
	}

	if opts.Unit != "" {
		elements := units.UnitElements[strings.TrimSpace(opts.Unit)]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		if strings.TrimSpace(opts.Unit) == "auth" {
			elements = []string{AuthElementForMode(opts.Enterprise)}
		}
		ui.Section("Unit")
		ui.KeyValues([][2]string{{"unit", opts.Unit}, {"elements", fmt.Sprintf("%d", len(elements))}})
		elements = filterEnterpriseElements(elements, opts.Enterprise)
		operable, err := operableElements(elements)
		if err != nil {
			return err
		}
		for _, element := range operable {
			if element == "logingestion-receiver" {
				ui.Skip("logingestion-receiver: use hbctl receiver restart")
				continue
			}
			if err := restartElement(opts.Project, env, element); err != nil {
				return err
			}
		}
		ui.Success("Unit %s restarted", opts.Unit)
		return nil
	}

	if opts.All {
		if err := validateCoreComposeFiles(); err != nil {
			return err
		}
		ui.Section("Full stack")
		ui.Step("Restarting all available services")
		for _, element := range fullStackRestartElements(opts.Enterprise) {
			if err := restartElement(opts.Project, env, element); err != nil {
				return err
			}
		}
		ui.Success("Full stack restarted")
		return nil
	}

	return fmt.Errorf("specify --element, --unit, or --all")
}

func restartTargetNeedsMongo(opts RestartOptions) bool {
	if opts.All {
		return true
	}
	if strings.TrimSpace(opts.Element) != "" {
		return elementRequiresMongoDiscovery(ElementForMode(opts.Element, opts.Enterprise))
	}
	if strings.TrimSpace(opts.Unit) != "" {
		elements := units.UnitElements[strings.TrimSpace(opts.Unit)]
		if strings.TrimSpace(opts.Unit) == "auth" {
			elements = []string{AuthElementForMode(opts.Enterprise)}
		}
		for _, element := range elements {
			if elementRequiresMongoDiscovery(element) {
				return true
			}
		}
	}
	return false
}

func fullStackRestartElements(enterprise bool) []string {
	elements := []string{"mongodb", "herringbone-proxy", AuthElementForMode(enterprise), "herringbone-logs", "herringbone-search", "parser-cardset", "parser-extractor", "detectionengine-detector", "detectionengine-matcher", "detectionengine-ruleset", "incidents-incidentset", "incidents-correlator", "incidents-orchestrator", "operations-center"}
	if enterprise {
		elements = append(elements, "fingerprint-scoreset", "fingerprint-identifier", "ollama", "fingerprint-tuner", "parser-enrichment")
	}
	return elements
}

func restartElement(project string, env map[string]string, element string) error {
	element = CanonicalElementName(element)
	if IsEnterpriseElement(element) && !strings.EqualFold(strings.TrimSpace(env["HB_ENTERPRISE"]), "true") {
		ui.Skip("%s: enterprise service requires --enterprise", element)
		return nil
	}
	start, reason, err := shouldStartElement(element, ComposeFilesForElement(element))
	if err != nil {
		return err
	}
	if !start {
		ui.Skip("%s: %s", element, reason)
		return nil
	}
	if elementRequiresMongoDiscovery(element) {
		if err := ensureMongoServiceDiscovery(project, env); err != nil {
			return err
		}
	}
	ui.Step("Restarting %s", element)
	composeArgs := []string{"-p", project}
	composeArgs = append(composeArgs, ComposeFilesForElement(element)...)
	service, err := resolveComposeServiceName(ComposeFilesForElement(element), element)
	if err != nil {
		return err
	}
	composeArgs = append(composeArgs, "restart", service)
	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		return err
	}
	ui.Success("%s restarted", element)
	return nil
}
