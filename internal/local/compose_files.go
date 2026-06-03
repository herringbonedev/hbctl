package local

import (
	"os"
	"strings"
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
)

// CanonicalElementName keeps old operator muscle memory working while making the
// enterprise -e service names the canonical targets for enterprise-only services.
func CanonicalElementName(element string) string {
	switch strings.TrimSpace(element) {
	case "herringbone-auth":
		return "herringbone-auth-e"
	case "proxy":
		return "herringbone-proxy"
	case "fingerprint-identifier", "fingerprint-identifier-e":
		return "fingerprint-identifier-e"
	case "fingerprint-scoreset", "fingerprint-scoreset-e":
		return "fingerprint-scoreset-e"
	case "parser-enrichment", "parser-enrichment-e":
		return "parser-enrichment-e"
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
	case "parser-enrichment-e":
		files = append(files, "-f", ComposeParserEnrich)
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
	case "herringbone-auth-e":
		files = append(files, "-f", ComposeAuth)
	case "fingerprint-identifier-e":
		files = append(files, "-f", ComposeFingerprintIdentifier)
	case "fingerprint-scoreset-e":
		files = append(files, "-f", ComposeFingerprintScoreset)
	case "herringbone-proxy":
		files = append(files, "-f", ComposeProxy)
	}

	return files
}

func ComposeFilesForFullStack() []string {
	candidates := []string{
		ComposeMongo,
		ComposeProxy,
		ComposeAuth,
		ComposeLogs,
		ComposeSearch,
		ComposeFingerprintScoreset,
		ComposeFingerprintIdentifier,
		ComposeParserCardset,
		ComposeParserEnrich,
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
