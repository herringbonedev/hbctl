package cmd

var unitElements = map[string][]string{
	"database":  {"mongodb"},
	"receiver":  {"logingestion-receiver"},
	"logs":      {"herringbone-logs"},
	"search":    {"herringbone-search"},
	"parser":    {"parser-cardset", "parser-enrichment", "parser-extractor"},
	"detection": {"detectionengine-detector", "detectionengine-matcher", "detectionengine-ruleset"},
	"incidents": {"incidents-incidentset", "incidents-correlator", "incidents-orchestrator"},
}

