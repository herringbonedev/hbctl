package local

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/docker"
	hbmongo "github.com/herringbonedev/hbctl/internal/mongo"
	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/herringbonedev/hbctl/internal/units"
)

type StartOptions struct {
	Project         string
	SecretsDir      string
	Element         string
	Unit            string
	All             bool
	RecvType        string
	TokenCreate     bool
	NoTokenCreate   bool
	BootstrapTokens bool
	Enterprise      bool
}

type requestOptions struct {
	OrgID string
}

func secretsDirForProject(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		base, err := filepath.Abs(strings.TrimSpace(override))
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "runtime"), nil
	}

	files := ComposeFilesForElement("herringbone-auth")
	if len(files) == 0 {
		return "", fmt.Errorf("no compose files found")
	}
	composeFile := files[1]
	base := filepath.Dir(composeFile)
	return filepath.Join(base, "secrets", "runtime"), nil
}

func Start(opts StartOptions) error {
	opts.Element = strings.TrimSpace(opts.Element)
	opts.Unit = strings.TrimSpace(opts.Unit)

	ui.Header("Herringbone start")
	ui.Step("Loading encrypted MongoDB credentials")

	sec, err := secrets.LoadMongo()
	if err != nil {
		return fmt.Errorf("failed to load MongoDB secret: %w", err)
	}

	env := blankLifecycleEnv(opts.Enterprise)
	env["MONGO_HOST"] = sec.Host
	env["MONGO_PORT"] = fmt.Sprintf("%d", sec.Port)
	env["MONGO_USER"] = sec.User
	env["MONGO_PASS"] = sec.Password
	env["DB_NAME"] = sec.Database
	env["AUTH_DB"] = sec.AuthSource

	desiredServiceTokens := serviceTokensForStart(opts)
	forceTokenRefresh := startForcesTokenRefresh(opts)
	runtimeSecretsRequired := startNeedsRuntimeSecrets(opts, desiredServiceTokens, forceTokenRefresh)

	if opts.NoTokenCreate {
		ui.Warn("--no-token-create is deprecated and ignored. Token creation is opt-in with --token-create")
	}

	var secretsDir string
	var jwtSecret *secrets.JWTSecret
	var svcKeys *secrets.ServiceKey

	if runtimeSecretsRequired {
		ui.Step("Preparing auth runtime secrets")

		jwtSecret, err = secrets.LoadJWTSecret()
		if err != nil {
			return fmt.Errorf("failed to load JWT secret: %w", err)
		}

		svcKeys, err = secrets.LoadServiceKey()
		if err != nil {
			return fmt.Errorf("failed to load service keys: %w", err)
		}

		secretsDir, err = secretsDirForProject(opts.SecretsDir)
		if err != nil {
			return err
		}

		if err := prepareAuthSecrets(secretsDir, jwtSecret.JWTSecret, svcKeys.PrivSvcKey, svcKeys.PubSvcKey); err != nil {
			return err
		}
	}

	if opts.All {
		ui.Section("Full stack")
		plan := []string{
			"Proxy is reused when an existing proxy container is present; otherwise hbctl creates one.",
			"MongoDB is reused when an existing MongoDB container or volume is present; otherwise hbctl creates one without removing data.",
			"Auth is reused when an existing auth container is present; otherwise hbctl creates one.",
			"Enterprise services are included only when --enterprise is provided; hbctl does not rename compose services.",
		}
		plan = append(plan, "MongoDB init-mongo.js is replayed idempotently after MongoDB is reachable, so existing volumes still get default org/scopes/index initialization.")
		if opts.Enterprise {
			plan = append(plan, "Enterprise platform/org seed data is ensured only when --enterprise is provided.")
		} else {
			plan = append(plan, "Core/free mode skips enterprise platform/org seed data but still runs common init-mongo.js.")
		}
		plan = append(plan,
			"Required service account token files are created before application services are started.",
			"Application services are created or started after the protected core is ready.",
			"Receivers are not started by --all; use hbctl receiver start so each receiver keeps its own compose project and port.",
		)
		ui.Plan("Start policy", plan)

		if err := ensureCoreService(opts.Project, env, "herringbone-proxy"); err != nil {
			return err
		}

		if err := ensureCoreDatabase(opts.Project, sec); err != nil {
			return err
		}

		if err := ensureCommonMongoSeedData(opts.Project); err != nil {
			return err
		}

		if opts.Enterprise {
			if err := ensureEnterpriseMongoSeedData(opts.Project, sec); err != nil {
				return err
			}
		} else {
			ui.Skip("Enterprise platform/org seed data: core/free mode")
		}

		authElement := AuthElementForMode(opts.Enterprise)
		if err := ensureCoreService(opts.Project, env, authElement); err != nil {
			return err
		}

		if err := waitHTTP("http://localhost:8080/health", 45*time.Second); err != nil {
			_ = waitHTTP("http://localhost:8080/docs", 5*time.Second)
		}

		if len(desiredServiceTokens) > 0 {
			if secretsDir == "" || jwtSecret == nil {
				return fmt.Errorf("service token bootstrap required but auth runtime secrets were not prepared")
			}
			if err := ensureServiceTokens(secretsDir, jwtSecret.JWTSecret, desiredServiceTokens, forceTokenRefresh); err != nil {
				return err
			}
		} else if forceTokenRefresh {
			ui.Info("No service tokens are required for this start target")
		}

		if err := cleanupMainProjectReceivers(opts.Project); err != nil {
			return err
		}

		if err := startFullStackApplications(opts.Project, env, opts.Enterprise); err != nil {
			return err
		}

		ui.Success("Full Herringbone stack start complete")
		return nil
	}

	if opts.Unit != "" {
		if strings.TrimSpace(opts.Unit) == "receiver" {
			return fmt.Errorf("receivers are managed separately; use hbctl receiver start --type <udp|tcp|http|remote>")
		}
		elements := units.UnitElements[opts.Unit]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		if strings.TrimSpace(opts.Unit) == "auth" {
			elements = []string{AuthElementForMode(opts.Enterprise)}
		}

		if len(desiredServiceTokens) > 0 {
			if secretsDir == "" || jwtSecret == nil {
				return fmt.Errorf("service token bootstrap required but auth runtime secrets were not prepared")
			}
			if err := waitHTTP("http://localhost:8080/health", 45*time.Second); err != nil {
				return err
			}
			if err := ensureServiceTokens(secretsDir, jwtSecret.JWTSecret, desiredServiceTokens, forceTokenRefresh); err != nil {
				return err
			}
		} else if forceTokenRefresh {
			ui.Info("No service tokens are required for this start target")
		}

		for _, el := range elements {
			element := CanonicalElementName(el)
			if element == "logingestion-receiver" {
				ui.Skip("logingestion-receiver: use hbctl receiver start so the receiver gets its own compose project and port")
				continue
			}
			if IsEnterpriseElement(element) && !opts.Enterprise {
				ui.Skip("%s: enterprise service requires --enterprise", element)
				continue
			}
			if err := startElement(opts.Project, env, element); err != nil {
				return err
			}
		}

		ui.Success("Unit %s start complete", opts.Unit)
		return nil
	}

	if opts.Element != "" {
		element := ElementForMode(opts.Element, opts.Enterprise)
		if element == "logingestion-receiver" {
			return fmt.Errorf("logingestion-receiver is managed separately; use hbctl receiver start --type <udp|tcp|http|remote> instead of hbctl start --element logingestion-receiver")
		}
		if IsEnterpriseElement(element) && !opts.Enterprise {
			return fmt.Errorf("%s is an enterprise service; pass --enterprise to start it", element)
		}

		if opts.RecvType != "" {
			env["RECEIVER_TYPE"] = strings.ToUpper(opts.RecvType)
		}

		if len(desiredServiceTokens) > 0 {
			if secretsDir == "" || jwtSecret == nil {
				return fmt.Errorf("service token bootstrap required but auth runtime secrets were not prepared")
			}
			if err := waitHTTP("http://localhost:8080/health", 45*time.Second); err != nil {
				return err
			}
			if err := ensureServiceTokens(secretsDir, jwtSecret.JWTSecret, desiredServiceTokens, forceTokenRefresh); err != nil {
				return err
			}
		} else if forceTokenRefresh {
			ui.Info("No service tokens are required for this start target")
		}

		if err := startElement(opts.Project, env, element); err != nil {
			return err
		}

		ui.Success("Element %s start complete", opts.Element)
		return nil
	}

	return fmt.Errorf("error: specify --element, --unit, or --all")
}

