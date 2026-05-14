package local

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/secrets"
)

const defaultReceiverPortStart = 9000
const defaultReceiverPortEnd = 9999

const receiverServiceName = "logingestion-receiver"

type ReceiverStartOptions struct {
	Project          string
	ReceiverType     string
	Mode             string
	HostPort         int
	ContainerPort    int
	ForwardRoute     string
	ComposeFile      string
	IngestionKey     string
	IngestionKeyFile string
}

type ReceiverStopOptions struct {
	Project      string
	ReceiverType string
	HostPort     int
	ComposeFile  string
}

type ReceiverRestartOptions struct {
	Project      string
	ReceiverType string
	HostPort     int
	ComposeFile  string
}

type ReceiverListOptions struct {
	Project string
	JSON    bool
}

type ReceiverLogsOptions struct {
	Project      string
	ReceiverType string
	HostPort     int
	ComposeFile  string
	Follow       bool
	Tail         int
}

type ReceiverInstance struct {
	Name          string      `json:"name"`
	Project       string      `json:"project"`
	Service       string      `json:"service"`
	ReceiverType  string      `json:"receiver_type"`
	Mode          string      `json:"mode,omitempty"`
	HostPort      int         `json:"host_port"`
	ContainerPort int         `json:"container_port"`
	State         string      `json:"state"`
	Status        string      `json:"status"`
	ForwardRoute  string      `json:"forward_route,omitempty"`
	Ports         []Publisher `json:"ports,omitempty"`
}

type dockerPSReceiverRow struct {
	Names  string `json:"Names"`
	State  string `json:"State"`
	Status string `json:"Status"`
	Ports  string `json:"Ports"`
	Labels string `json:"Labels"`
}

type dockerInspectConfig struct {
	Config struct {
		Env []string `json:"Env"`
	} `json:"Config"`
}

func StartReceiver(opts ReceiverStartOptions) error {
	composeFile, err := resolveReceiverComposeFile(opts.ComposeFile)
	if err != nil {
		return err
	}

	hostPort := opts.HostPort
	if hostPort <= 0 {
		hostPort, err = findAvailablePort(opts.ReceiverType, defaultReceiverPortStart, defaultReceiverPortEnd)
		if err != nil {
			return err
		}
	}

	if opts.ContainerPort <= 0 {
		opts.ContainerPort = 7004
	}

	if _, err := lookupReceiverInstance(opts.Project, opts.ReceiverType, hostPort); err == nil {
		return fmt.Errorf("receiver %s on port %d already exists", opts.ReceiverType, hostPort)
	}

	env, err := receiverEnvironment(opts, hostPort)
	if err != nil {
		return err
	}

	project := receiverProjectName(opts.Project, opts.ReceiverType, hostPort)
	fmt.Printf("[hbctl] Starting %s receiver on host port %d using compose project %s\n", strings.ToUpper(strings.TrimSpace(opts.ReceiverType)), hostPort, project)
	return docker.ComposeWithEnv(env,
		"-p", project,
		"-f", composeFile,
		"up", "-d",
	)
}

func StopReceiver(opts ReceiverStopOptions) error {
	composeFile, err := resolveReceiverComposeFile(opts.ComposeFile)
	if err != nil {
		return err
	}

	instance, err := lookupReceiverInstance(opts.Project, opts.ReceiverType, opts.HostPort)
	if err != nil {
		return err
	}

	env, err := receiverEnvFromContainer(instance.Name)
	if err != nil {
		return err
	}

	containerPort := instance.ContainerPort
	if containerPort <= 0 {
		containerPort = blankInt(env["CONTAINER_PORT"], 7004)
	}

	env["HOST_PORT"] = strconv.Itoa(instance.HostPort)
	env["CONTAINER_PORT"] = strconv.Itoa(containerPort)

	fmt.Printf("[hbctl] Stopping %s receiver on host port %d\n", strings.ToUpper(instance.ReceiverType), instance.HostPort)
	return docker.ComposeWithEnv(env,
		"-p", instance.Project,
		"-f", composeFile,
		"down",
	)
}

