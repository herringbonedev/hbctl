package local

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/herringbonedev/hbctl/internal/ui"
)

const (
	ComposeMongo                 = "compose.mongo.yml"
	ComposeReceiver              = "compose.logingestion.receiver.yml"
	ComposeLogs                  = "compose.herringbone.logs.yml"
	ComposeParserCardset         = "compose.parser.cardset.yml"
	ComposeParserEnrich          = "compose.parser.enrichment.yml"
	ComposeParserExtract         = "compose.parser.extractor.yml"
	ComposeDetector              = "compose.detectionengine.detector.yml"
	ComposeMatcher               = "compose.detectionengine.matcher.yml"
	ComposeRuleset               = "compose.detectionengine.ruleset.yml"
	ComposeOperationsCenter      = "compose.operations.center.yml"
	ComposeIncidentSet           = "compose.incidents.incidentset.yml"
	ComposeIncidentCorrelator    = "compose.incidents.correlator.yml"
	ComposeIncidentOrchestrator  = "compose.incidents.orchestrator.yml"
	ComposeSearch                = "compose.herringbone.search.yml"
	ComposeAuth                  = "compose.herringbone.auth.yml"
	ComposeProxy                 = "compose.proxy.yml"
	ComposeFingerprintIdentifier = "compose.fingerprint.identifier.yml"
	ComposeFingerprintScoreset   = "compose.fingerprint.scoreset.yml"
	ComposeFingerprintTuner      = "compose.fingerprint.tuner.yml"
	ComposeOllama                = "compose.ollama.yml"
)

// CanonicalElementName returns the real compose service name used by
// both core and enterprise deployments. Enterprise mode is selected by the
// active compose files/images and HB_ENTERPRISE, not by renaming services.
func CanonicalElementName(element string) string {
	switch strings.TrimSpace(element) {
	case "auth", "herringbone-auth", "herringbone-auth-e":
		return "herringbone-auth"
	case "proxy":
		return "herringbone-proxy"
	case "mongo", "mongodb":
		return "mongodb"
	case "ollama", "llm", "llm-ollama", "local-llm":
		return "ollama"
	case "fingerprint-identifier", "fingerprint-identifier-e":
		return "fingerprint-identifier"
	case "fingerprint-scoreset", "fingerprint-scoreset-e":
		return "fingerprint-scoreset"
	case "fingerprint-tuner", "fingerprint-tuner-e":
		return "fingerprint-tuner"
	case "parser-enrichment", "parser-enrichment-e":
		return "parser-enrichment"
	default:
		return strings.TrimSpace(element)
	}
}

func ComposeFilesForElement(element string) []string {
	element = CanonicalElementName(element)
	files := []string{"-f", ComposeMongo}

	switch element {
	case "logingestion-receiver":
		files = append(files, "-f", ComposeReceiver)
	case "herringbone-logs":
		files = append(files, "-f", ComposeLogs)
	case "parser-cardset":
		files = append(files, "-f", ComposeParserCardset)
	case "parser-enrichment":
		files = append(files, "-f", ComposeFingerprintScoreset, "-f", ComposeFingerprintIdentifier, "-f", ComposeParserEnrich)
	case "parser-extractor":
		files = append(files, "-f", ComposeParserExtract)
	case "detectionengine-detector":
		files = append(files, "-f", ComposeDetector)
	case "detectionengine-matcher":
		files = append(files, "-f", ComposeMatcher)
	case "detectionengine-ruleset":
		files = append(files, "-f", ComposeRuleset)
	case "operations-center":
		files = append(files, "-f", ComposeOperationsCenter)
	case "incidents-incidentset":
		files = append(files, "-f", ComposeIncidentSet)
	case "incidents-correlator":
		files = append(files, "-f", ComposeIncidentCorrelator)
	case "incidents-orchestrator":
		files = append(files, "-f", ComposeIncidentOrchestrator)
	case "herringbone-search":
		files = append(files, "-f", ComposeSearch)
	case "herringbone-auth":
		files = append(files, "-f", ComposeAuth)
	case "fingerprint-identifier":
		files = append(files, "-f", ComposeFingerprintScoreset, "-f", ComposeFingerprintIdentifier)
	case "fingerprint-scoreset":
		files = append(files, "-f", ComposeFingerprintScoreset)
	case "fingerprint-tuner":
		files = append(files, "-f", ComposeFingerprintTuner)
	case "ollama":
		files = append(files, "-f", ComposeOllama)
	case "herringbone-proxy":
		files = append(files, "-f", ComposeProxy)
	}

	return files
}