func startNeedsRuntimeSecrets(opts StartOptions, serviceTokens []ServiceIdentity, forceTokenRefresh bool) bool {
	if opts.All || forceTokenRefresh || len(serviceTokens) > 0 {
		return true
	}
	if CanonicalElementName(opts.Element) == "herringbone-auth" {
		return true
	}
	for _, element := range units.UnitElements[opts.Unit] {
		if CanonicalElementName(element) == "herringbone-auth" {
			return true
		}
	}
	return false
}

func startForcesTokenRefresh(opts StartOptions) bool {
	return opts.TokenCreate || opts.BootstrapTokens
}

func serviceTokensForStart(opts StartOptions) []ServiceIdentity {
	if CanonicalElementName(opts.Element) == "logingestion-receiver" || strings.TrimSpace(opts.Unit) == "receiver" {
		return nil
	}
	if opts.All {
		return BootstrapServicesForMode(opts.Enterprise)
	}

	elements := []string{}
	if opts.Element != "" {
		elements = append(elements, ElementForMode(opts.Element, opts.Enterprise))
	}
	if opts.Unit != "" {
		if strings.TrimSpace(opts.Unit) == "auth" {
			elements = append(elements, AuthElementForMode(opts.Enterprise))
		} else {
			elements = append(elements, units.UnitElements[opts.Unit]...)
		}
	}

	wanted := map[string]bool{}
	needsSharedToken := false
	for _, element := range elements {
		element = CanonicalElementName(element)
		if IsEnterpriseElement(element) && !opts.Enterprise {
			continue
		}
		wanted[element] = true
		if !isProtectedCoreService(element) {
			needsSharedToken = true
		}
	}
	if needsSharedToken {
		wanted["herringbone"] = true
	}

	out := []ServiceIdentity{}
	for _, svc := range BootstrapServicesForMode(opts.Enterprise) {
		if wanted[CanonicalElementName(svc.Name)] {
			out = append(out, svc)
		}
	}
	return out
}