func RestartReceiver(opts ReceiverRestartOptions) error {
	composeFile, err := resolveReceiverComposeFile(opts.ComposeFile)
	if err != nil {
		return err
	}

	instance, err := lookupReceiverInstance(opts.Project, opts.ReceiverType, opts.HostPort)
	if err != nil {
		return err
	}

	env, err := receiverEnvFromContainer(instance.Name)
	if err != nil {
		return err
	}

	fmt.Printf("[hbctl] Restarting %s receiver on host port %d\n", strings.ToUpper(instance.ReceiverType), instance.HostPort)
	return docker.ComposeWithEnv(env,
		"-p", instance.Project,
		"-f", composeFile,
		"restart", receiverServiceName,
	)
}

func LogsReceiver(opts ReceiverLogsOptions) error {
	composeFile, err := resolveReceiverComposeFile(opts.ComposeFile)
	if err != nil {
		return err
	}

	instance, err := lookupReceiverInstance(opts.Project, opts.ReceiverType, opts.HostPort)
	if err != nil {
		return err
	}

	env, err := receiverEnvFromContainer(instance.Name)
	if err != nil {
		return err
	}

	containerPort := instance.ContainerPort
	if containerPort <= 0 {
		containerPort = blankInt(env["CONTAINER_PORT"], 7004)
	}

	env["HOST_PORT"] = strconv.Itoa(instance.HostPort)
	env["CONTAINER_PORT"] = strconv.Itoa(containerPort)

	args := []string{
		"-p", instance.Project,
		"-f", composeFile,
		"logs",
	}
	if opts.Follow {
		args = append(args, "-f")
	}
	if opts.Tail > 0 {
		args = append(args, "--tail", strconv.Itoa(opts.Tail))
	}
	args = append(args, receiverServiceName)

	return docker.ComposeWithEnv(env, args...)
}

func ListReceivers(opts ReceiverListOptions) error {
	rows, err := inspectReceiverContainers(opts.Project)
	if err != nil {
		return err
	}

	instances := make([]ReceiverInstance, 0, len(rows))
	for _, row := range rows {
		instance, ok := parseReceiverInstance(row, opts.Project)
		if !ok {
			continue
		}
		if enriched, err := receiverEnvFromContainer(instance.Name); err == nil {
			instance.Mode = receiverModeFromEnv(enriched)
			instance.ForwardRoute = enriched["FORWARD_ROUTE"]
		}
		instances = append(instances, instance)
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].ReceiverType == instances[j].ReceiverType {
			return instances[i].HostPort < instances[j].HostPort
		}
		return instances[i].ReceiverType < instances[j].ReceiverType
	})

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(instances)
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TYPE\tMODE\tHOST PORT\tCONTAINER PORT\tSTATE\tSTATUS\tFORWARD ROUTE\tNAME")
	for _, instance := range instances {
		fmt.Fprintf(writer, "%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
			instance.ReceiverType,
			blankDefault(instance.Mode, "local"),
			instance.HostPort,
			instance.ContainerPort,
			instance.State,
			instance.Status,
			instance.ForwardRoute,
			instance.Name,
		)
	}
	return writer.Flush()
}

func resolveReceiverComposeFile(override string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(override) != "" {
		candidates = append(candidates, strings.TrimSpace(override))
	}
	candidates = append(candidates, ComposeReceiver, "compose.receiver.yml")

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return candidates[len(candidates)-1], nil
}

func receiverEnvironment(opts ReceiverStartOptions, resolvedHostPort int) (map[string]string, error) {
	keyValue, err := resolveIngestionKeyValue(opts.IngestionKey, opts.IngestionKeyFile)
	if err != nil {
		return nil, err
	}

	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		if strings.TrimSpace(opts.ForwardRoute) != "" {
			mode = "forward"
		} else {
			mode = "local"
		}
	}

	return receiverLifecycleEnvironment(
		opts.ReceiverType,
		mode,
		resolvedHostPort,
		opts.ContainerPort,
		opts.ForwardRoute,
		keyValue,
	)
}

