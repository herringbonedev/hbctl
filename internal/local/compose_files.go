package local

const (
	ComposeMongo                = "compose.mongo.yml"
	ComposeReceiver             = "compose.logingestion.receiver.yml"
	ComposeLogs                 = "compose.herringbone.logs.yml"
	ComposeParserCardset        = "compose.parser.cardset.yml"
	ComposeParserEnrich         = "compose.parser.enrichment.yml"
	ComposeParserExtract        = "compose.parser.extractor.yml"
	ComposeDetector             = "compose.detectionengine.detector.yml"
	ComposeMatcher              = "compose.detectionengine.matcher.yml"
	ComposeRuleset              = "compose.detectionengine.ruleset.yml"
	ComposeOperationsCenter     = "compose.operations.center.yml"
	ComposeIncidentSet          = "compose.incidents.incidentset.yml"
	ComposeIncidentCorrelator   = "compose.incidents.correlator.yml"
	ComposeIncidentOrchestrator = "compose.incidents.orchestrator.yml"
	ComposeSearch               = "compose.herringbone.search.yml"
	ComposeAuth					= "compose.herringbone.auth.yml"
)

func ComposeFilesForElement(element string) []string {
	files := []string{"-f", ComposeMongo}

	switch element {
	case "logingestion-receiver":
		files = append(files, "-f", ComposeReceiver)
	case "herringbone-logs":
		files = append(files, "-f", ComposeLogs)
	case "parser-cardset":
		files = append(files, "-f", ComposeParserCardset)
	case "parser-enrichment":
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
	case "herringbone-auth":
		files = append(files, "-f", ComposeAuth)
	}

	return files
}