func ensureServiceTokens(secretsDir string, jwtSecret string, services []ServiceIdentity, refresh bool) error {
	ui.Section("Service token bootstrap")

	if len(services) == 0 {
		ui.Info("No service token files are required for this target")
		return nil
	}

	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return err
	}

	toMint := make([]ServiceIdentity, 0, len(services))
	for _, svc := range services {
		missing := missingServiceTokenFiles(secretsDir, svc)
		if refresh {
			toMint = append(toMint, svc)
			continue
		}
		if len(missing) == 0 {
			ui.Success("%s token file already exists", svc.Name)
			continue
		}
		if token, ok := existingServiceToken(secretsDir, svc); ok {
			ui.Step("Repairing service token file aliases for %s", svc.Name)
			for _, filename := range missing {
				if err := writeRuntimeSecretFile(filepath.Join(secretsDir, filename), token); err != nil {
					return err
				}
			}
			continue
		}
		toMint = append(toMint, svc)
	}

	if len(toMint) == 0 {
		ui.Success("Service token files are ready")
		return nil
	}

	if err := ensureAdminToken(secretsDir, jwtSecret); err != nil {
		return err
	}

	adminToken, err := loadAdminToken(secretsDir)
	if err != nil {
		return err
	}

	authURL := "http://localhost:8080"
	client := &http.Client{Timeout: 10 * time.Second}

	for _, svc := range toMint {
		action := "Creating"
		if refresh {
			action = "Refreshing"
		}
		ui.Step("%s service token for %s", action, svc.Name)
		svcID := fmt.Sprintf("%s-%s", svc.ID, uuidString())

		createBody := map[string]any{
			"service_id":   svcID,
			"service_name": svc.Name,
			"scopes":       svc.Scopes,
		}

		if err := postJSON(
			client,
			authURL+"/herringbone/auth/services/internal/register",
			adminToken,
			createBody,
			nil,
			requestOptions{},
		); err != nil {
			return fmt.Errorf("create internal service %s failed: %w", svc.Name, err)
		}

		tokenResp := struct {
			AccessToken string `json:"access_token"`
		}{}

		if err := postJSON(
			client,
			authURL+"/herringbone/auth/services/internal/token",
			adminToken,
			map[string]any{
				"service": svc.Name,
				"scopes":  svc.Scopes,
			},
			&tokenResp,
			requestOptions{},
		); err != nil {
			return fmt.Errorf("internal token mint failed for %s: %w", svc.Name, err)
		}

		token := strings.TrimSpace(tokenResp.AccessToken)
		if token == "" {
			return fmt.Errorf("auth returned an empty token for %s", svc.Name)
		}

		for _, filename := range serviceTokenFilenames(svc) {
			if err := writeRuntimeSecretFile(filepath.Join(secretsDir, filename), token); err != nil {
				return err
			}
		}
	}

	ui.Success("Service token files written to %s", secretsDir)
	return nil
}

func serviceTokenFilenames(svc ServiceIdentity) []string {
	if len(svc.TokenFiles) > 0 {
		return uniqueStrings(svc.TokenFiles)
	}

	serviceName := strings.TrimSpace(svc.Name)
	if serviceName == "" {
		return nil
	}

	return []string{strings.ReplaceAll(serviceName, "-", "_") + "_service_token"}
}

