package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/docker"
	hbmongo "github.com/herringbonedev/hbctl/internal/mongo"
	"github.com/herringbonedev/hbctl/internal/secrets"
)

const (
	composeMongo                = "compose.mongo.yml"
	composeReceiver             = "compose.logingestion.receiver.yml"
	composeLogs                 = "compose.herringbone.logs.yml"
	composeParserCardset        = "compose.parser.cardset.yml"
	composeParserEnrich         = "compose.parser.enrichment.yml"
	composeParserExtract        = "compose.parser.extractor.yml"
	composeDetector             = "compose.detectionengine.detector.yml"
	composeMatcher              = "compose.detectionengine.matcher.yml"
	composeRuleset              = "compose.detectionengine.ruleset.yml"
	composeOperationsCenter     = "compose.operations.center.yml"
	composeIncidentSet          = "compose.incidents.incidentset.yml"
	composeIncidentCorrelator   = "compose.incidents.correlator.yml"
	composeIncidentOrchestrator = "compose.incidents.orchestrator.yml"
	composeSearch               = "compose.herringbone.search.yml"
)

func composeFilesForElement(element string) []string {
	files := []string{"-f", composeMongo}

	switch element {
	case "logingestion-receiver":
		files = append(files, "-f", composeReceiver)
	case "herringbone-logs":
		files = append(files, "-f", composeLogs)
	case "parser-cardset":
		files = append(files, "-f", composeParserCardset)
	case "parser-enrichment":
		files = append(files, "-f", composeParserEnrich)
	case "parser-extractor":
		files = append(files, "-f", composeParserExtract)
	case "detectionengine-detector":
		files = append(files, "-f", composeDetector)
	case "detectionengine-matcher":
		files = append(files, "-f", composeMatcher)
	case "detectionengine-ruleset":
		files = append(files, "-f", composeRuleset)
	case "operations-center":
		files = append(files, "-f", composeOperationsCenter)
	case "incidents-incidentset":
		files = append(files, "-f", composeIncidentSet)
	case "incidents-correlator":
		files = append(files, "-f", composeIncidentCorrelator)
	case "incidents-orchestrator":
		files = append(files, "-f", composeIncidentOrchestrator)
	case "herringbone-search":
		files = append(files, "-f", composeSearch)
	}

	return files
}

func init() {
	Register("start", startCmd)
}

func startCmd(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	element := fs.String("element", "", "Element (service) to start")
	unit := fs.String("unit", "", "Unit (subsystem) to start")
	all := fs.Bool("all", false, "Start full Herringbone stack")
	recvType := fs.String("type", "", "Receiver type (UDP, TCP, HTTP)")
	fs.Parse(args)

	fmt.Println("[hbctl] Decrypting secrets...")
	sec, err := secrets.LoadMongo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to load MongoDB secret:", err)
		os.Exit(1)
	}

	env := map[string]string{
		"MONGO_ROOT_PASS": "",
		"MONGO_HOST":      sec.Host,
		"MONGO_PORT":      fmt.Sprintf("%d", sec.Port),
		"MONGO_USER":      sec.User,
		"MONGO_PASS":      sec.Password,
		"DB_NAME":         sec.Database,
		"AUTH_DB":         sec.AuthSource,
		"RECEIVER_TYPE":   "",
		"MATCHER_API":     "",
	}

	if *all {
		fmt.Println("[hbctl] Starting full Herringbone stack...")
		ensureDatabase(sec)

		for _, e := range allElements {
			if e.Name == "logingestion-receiver" {
				env["RECEIVER_TYPE"] = "UDP"
			}
			startElement(env, e.Name)
		}
		return
	}

	if *unit != "" {
		ensureDatabase(sec)

		elements := unitElements[*unit]
		if len(elements) == 0 {
			fmt.Fprintln(os.Stderr, "Unknown unit:", *unit)
			os.Exit(1)
		}

		for _, el := range elements {
			startElement(env, el)
		}
		return
	}

	if *element != "" {
		if *element == "logingestion-receiver" && *recvType == "" {
			fmt.Fprintln(os.Stderr, "Error: --type required for receiver")
			os.Exit(1)
		}

		ensureDatabase(sec)

		if *recvType != "" {
			env["RECEIVER_TYPE"] = strings.ToUpper(*recvType)
		}

		startElement(env, *element)
		return
	}

	fmt.Fprintln(os.Stderr, "Error: specify --element, --unit, or --all")
	os.Exit(1)
}

func startElement(env map[string]string, element string) {
	args := []string{"-p", composeProject}
	args = append(args, composeFilesForElement(element)...)
	args = append(args, "up", "-d", "--no-recreate", element)

	fmt.Println("[hbctl] Starting", element, "...")
	if err := docker.ComposeWithEnv(env, args...); err != nil {
		os.Exit(1)
	}
}

func ensureDatabase(sec *secrets.MongoSecret) {
	rootPass := randomPassword(24)

	rootURI := fmt.Sprintf(
		"mongodb://root:%s@localhost:%d/admin?authSource=admin",
		rootPass, sec.Port,
	)

	appURI := fmt.Sprintf(
		"mongodb://%s:%s@localhost:%d/%s?authSource=%s",
		sec.User, sec.Password, sec.Port, sec.Database, sec.AuthSource,
	)

	fmt.Println("[hbctl] Checking MongoDB app user...")
	if hbmongo.CanConnect(appURI) {
		fmt.Println("[hbctl] MongoDB already initialized.")
		return
	}

	env := map[string]string{
		"MONGO_ROOT_PASS": rootPass,
	}

	fmt.Println("[hbctl] Ensuring MongoDB is running...")
	if err := docker.ComposeWithEnv(env,
		"-p", composeProject,
		"-f", composeMongo,
		"up", "-d", "mongodb",
	); err != nil {
		os.Exit(1)
	}

	fmt.Println("[hbctl] Waiting for MongoDB root auth...")
	if err := hbmongo.WaitForConnect(rootURI, 60*time.Second); err != nil {
		fmt.Fprintln(os.Stderr, "Mongo root not ready:", err)
		os.Exit(1)
	}

	fmt.Println("[hbctl] Bootstrapping MongoDB user...")
	if err := hbmongo.EnsureUser(
		"localhost",
		sec.Port,
		rootPass,
		sec.User,
		sec.Password,
		sec.Database,
	); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to bootstrap MongoDB user:", err)
		os.Exit(1)
	}

	fmt.Println("[hbctl] MongoDB ready.")
}

func randomPassword(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
