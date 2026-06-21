package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

func modelCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "model", Short: "Manage the fingerprint tuner Ollama model"}
	cmd.AddCommand(modelListCommand())
	cmd.AddCommand(modelPullCommand())
	cmd.AddCommand(modelUseCommand())
	cmd.AddCommand(modelShowCommand())
	return cmd
}

func modelListCommand() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List pulled Ollama models", RunE: func(cmd *cobra.Command, args []string) error {
		ui.FHeader(cmd.OutOrStdout(), "Herringbone models")
		out, err := dockerOllama("list")
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}}
}

func modelPullCommand() *cobra.Command {
	return &cobra.Command{Use: "pull <model>", Short: "Pull an Ollama model", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		model := strings.TrimSpace(args[0])
		if model == "" {
			return fmt.Errorf("model is required")
		}
		ui.FHeader(cmd.OutOrStdout(), "Herringbone model pull")
		ui.FStep(cmd.OutOrStdout(), "Pulling %s", model)
		out, err := dockerOllama("pull", model)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		ui.FSuccess(cmd.OutOrStdout(), "Model ready: %s", model)
		return nil
	}}
}

func modelUseCommand() *cobra.Command {
	return &cobra.Command{Use: "use <model>", Short: "Pull if needed, save, and restart fingerprint-tuner", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		model := strings.TrimSpace(args[0])
		if model == "" {
			return fmt.Errorf("model is required")
		}
		ui.FHeader(cmd.OutOrStdout(), "Herringbone model use")
		list, _ := dockerOllama("list")
		if !strings.Contains(list, model) {
			ui.FStep(cmd.OutOrStdout(), "Pulling %s", model)
			if out, err := dockerOllama("pull", model); err != nil {
				return err
			} else {
				fmt.Fprint(cmd.OutOrStdout(), out)
			}
		} else {
			ui.FSuccess(cmd.OutOrStdout(), "Model already available: %s", model)
		}
		if err := saveModelEnv(model); err != nil {
			return err
		}
		ui.FSuccess(cmd.OutOrStdout(), "Saved fingerprint tuner model: %s", model)

		// Do not bypass hbctl lifecycle here. The direct docker compose path used
		// to recreate fingerprint-tuner without the Mongo secret environment and
		// without the Mongo service-discovery repair. That left the tuner unable to
		// resolve mongodb even though start/restart/upgrade had been fixed. Route
		// through local.Start so model changes use the same protected core and
		// network logic as every other hbctl lifecycle command.
		os.Setenv("FINGERPRINT_TUNER_LLM_MODEL", model)
		os.Setenv("OLLAMA_MODEL", model)
		return local.Start(local.StartOptions{
			Project:    projectName,
			SecretsDir: secretsDirOverride,
			Element:    "fingerprint-tuner",
			Enterprise: true,
		})
	}}
}

func modelShowCommand() *cobra.Command {
	return &cobra.Command{Use: "show", Short: "Show configured tuner model", RunE: func(cmd *cobra.Command, args []string) error {
		model := local.ResolveFingerprintTunerModel()
		ui.FHeader(cmd.OutOrStdout(), "Herringbone model")
		ui.FKeyValues(cmd.OutOrStdout(), [][2]string{{"model", model}})
		return nil
	}}
}

func dockerOllama(args ...string) (string, error) {
	container, err := ollamaContainerID()
	if err != nil {
		return "", err
	}
	full := append([]string{"exec", container, "ollama"}, args...)
	cmd := exec.Command("docker", full...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("docker ollama %s failed: %w\n%s", strings.Join(args, " "), err, buf.String())
	}
	return buf.String(), nil
}

func ollamaContainerID() (string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "label=com.docker.compose.project="+projectName, "--filter", "label=com.docker.compose.service=ollama", "--format", "{{.ID}}")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if id == "" {
		return "", fmt.Errorf("ollama container is not running")
	}
	return id, nil
}

func saveModelEnv(model string) error {
	path := filepath.Join(".", ".env")
	content, _ := os.ReadFile(path)
	lines := strings.Split(string(content), "\n")
	wanted := map[string]string{
		"FINGERPRINT_TUNER_LLM_MODEL": model,
		"OLLAMA_MODEL":                model,
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(lines)+len(wanted))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		replaced := false
		for key, value := range wanted {
			if strings.HasPrefix(trimmed, key+"=") {
				if !seen[key] {
					out = append(out, key+"="+value)
					seen[key] = true
				}
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, line)
		}
	}
	for key, value := range wanted {
		if !seen[key] {
			out = append(out, key+"="+value)
		}
	}
	return os.WriteFile(path, []byte(strings.TrimRight(strings.Join(out, "\n"), "\n")+"\n"), 0o644)
}

func readEnvModel() string {
	content, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "FINGERPRINT_TUNER_LLM_MODEL=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "FINGERPRINT_TUNER_LLM_MODEL="))
		}
	}
	return ""
}
