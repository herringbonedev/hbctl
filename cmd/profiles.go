package cmd

type profileInfo struct {
	Name        string
	Description string
}

var allProfiles = []profileInfo{
	{
		Name:        "logingestion-receiver",
		Description: "UDP/TCP/HTTP log ingestion receiver",
	},
	{
		Name:        "herringbone-logs",
		Description: "Logs API / UI service",
	},
	{
		Name:        "parser-cardset",
		Description: "Cardset metadata parser service",
	},
	{
		Name:        "parser-enrichment",
		Description: "Log enrichment parser service",
	},
	{
		Name:        "parser-extractor",
		Description: "Regex/JSONPath extractor service",
	},
	{
		Name:        "detectionengine-detector",
		Description: "Detection engine detector service",
	},
	{
		Name:        "detectionengine-matcher",
		Description: "Detection engine matcher service",
	},
	{
		Name:        "detectionengine-ruleset",
		Description: "Detection engine ruleset service",
	},
}