func ComposeFilesForFullStack(enterprise bool) []string {
	candidates := []string{
		ComposeMongo,
		ComposeProxy,
		ComposeAuth,
		ComposeLogs,
		ComposeSearch,
		ComposeParserCardset,
		ComposeParserExtract,
		ComposeDetector,
		ComposeMatcher,
		ComposeRuleset,
		ComposeIncidentSet,
		ComposeIncidentCorrelator,
		ComposeIncidentOrchestrator,
		ComposeOperationsCenter,
		ComposeReceiver,
	}
	if enterprise {
		candidates = append(candidates, ComposeFingerprintScoreset, ComposeFingerprintIdentifier, ComposeOllama, ComposeFingerprintTuner, ComposeParserEnrich)
	}

	seen := map[string]bool{}
	args := []string{}
	for _, file := range candidates {
		file = strings.TrimSpace(file)
		if file == "" || seen[file] {
			continue
		}
		if _, err := os.Stat(file); err != nil {
			continue
		}
		seen[file] = true
		args = append(args, "-f", file)
	}
	return args
}

func ComposeFilesForElements(elements []string) []string {
	seen := map[string]bool{}
	args := []string{}
	for _, element := range elements {
		files := ComposeFilesForElement(element)
		for i := 0; i < len(files); i++ {
			if files[i] != "-f" || i+1 >= len(files) {
				continue
			}
			file := strings.TrimSpace(files[i+1])
			i++
			if file == "" || seen[file] {
				continue
			}
			if _, err := os.Stat(file); err != nil {
				continue
			}
			seen[file] = true
			args = append(args, "-f", file)
		}
	}
	return args
}

// IsEnterpriseElement returns true for logical services that are only included
// when the operator explicitly passes --enterprise. It intentionally does not
// inspect container names or require a service-name suffix.
func IsEnterpriseElement(element string) bool {
	switch CanonicalElementName(element) {
	case "fingerprint-scoreset", "fingerprint-identifier", "fingerprint-tuner", "parser-enrichment":
		return true
	default:
		return false
	}
}

func ElementForMode(element string, enterprise bool) string {
	return CanonicalElementName(element)
}

func AuthElementForMode(enterprise bool) string {
	return "herringbone-auth"
}

func filterEnterpriseElements(elements []string, enterprise bool) []string {
	out := make([]string, 0, len(elements))
	for _, element := range elements {
		element = CanonicalElementName(element)
		if IsEnterpriseElement(element) && !enterprise {
			continue
		}
		out = append(out, element)
	}
	return out
}

func validateCoreComposeFiles() error {
	required := map[string]string{
		"mongo": ComposeMongo,
		"proxy": ComposeProxy,
		"auth":  ComposeAuth,
	}

	for name, file := range required {
		if _, err := os.Stat(file); err != nil {
			return fmt.Errorf("required %s compose file missing: %s", name, file)
		}
	}

	return nil
}

func operableElements(elements []string) ([]string, error) {
	out := make([]string, 0, len(elements))
	for _, element := range elements {
		element = CanonicalElementName(element)
		start, reason, err := shouldStartElement(element, ComposeFilesForElement(element))
		if err != nil {
			return nil, err
		}
		if !start {
			ui.Skip("%s: %s", element, reason)
			continue
		}
		out = append(out, element)
	}
	return out, nil
}

func composeServiceAliases(element string) []string {
	canonical := CanonicalElementName(element)
	return []string{canonical}
}

func resolveComposeServiceName(composeArgs []string, element string) (string, error) {
	canonical := CanonicalElementName(element)
	services, err := composeConfigServices(composeArgs)
	if err != nil {
		// Starting/stopping will still surface a real compose error if this guess is
		// wrong. We do not fail early here because docker compose config can fail on
		// older compose files with required environment interpolation.
		return canonical, nil
	}

	available := map[string]string{}
	for _, svc := range services {
		available[CanonicalElementName(svc)] = svc
		available[strings.TrimSpace(svc)] = svc
	}

	for _, alias := range composeServiceAliases(canonical) {
		if actual, ok := available[alias]; ok {
			return actual, nil
		}
		if actual, ok := available[CanonicalElementName(alias)]; ok {
			return actual, nil
		}
	}

	return "", fmt.Errorf("compose service for %q not found. Available services: %s", canonical, strings.Join(services, ", "))
}

func composeConfigEnv() []string {
	env := os.Environ()
	hasProfiles := false
	for _, item := range env {
		if strings.HasPrefix(item, "COMPOSE_PROFILES=") {
			hasProfiles = true
			break
		}
	}
	if !hasProfiles {
		env = append(env, "COMPOSE_PROFILES=ops")
	}
	return env
}

func composeConfigServices(composeArgs []string) ([]string, error) {
	args := append([]string{}, composeArgs...)
	args = append(args, "config", "--services")

	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Env = composeConfigEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("docker compose config --services failed: %s", msg)
		}
		return nil, err
	}

	services := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		services = append(services, line)
	}
	return services, nil
}