func serviceTokenReadCandidates(svc ServiceIdentity) []string {
	candidates := append([]string{}, serviceTokenFilenames(svc)...)
	candidates = append(candidates, svc.LegacyTokenFiles...)

	serviceName := strings.TrimSpace(svc.Name)
	if serviceName != "" {
		candidates = append(candidates, strings.ReplaceAll(serviceName, "-", "_")+"_service_token")
	}

	return uniqueStrings(candidates)
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func missingServiceTokenFiles(secretsDir string, svc ServiceIdentity) []string {
	missing := []string{}
	for _, filename := range serviceTokenFilenames(svc) {
		b, err := os.ReadFile(filepath.Join(secretsDir, filename))
		if err != nil || strings.TrimSpace(string(b)) == "" {
			missing = append(missing, filename)
		}
	}
	return missing
}

func existingServiceToken(secretsDir string, svc ServiceIdentity) (string, bool) {
	for _, filename := range serviceTokenReadCandidates(svc) {
		b, err := os.ReadFile(filepath.Join(secretsDir, filename))
		if err == nil {
			token := strings.TrimSpace(string(b))
			if token != "" {
				return token, true
			}
		}
	}
	return "", false
}

func ensureAdminToken(secretsDir, jwtSecret string) error {
	path := filepath.Join(secretsDir, "admin_token")

	if b, err := os.ReadFile(path); err == nil {
		if strings.TrimSpace(string(b)) != "" {
			ui.Info("Admin bootstrap token already exists")
			return nil
		}
	}

	tok, err := mintAdminJWT(jwtSecret)
	if err != nil {
		return err
	}

	if err := writeRuntimeSecretFile(path, tok); err != nil {
		return fmt.Errorf("failed writing admin_token: %w", err)
	}

	ui.Success("Admin bootstrap token written")
	return nil
}

func mintAdminJWT(secret string) (string, error) {
	now := time.Now().UTC()

	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}

	payload := map[string]any{
		"sub":   "hbctl-bootstrap",
		"email": "hbctl@local",
		"typ":   "user",
		"scope": []string{"*"},
		"iat":   now.Unix(),
		"exp":   now.Add(24 * time.Hour).Unix(),
	}

	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	enc := base64.RawURLEncoding
	h := enc.EncodeToString(hb)
	p := enc.EncodeToString(pb)
	msg := h + "." + p

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(msg))
	sig := enc.EncodeToString(mac.Sum(nil))

	return msg + "." + sig, nil
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", url, nil)
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for %s", url)
}

func prepareAuthSecrets(secretsDir, jwtSecret, svcPriv, svcPub string) error {
	ui.Step("Writing runtime secret files")

	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return err
	}

	files := map[string]string{
		"jwt_secret":              jwtSecret,
		"service_jwt_private_key": svcPriv,
		"service_jwt_public_key":  svcPub,
	}

	for name, value := range files {
		path := filepath.Join(secretsDir, name)

		if err := writeRuntimeSecretFile(path, value); err != nil {
			return fmt.Errorf("failed writing %s: %w", name, err)
		}
	}

	bootstrapPath := filepath.Join(secretsDir, "bootstrap_token")

	if _, err := os.Stat(bootstrapPath); os.IsNotExist(err) {
		token, err := generateBootstrapToken(32)
		if err != nil {
			return fmt.Errorf("failed generating bootstrap token: %w", err)
		}

		if err := writeRuntimeSecretFile(bootstrapPath, token); err != nil {
			return fmt.Errorf("failed writing bootstrap_token: %w", err)
		}

		ui.Success("Generated bootstrap token")
	} else {
		ui.Info("Bootstrap token already exists")
	}

	return nil
}

func writeRuntimeSecretFile(path string, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	if existing, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(existing)) == strings.TrimSpace(value) {
		return os.Chmod(path, 0444)
	}

	if _, err := os.Stat(path); err == nil {
		_ = os.Chmod(path, 0600)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.TrimSpace(value)), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0444)
}

func requiredStartElement(element string) bool {
	switch CanonicalElementName(strings.TrimSpace(element)) {
	case "herringbone-auth", "herringbone-proxy", "mongodb":
		return true
	default:
		return false
	}
}

func composeFileArgs(composeArgs []string) ([]string, error) {
	files := []string{}
	for i := 0; i < len(composeArgs); i++ {
		if composeArgs[i] != "-f" {
			continue
		}

		if i+1 >= len(composeArgs) {
			return nil, fmt.Errorf("missing compose file after -f")
		}

		files = append(files, composeArgs[i+1])
		i++
	}
	return files, nil
}