func resolveIngestionKeyValue(inlineValue, filePath string) (string, error) {
	inlineValue = strings.TrimSpace(inlineValue)
	filePath = strings.TrimSpace(filePath)
	if inlineValue != "" && filePath != "" {
		return "", fmt.Errorf("use either --ingestion-key or --ingestion-key-file, not both")
	}
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read ingestion key file: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return inlineValue, nil
}

func receiverLifecycleEnvironment(receiverType, mode string, hostPort, containerPort int, forwardRoute, ingestionKey string) (map[string]string, error) {
	receiverType = strings.ToUpper(strings.TrimSpace(receiverType))
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "local"
	}
	if strings.TrimSpace(forwardRoute) != "" {
		mode = "forward"
	}
	forwardRoute = strings.TrimSpace(forwardRoute)
	ingestionKey = strings.TrimSpace(ingestionKey)

	if receiverType == "" {
		return nil, fmt.Errorf("receiver type is required")
	}
	if hostPort <= 0 {
		return nil, fmt.Errorf("host port is required")
	}
	if containerPort <= 0 {
		containerPort = 7004
	}

	env := map[string]string{
		"HOST_PORT":      strconv.Itoa(hostPort),
		"CONTAINER_PORT": strconv.Itoa(containerPort),
		"RECEIVER_TYPE":  receiverType,
		"RECEIVER_MODE":  mode,
		"FORWARD_ROUTE":  forwardRoute,
		"INGESTION_KEY":  "",
		"HB_ENTERPRISE":  "true",
		"MONGO_HOST":     "",
		"MONGO_PORT":     "",
		"MONGO_USER":     "",
		"MONGO_PASS":     "",
		"DB_NAME":        "",
		"AUTH_DB":        "",
	}

	switch mode {
	case "forward":
		if forwardRoute == "" {
			return nil, fmt.Errorf("--forward-route required for forward mode")
		}
		if ingestionKey == "" {
			return nil, fmt.Errorf("forward mode requires an existing ingestion key via --ingestion-key or --ingestion-key-file")
		}
		env["INGESTION_KEY"] = ingestionKey
	case "local":
		mongoSecret, err := secrets.LoadMongo()
		if err != nil {
			return nil, fmt.Errorf("failed to load MongoDB secret: %w", err)
		}
		env["MONGO_HOST"] = mongoSecret.Host
		env["MONGO_PORT"] = strconv.Itoa(mongoSecret.Port)
		env["MONGO_USER"] = mongoSecret.User
		env["MONGO_PASS"] = mongoSecret.Password
		env["DB_NAME"] = mongoSecret.Database
		env["AUTH_DB"] = mongoSecret.AuthSource
	default:
		return nil, fmt.Errorf("unsupported receiver mode %q", mode)
	}

	return env, nil
}

func receiverProjectName(project, receiverType string, hostPort int) string {
	base := strings.ToLower(strings.TrimSpace(project))
	if base == "" {
		base = "herringbone"
	}
	receiverType = strings.ToLower(strings.TrimSpace(receiverType))
	return fmt.Sprintf("%s-receiver-%s-%d", base, receiverType, hostPort)
}

