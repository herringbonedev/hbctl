package local

import "github.com/google/uuid"

type ServiceIdentity struct {
	Name   string
	ID     string
	Scopes []string
}

var herringboneNamespace = uuid.MustParse("8b6c3c94-1c2e-4f8a-9b3e-9f2d8b0c9a11")

func serviceUUID(name string) string {
	return uuid.NewSHA1(herringboneNamespace, []byte(name)).String()
}

var BootstrapServices = []ServiceIdentity{
    {
        Name:   "parser-enrichment",
        ID:     serviceUUID("parser-enrichment"),
        Scopes: []string{
            "extractor:call",
            "parser:cards:read",
        },
    },
    {
        Name:   "parser-extractor",
        ID:     serviceUUID("parser-extractor"),
        Scopes: []string{
            "parser:extract",
        },
    },
    {
        Name:   "parser-cardset",
        ID:     serviceUUID("parser-cardset"),
        Scopes: []string{
            "parser:cards:read",
            "parser:cards:write",
        },
    },
	{
        Name:   "incidents-incidentset",
        ID:     serviceUUID("incidents-incidentset"),
        Scopes: []string{
			"incidents:write",
        },
    },
    {
        Name:   "incidents-orchestrator",
        ID:     serviceUUID("incidents-orchestrator"),
        Scopes: []string{
            "incidents:write",
			"incidents:correlate",
        },
    },
    {
        Name:   "incidents-correlator",
        ID:     serviceUUID("incidents-correlator"),
        Scopes: []string{
			"events:read",
        },
    },
	{
        Name:   "detectionengine-detector",
        ID:     serviceUUID("detectionengine-detector"),
        Scopes: []string{
			"incidents:orchestrate",
			"detectionengine:run",
        },
    },
	{
        Name:   "detectionengine-matcher",
        ID:     serviceUUID("detectionengine-matcher"),
        Scopes: []string{
			"detectionengine:run",
        },
    },
		{
        Name:   "detectionengine-ruleset",
        ID:     serviceUUID("detectionengine-ruleset"),
        Scopes: []string{
			"rules:read",
        },
    },
}