func shouldStartElement(element string, composeArgs []string) (bool, string, error) {
	files, err := composeFileArgs(composeArgs)
	if err != nil {
		return false, "", err
	}

	if len(files) == 0 {
		if requiredStartElement(element) {
			return false, "", fmt.Errorf("required element %q has no compose file mapping", element)
		}
		return false, "no compose file mapping", nil
	}

	hasElementCompose := false
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		if file != ComposeMongo || CanonicalElementName(element) == "mongodb" {
			hasElementCompose = true
		}

		if _, err := os.Stat(file); err != nil {
			if requiredStartElement(element) {
				return false, "", fmt.Errorf("required compose file missing for %q: %s", element, file)
			}
			return false, fmt.Sprintf("compose file missing: %s", file), nil
		}
	}

	if !hasElementCompose {
		if requiredStartElement(element) {
			return false, "", fmt.Errorf("required element %q has no element compose file mapping", element)
		}
		return false, "no element compose file mapping", nil
	}

	return true, "", nil
}

func startElement(project string, env map[string]string, element string) error {
	composeArgs := ComposeFilesForElement(element)
	start, reason, err := shouldStartElement(element, composeArgs)
	if err != nil {
		return err
	}
	if !start {
		ui.Skip("%s: %s", element, reason)
		return nil
	}

	element = CanonicalElementName(element)
	service, err := resolveComposeServiceName(composeArgs, element)
	if err != nil {
		if requiredStartElement(element) {
			return err
		}
		ui.Skip("%s: %v", element, err)
		return nil
	}

	serviceEnv := envWithSingleReplicaGuards(env, element)
	if serviceHasFixedHostPort(element) {
		if err := pruneStoppedContainers(pruneContainerOptions{Project: project, IncludeProtected: false, Services: []string{element}, QuietIfEmpty: true}); err != nil {
			return err
		}

		existing, err := containersForService(project, element, false)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			ui.Success("Existing %s container already owns its fixed host port; reusing it", element)
			printContainerReuseTable(existing)
			return nil
		}
	}

	args := []string{"-p", project}
	args = append(args, composeArgs...)
	args = append(args, "up", "-d", "--no-recreate")
	if serviceHasFixedHostPort(element) {
		args = append(args, "--scale", service+"=1")
		ui.Info("%s publishes a fixed host port; forcing one replica to prevent port conflicts", element)
	}
	args = append(args, service)

	if service != element {
		ui.Step("Starting %s using compose service %s", element, service)
	} else {
		ui.Step("Starting %s", element)
	}
	return docker.ComposeWithEnv(serviceEnv, args...)
}

func envWithSingleReplicaGuards(env map[string]string, element string) map[string]string {
	out := map[string]string{}
	for k, v := range env {
		out[k] = v
	}
	out["COMPOSE_IGNORE_ORPHANS"] = "true"
	out["COMPOSE_PROGRESS"] = "plain"
	for _, key := range fixedHostPortReplicaEnvKeys(element) {
		out[key] = "1"
	}
	return out
}

func serviceHasFixedHostPort(element string) bool {
	return len(fixedHostPortReplicaEnvKeys(element)) > 0
}

func fixedHostPortReplicaEnvKeys(element string) []string {
	switch CanonicalElementName(element) {
	case "fingerprint-identifier":
		return []string{"FINGERPRINT_IDENTIFIER_REPLICAS"}
	case "logingestion-receiver":
		return []string{"LOGINGESTION_RECEIVER_REPLICAS"}
	default:
		return nil
	}
}

func ensureCoreService(project string, env map[string]string, element string) error {
	element = CanonicalElementName(element)
	ui.Section("Protected core: " + element)

	existing, err := containersForExactService(project, element, true)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		ui.Success("Existing %s container(s) found; reusing them", element)
		printContainerReuseTable(existing)

		toStart := stoppedContainers(existing)
		if len(toStart) == 0 {
			ui.Info("%s is already running", element)
			return nil
		}

		ui.Step("Starting existing %s container(s)", element)
		if err := startContainers(toStart); err != nil {
			return err
		}
		ui.Success("Started existing %s container(s)", element)
		return nil
	}

	ui.Step("No existing %s container found; creating with Docker Compose", element)
	return startElement(project, env, element)
}

func ensureCoreDatabase(project string, sec *secrets.MongoSecret) error {
	ui.Section("Protected core: mongodb")
	existing, err := containersForExactService(project, "mongodb", true)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		ui.Success("Existing MongoDB container found; reusing it")
		printContainerReuseTable(existing)
	} else {
		ui.Info("No existing MongoDB container found. hbctl will create one without removing existing volumes.")
	}

	return ensureDatabase(project, sec)
}

