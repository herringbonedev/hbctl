package local

import "github.com/google/uuid"

type ServiceIdentity struct {
	// Name is the auth-side service identity name. It may intentionally differ
	// from the Docker Compose service name for enterprise images that still set
	// HERRINGBONE_SERVICE to an enterprise identity.
	Name             string
	ID               string
	Scopes           []string
	EnterpriseOnly   bool
	TokenFiles       []string
	LegacyTokenFiles []string
}

var herringboneNamespace = uuid.MustParse("8b6c3c94-1c2e-4f8a-9b3e-9f2d8b0c9a11")

func serviceUUID(name string) string {
	return uuid.NewSHA1(herringboneNamespace, []byte(name)).String()
}

// BootstrapServices follows the original working v0.5 bootstrap model:
// register a service identity through auth, mint a token through auth, then
// write the token to the runtime secret file that the compose service mounts.
// Docker service names are not inferred from these identities.
var BootstrapServices = []ServiceIdentity{
	{
		Name: "herringbone",
		ID:   serviceUUID("herringbone"),
		TokenFiles: []string{
			"herringbone_service_token",
		},
		Scopes: []string{
			"logs:read",
			"logs:ingest",
			"events:read",
			"parser:cards:read",
			"parser:cards:write",
			"incidents:write",
			"incidents:correlate",
			"incidents:orchestrate",
			"detectionengine:run",
			"rules:read",
		},
	},
	{
		Name:           "fingerprint-scoreset-e",
		ID:             serviceUUID("fingerprint-scoreset-e"),
		EnterpriseOnly: true,
		TokenFiles: []string{
			"fingerprint_scoreset_service_token",
		},
		LegacyTokenFiles: []string{
			"fingerprint_scoreset_e_service_token",
		},
		Scopes: []string{
			"fingerprint:scorecards:read",
			"fingerprint:scorecards:write",
		},
	},
	{
		Name:           "fingerprint-identifier-e",
		ID:             serviceUUID("fingerprint-identifier-e"),
		EnterpriseOnly: true,
		TokenFiles: []string{
			"fingerprint_identifier_service_token",
		},
		LegacyTokenFiles: []string{
			"fingerprint_identifier_e_service_token",
		},
		Scopes: []string{
			"logs:read",
			"parser:cards:read",
			"fingerprint:scorecards:read",
		},
	},
	{
		Name:           "parser-enrichment-e",
		ID:             serviceUUID("parser-enrichment-e"),
		EnterpriseOnly: true,
		TokenFiles: []string{
			"parser_enrichment_service_token",
		},
		LegacyTokenFiles: []string{
			"parser_enrichment_e_service_token",
		},
		Scopes: []string{
			"extractor:call",
			"parser:cards:read",
		},
	},
	{
		Name: "fingerprint-identifier",
		ID:   serviceUUID("fingerprint-identifier"),
		Scopes: []string{
			"logs:read",
			"parser:cards:read",
		},
	},
	{
		Name: "parser-enrichment",
		ID:   serviceUUID("parser-enrichment"),
		Scopes: []string{
			"extractor:call",
			"parser:cards:read",
		},
	},
	{
		Name: "parser-extractor",
		ID:   serviceUUID("parser-extractor"),
		Scopes: []string{
			"parser:extract",
		},
	},
	{
		Name: "parser-cardset",
		ID:   serviceUUID("parser-cardset"),
		Scopes: []string{
			"parser:cards:read",
			"parser:cards:write",
		},
	},
	{
		Name: "incidents-incidentset",
		ID:   serviceUUID("incidents-incidentset"),
		Scopes: []string{
			"incidents:write",
		},
	},
	{
		Name: "incidents-orchestrator",
		ID:   serviceUUID("incidents-orchestrator"),
		Scopes: []string{
			"incidents:write",
			"incidents:correlate",
		},
	},
	{
		Name: "incidents-correlator",
		ID:   serviceUUID("incidents-correlator"),
		Scopes: []string{
			"events:read",
		},
	},
	{
		Name: "detectionengine-detector",
		ID:   serviceUUID("detectionengine-detector"),
		Scopes: []string{
			"incidents:orchestrate",
			"detectionengine:run",
		},
	},
	{
		Name: "detectionengine-matcher",
		ID:   serviceUUID("detectionengine-matcher"),
		Scopes: []string{
			"detectionengine:run",
		},
	},
	{
		Name: "detectionengine-ruleset",
		ID:   serviceUUID("detectionengine-ruleset"),
		Scopes: []string{
			"rules:read",
		},
	},
}

func BootstrapServicesForMode(enterprise bool) []ServiceIdentity {
	out := make([]ServiceIdentity, 0, len(BootstrapServices))
	for _, svc := range BootstrapServices {
		if svc.EnterpriseOnly && !enterprise {
			continue
		}
		out = append(out, svc)
	}
	return out
}