func findAvailablePort(receiverType string, start, end int) (int, error) {
	for port := start; port <= end; port++ {
		if portAvailable(receiverType, port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available %s receiver port found in range %d-%d", receiverType, start, end)
}

func portAvailable(receiverType string, port int) bool {
	switch strings.ToLower(strings.TrimSpace(receiverType)) {
	case "udp":
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
		if err != nil {
			return false
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	default:
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return false
		}
		_ = listener.Close()
		return true
	}
}

func inspectReceiverContainers(project string) ([]dockerPSReceiverRow, error) {
	filterProject := strings.ToLower(strings.TrimSpace(project))
	if filterProject == "" {
		filterProject = "herringbone"
	}

	cmd := exec.Command(
		"docker", "ps",
		"--filter", fmt.Sprintf("name=%s-receiver-", filterProject),
		"--format", "json",
	)
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(exitError.Stderr)
		}
		return nil, err
	}

	var rows []dockerPSReceiverRow
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var row dockerPSReceiverRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func lookupReceiverInstance(project, receiverType string, hostPort int) (ReceiverInstance, error) {
	receiverType = strings.ToLower(strings.TrimSpace(receiverType))
	rows, err := inspectReceiverContainers(project)
	if err != nil {
		return ReceiverInstance{}, err
	}
	for _, row := range rows {
		instance, ok := parseReceiverInstance(row, project)
		if !ok {
			continue
		}
		if instance.ReceiverType == receiverType && instance.HostPort == hostPort {
			if env, err := receiverEnvFromContainer(instance.Name); err == nil {
				instance.Mode = receiverModeFromEnv(env)
				instance.ForwardRoute = env["FORWARD_ROUTE"]
			}
			return instance, nil
		}
	}
	return ReceiverInstance{}, fmt.Errorf("receiver %s on port %d not found", receiverType, hostPort)
}

func receiverEnvFromInstance(instance ReceiverInstance) (map[string]string, error) {
	return receiverEnvFromContainer(instance.Name)
}

func receiverEnvFromContainer(containerName string) (map[string]string, error) {
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{json .}}")
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(exitError.Stderr)
		}
		return nil, err
	}

	var raw dockerInspectConfig
	if err := json.Unmarshal(bytes.TrimSpace(output), &raw); err != nil {
		return nil, err
	}

	env := map[string]string{}
	for _, entry := range raw.Config.Env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[parts[0]] = parts[1]
	}
	return env, nil
}

func parseReceiverInstance(row dockerPSReceiverRow, project string) (ReceiverInstance, bool) {
	name := row.Names
	receiverType, hostPort, ok := receiverIdentityFromName(name, project)
	if !ok {
		return ReceiverInstance{}, false
	}

	ports := parsePorts(row.Ports)
	containerPort := 0
	if len(ports) > 0 {
		containerPort = ports[0].TargetPort
		for _, published := range ports {
			if published.PublishedPort == hostPort {
				containerPort = published.TargetPort
				break
			}
		}
	}

	return ReceiverInstance{
		Name:          name,
		Project:       receiverProjectName(project, receiverType, hostPort),
		Service:       receiverServiceName,
		ReceiverType:  receiverType,
		HostPort:      hostPort,
		ContainerPort: containerPort,
		State:         row.State,
		Status:        row.Status,
		Ports:         ports,
	}, true
}

func receiverIdentityFromName(containerName, project string) (string, int, bool) {
	base := strings.ToLower(strings.TrimSpace(project))
	if base == "" {
		base = "herringbone"
	}
	prefix := fmt.Sprintf("%s-receiver-", base)
	if !strings.HasPrefix(containerName, prefix) {
		return "", 0, false
	}

	trimmed := strings.TrimPrefix(containerName, prefix)
	parts := strings.Split(trimmed, "-")
	if len(parts) < 2 {
		return "", 0, false
	}

	receiverType := parts[0]
	portValue, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, false
	}
	return receiverType, portValue, true
}

func receiverModeFromEnv(env map[string]string) string {
	if strings.TrimSpace(env["FORWARD_ROUTE"]) != "" {
		return "forward"
	}
	if mode := strings.ToLower(strings.TrimSpace(env["RECEIVER_MODE"])); mode != "" {
		return mode
	}
	return "local"
}

func receiverModeWithFallback(env map[string]string, fallback string) string {
	if mode := receiverModeFromEnv(env); strings.TrimSpace(mode) != "" {
		return mode
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "local"
}

func blankDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func blankInt(value string, fallbacks ...int) int {
	value = strings.TrimSpace(value)
	if value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	for _, fallback := range fallbacks {
		if fallback > 0 {
			return fallback
		}
	}
	return 0
}