func startFullStackApplications(project string, env map[string]string, enterprise bool) error {
	ui.Section("Application services")
	for _, e := range units.AllElements {
		element := CanonicalElementName(e.Name)
		if isProtectedCoreService(element) {
			continue
		}
		if IsEnterpriseElement(element) && !enterprise {
			ui.Skip("%s: enterprise service requires --enterprise", element)
			continue
		}
		if element == "logingestion-receiver" {
			ui.Skip("logingestion-receiver: receivers are managed separately with hbctl receiver start/list/stop")
			continue
		}
		if err := startElement(project, env, element); err != nil {
			return err
		}
	}
	return nil
}

func ensureFullStackReceiver(project string, env map[string]string) error {
	existing, err := containersForExactService(project, "logingestion-receiver", true)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		ui.Success("Existing receiver container(s) found; reusing them")
		printContainerReuseTable(existing)

		toStart := stoppedContainers(existing)
		if len(toStart) == 0 {
			ui.Info("Receiver container(s) are already running")
			return nil
		}

		ui.Step("Starting existing receiver container(s)")
		if err := startContainers(toStart); err != nil {
			return err
		}
		ui.Success("Started existing receiver container(s)")
		return nil
	}

	if strings.TrimSpace(env["RECEIVER_TYPE"]) == "" {
		env["RECEIVER_TYPE"] = "UDP"
		ui.Info("No existing receiver found. Creating default UDP receiver.")
	}

	return startElement(project, env, "logingestion-receiver")
}

func cleanupMainProjectReceivers(project string) error {
	containers, err := containersForExactService(project, "logingestion-receiver", true)
	if err != nil {
		return err
	}

	mainProject := strings.TrimSpace(project)
	if mainProject == "" {
		mainProject = "herringbone"
	}

	toStop := []herringboneContainer{}
	toRemove := []herringboneContainer{}
	for _, container := range containers {
		if container.Project != mainProject {
			continue
		}
		if isRunningContainer(container) {
			toStop = append(toStop, container)
			continue
		}
		if isPrunableContainer(container) {
			toRemove = append(toRemove, container)
		}
	}

	if len(toStop) == 0 && len(toRemove) == 0 {
		return nil
	}

	ui.Section("Receiver cleanup")
	ui.Warn("Receivers are managed with hbctl receiver start/list/stop, not hbctl start --all")
	if len(toStop) > 0 {
		ui.Step("Stopping receiver container(s) accidentally created in the main project")
		if err := stopContainers(toStop); err != nil {
			return err
		}
		toRemove = append(toRemove, toStop...)
	}
	if len(toRemove) > 0 {
		ui.Step("Removing main-project receiver container(s); receiver data volumes are not touched")
		if err := removeContainers(toRemove); err != nil {
			return err
		}
	}
	ui.Success("Main-project receiver cleanup complete")
	return nil
}

func printContainerReuseTable(containers []herringboneContainer) {
	rows := make([][]string, 0, len(containers))
	for _, container := range containers {
		rows = append(rows, []string{container.Service, container.Project, container.State, container.Status, container.Name})
	}
	ui.Table([]string{"SERVICE", "PROJECT", "STATE", "STATUS", "NAME"}, rows)
}

func ensureCommonMongoSeedData(project string) error {
	ui.Section("MongoDB seed data")

	if _, err := os.Stat("init-mongo.js"); err != nil {
		ui.Skip("init-mongo.js not found in current directory")
		return nil
	}

	ui.Step("Replaying init-mongo.js inside the running MongoDB container")
	if err := runMongoInitScriptInContainer(project); err != nil {
		return fmt.Errorf("failed to replay init-mongo.js: %w", err)
	}

	ui.Success("MongoDB init-mongo.js replay complete")
	return nil
}

func ensureEnterpriseMongoSeedData(project string, sec *secrets.MongoSecret) error {
	ui.Section("Enterprise platform/org seed data")

	ui.Step("Ensuring enterprise platform org exists")
	if err := ensurePlatformOrgUsingMongoContainerEnv(project); err != nil {
		controlHost := mongoHostForHbctl(sec.Host)
		ui.Warn("container-env platform-org seed failed; trying hbctl app credentials: %v", err)
		if appErr := hbmongo.EnsurePlatformOrg(controlHost, sec.Port, sec.User, sec.Password, sec.Database, sec.AuthSource); appErr != nil {
			return fmt.Errorf("failed to ensure platform org seed data with container env and hbctl credentials: container=%v app=%w", err, appErr)
		}
	}

	ui.Success("Enterprise platform org seed data ready")
	return nil
}

