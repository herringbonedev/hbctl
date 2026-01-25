package units

// ElementInfo describes a runnable Herringbone element (service).
type ElementInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Unit        string `json:"unit"`
}

var AllElements = []ElementInfo{
	{Name: "logingestion-receiver", Description: "UDP/TCP/HTTP log ingestion receiver", Unit: "receiver"},

	{Name: "herringbone-logs", Description: "Logs API", Unit: "logs"},
	{Name: "herringbone-search", Description: "Read-only search API over MongoDB collections", Unit: "search"},
	{Name: "herringbone-auth", Description: "Standard authentication API", Unit: "auth"},

	{Name: "parser-cardset", Description: "Cardset metadata parser service", Unit: "parser"},
	{Name: "parser-enrichment", Description: "Log enrichment parser service", Unit: "parser"},
	{Name: "parser-extractor", Description: "Regex/JSONPath extractor service", Unit: "parser"},

	{Name: "detectionengine-detector", Description: "Detection engine detector service", Unit: "detection"},
	{Name: "detectionengine-matcher", Description: "Detection engine matcher service", Unit: "detection"},
	{Name: "detectionengine-ruleset", Description: "Detection engine ruleset service", Unit: "detection"},

	{Name: "incidents-incidentset", Description: "Incident aggregation and tracking service", Unit: "incidents"},
	{Name: "incidents-correlator", Description: "Incident correlation engine", Unit: "incidents"},
	{Name: "incidents-orchestrator", Description: "Incident orchestration service", Unit: "incidents"},

	{Name: "operations-center", Description: "Operations Center UI", Unit: "operations-center"},
}

var UnitElements = map[string][]string{
	"database": 		 {"mongodb"},
	"receiver":  		 {"logingestion-receiver"},
	"logs":      		 {"herringbone-logs"},
	"search":    		 {"herringbone-search"},
	"auth":      		 {"herringbone-auth"},
	"operations-center": {"operations-center"},
	"parser":    		 {"parser-cardset", "parser-enrichment", "parser-extractor"},
	"detection": 		 {"detectionengine-detector", "detectionengine-matcher", "detectionengine-ruleset"},
	"incidents": 		 {"incidents-incidentset", "incidents-correlator", "incidents-orchestrator"},
}



var ServiceUnits = map[string][]string{
	"logs":      		 {"herringbone-logs", "herringbone-search", "mongodb"},
	"search":    		 {"herringbone-search", "mongodb"},
	"auth":    		 {"herringbone-auth", "mongodb"},
	"receiver":  		 {"logingestion-receiver", "mongodb"},
	"parser":    		 {"parser-cardset", "parser-enrichment", "parser-extractor"},
	"detection": 		 {"detectionengine-detector", "detectionengine-matcher", "detectionengine-ruleset"},
	"incidents": 		 {"incidents-incidentset", "incidents-correlator", "incidents-orchestrator", "mongodb"},
	"operations-center": {"operations-center"},
	"database":  		 {"mongodb"},
}