func runMongoInitScriptInContainer(project string) error {
	script := `
set -eu
DB_NAME="${MONGO_DB:-herringbone}"
if [ -n "${MONGO_INITDB_ROOT_PASSWORD:-}" ]; then
  exec mongosh --quiet -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin "$DB_NAME" /docker-entrypoint-initdb.d/init-mongo.js
fi
if [ -n "${MONGO_USER:-}" ] && [ -n "${MONGO_PASS:-}" ]; then
  exec mongosh --quiet -u "$MONGO_USER" -p "$MONGO_PASS" --authenticationDatabase "$DB_NAME" "$DB_NAME" /docker-entrypoint-initdb.d/init-mongo.js
fi
echo "no Mongo credentials were available inside the mongodb container" >&2
exit 1
`
	return dockerExecMongoShell(project, script)
}

func ensurePlatformOrgUsingMongoContainerEnv(project string) error {
	seed := `
const now = new Date();
db.organizations.updateOne(
  { slug: "platform" },
  {
    $set: {
      name: "Platform",
      slug: "platform",
      status: "active",
      updated_at: now,
      platform: true
    },
    $setOnInsert: {
      created_at: now,
      created_by: "hbctl"
    }
  },
  { upsert: true }
);
const platform = db.organizations.findOne({ slug: "platform", status: "active" });
if (!platform) {
  throw new Error("platform org upsert did not produce an active platform org");
}
print("platform org ready: " + platform._id);
`

	return runMongoJavaScriptInContainer(project, seed)
}

func runMongoJavaScriptInContainer(project string, js string) error {
	var encoded bytes.Buffer
	for _, line := range strings.Split(js, "\n") {
		encoded.WriteString(line)
		encoded.WriteByte('\n')
	}

	shell := fmt.Sprintf(`
set -eu
DB_NAME="${MONGO_DB:-${DB_NAME:-herringbone}}"
cat > /tmp/hbctl-seed.js <<'HBCTL_JS'
%s
HBCTL_JS
if [ -n "${MONGO_USER:-}" ] && [ -n "${MONGO_PASS:-}" ]; then
  exec mongosh --quiet -u "$MONGO_USER" -p "$MONGO_PASS" --authenticationDatabase "${AUTH_DB:-$DB_NAME}" "$DB_NAME" /tmp/hbctl-seed.js
fi
if [ -n "${MONGO_INITDB_ROOT_PASSWORD:-}" ]; then
  exec mongosh --quiet -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin "$DB_NAME" /tmp/hbctl-seed.js
fi
echo "no Mongo credentials were available inside the mongodb container" >&2
exit 1
`, encoded.String())

	return dockerExecMongoShell(project, shell)
}

func dockerExecMongoShell(project string, script string) error {
	containerID, err := mongoContainerID(project)
	if err != nil {
		return err
	}

	cmd := exec.Command("docker", "exec", containerID, "sh", "-lc", script)
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("docker exec mongodb failed for container %s: %s", containerID, msg)
	}
	return nil
}

func mongoContainerID(project string) (string, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		project = "herringbone"
	}

	if _, err := os.Stat(ComposeMongo); err == nil {
		cmd := exec.Command("docker", "compose", "-p", project, "-f", ComposeMongo, "ps", "-q", "mongodb")
		cmd.Env = os.Environ()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			ids := strings.Fields(stdout.String())
			if len(ids) > 0 {
				return ids[0], nil
			}
		}
	}

	cmd := exec.Command("docker", "ps", "-q", "--filter", "name=^/mongodb$")
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err == nil {
		ids := strings.Fields(stdout.String())
		if len(ids) > 0 {
			return ids[0], nil
		}
	}

	return "", fmt.Errorf("could not find running MongoDB container for project %q", project)
}

func ensureDatabase(project string, sec *secrets.MongoSecret) error {
	rootPass, err := secrets.EnsureMongoRootPassword()
	if err != nil {
		return fmt.Errorf("failed to load or create protected MongoDB root secret: %w", err)
	}

	containerHost := strings.TrimSpace(sec.Host)
	if containerHost == "" {
		containerHost = "mongodb"
	}
	controlHost := mongoHostForHbctl(containerHost)

	rootURI := fmt.Sprintf(
		"mongodb://root:%s@%s:%d/admin",
		rootPass, controlHost, sec.Port,
	)

	appURI := fmt.Sprintf(
		"mongodb://%s:%s@%s:%d/%s?authSource=%s",
		sec.User, sec.Password, controlHost, sec.Port, sec.Database, sec.AuthSource,
	)

	ui.Section("MongoDB protection")
	ui.KeyValues([][2]string{
		{"container host", containerHost},
		{"hbctl check host", controlHost},
	})

	ui.Step("Checking MongoDB app user")
	if hbmongo.CanConnect(appURI) {
		ui.Success("MongoDB already initialized")
		return nil
	}

	env := map[string]string{
		"MONGO_ROOT_PASS": rootPass,
		"MONGO_HOST":      containerHost,
		"MONGO_PORT":      fmt.Sprintf("%d", sec.Port),
		"MONGO_USER":      sec.User,
		"MONGO_PASS":      sec.Password,
		"DB_NAME":         sec.Database,
		"AUTH_DB":         sec.AuthSource,
	}

	ui.Step("Starting or re-attaching MongoDB without recreate")
	if _, err := os.Stat(ComposeMongo); err != nil {
		return fmt.Errorf("required mongodb compose file missing: %s", ComposeMongo)
	}
	if err := docker.ComposeWithEnv(env,
		"-p", project,
		"-f", ComposeMongo,
		"up", "-d", "--no-recreate", "mongodb",
	); err != nil {
		return err
	}

	ui.Step("Waiting for MongoDB app auth")
	if hbmongo.WaitForConnect(appURI, 25*time.Second) == nil {
		ui.Success("MongoDB existing volume re-attached successfully")
		return nil
	}

	ui.Step("Waiting for MongoDB root auth")
	if err := hbmongo.WaitForConnect(rootURI, 60*time.Second); err != nil {
		return fmt.Errorf("mongo root not ready and app credentials did not authenticate. hbctl did not recreate or remove MongoDB data; check that the MongoDB port is published on localhost:%d and that the stored root password matches this volume: %w", sec.Port, err)
	}

	ui.Step("Bootstrapping MongoDB app user")
	if err := hbmongo.EnsureUser(
		controlHost,
		sec.Port,
		rootPass,
		sec.User,
		sec.Password,
		sec.Database,
	); err != nil {
		return fmt.Errorf("failed to bootstrap MongoDB user: %w", err)
	}

	ui.Success("MongoDB ready")
	return nil
}

func mongoHostForHbctl(host string) string {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "", "mongo", "mongodb":
		return "localhost"
	default:
		return strings.TrimSpace(host)
	}
}

func postJSON(client *http.Client, url, token string, body any, out any, opts requestOptions) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	if opts.OrgID != "" {
		req.Header.Set("X-Herringbone-Org", opts.OrgID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		d, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(d))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func getJSON(client *http.Client, url, token string, out any, opts requestOptions) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	if opts.OrgID != "" {
		req.Header.Set("X-Herringbone-Org", opts.OrgID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		d, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(d))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func maybeResolvePlatformOrgID(client *http.Client, authURL, token string) (string, error) {
	type enterpriseMeResponse struct {
		Contexts []struct {
			ContextID string `json:"context_id"`
			Slug      string `json:"slug"`
		} `json:"contexts"`
	}

	var resp enterpriseMeResponse

	err := getJSON(client, authURL+"/herringbone/auth/enterprise/me", token, &resp, requestOptions{})
	if err != nil {
		msg := err.Error()

		if strings.Contains(msg, "404") {
			return "", nil
		}
		if strings.Contains(msg, "X-Herringbone-Org header required") {
			return "", nil
		}
		if strings.Contains(msg, "default context not allowed") {
			return "", nil
		}
		if strings.Contains(msg, "invalid user id") {
			return "", nil
		}
		if strings.Contains(msg, "user identity required") {
			return "", nil
		}

		return "", err
	}

	for _, ctx := range resp.Contexts {
		if ctx.Slug == "platform" && ctx.ContextID != "" {
			return ctx.ContextID, nil
		}
	}

	return "", nil
}

func loadAdminToken(secretsDir string) (string, error) {
	path := filepath.Join(secretsDir, "admin_token")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("admin token missing: %w", err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("admin token empty: %s", path)
	}
	return tok, nil
}

func uuidString() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func generateBootstrapToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func resumeDedicatedReceivers(project string) error {
	receivers, err := stoppedDedicatedReceivers(project)
	if err != nil {
		return err
	}
	if len(receivers) == 0 {
		ui.Info("No stopped dedicated receivers to resume")
		return nil
	}

	ui.Section("Dedicated receivers")
	tableRows := make([][]string, 0, len(receivers))
	for _, receiver := range receivers {
		tableRows = append(tableRows, []string{receiver.Project, receiver.State, receiver.Name})
	}
	ui.Table([]string{"PROJECT", "STATE", "NAME"}, tableRows)
	ui.Step("Resuming dedicated receiver container(s)")
	if err := startContainers(receivers); err != nil {
		return err
	}
	ui.Success("Resumed %d dedicated receiver container(s)", len(receivers))
	return nil
}
