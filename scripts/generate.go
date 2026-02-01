//go:build ignore

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	configFile     = "stromboli.yaml"
	swaggerURLTmpl = "https://raw.githubusercontent.com/tomblancdev/stromboli/v%s/docs/swagger/swagger.yaml"
	outputDir      = "generated"
)

// Go package prefixes to remove from swagger definitions
var packagePrefixes = []string{
	"internal_api.",
	"stromboli_internal_job.",
	"stromboli_internal_session.",
}

type Config struct {
	APIVersion      string `yaml:"apiVersion"`
	APIVersionRange string `yaml:"apiVersionRange"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Read config
	cfg, err := readConfig(configFile)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	fmt.Printf("Target API version: %s\n", cfg.APIVersion)

	// Fetch swagger spec
	swaggerURL := fmt.Sprintf(swaggerURLTmpl, cfg.APIVersion)
	fmt.Printf("Fetching: %s\n", swaggerURL)

	swaggerPath := filepath.Join(outputDir, "swagger.yaml")
	if err := downloadFile(swaggerURL, swaggerPath); err != nil {
		return fmt.Errorf("downloading swagger: %w", err)
	}

	// Normalize swagger (remove Go package prefixes)
	fmt.Println("Normalizing swagger definitions...")
	if err := normalizeSwagger(swaggerPath); err != nil {
		return fmt.Errorf("normalizing swagger: %w", err)
	}

	// Generate client using go-swagger
	fmt.Println("Generating client...")
	if err := generateClient(swaggerPath); err != nil {
		return fmt.Errorf("generating client: %w", err)
	}

	fmt.Println("Done!")
	return nil
}

func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// normalizeSwagger removes Go package prefixes from definition names and references.
// This is needed because swaggo generates definitions like "internal_api.RefreshRequest"
// which code generators cannot handle.
func normalizeSwagger(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)

	// Remove package prefixes from definitions and references
	for _, prefix := range packagePrefixes {
		// Escape dots for regex
		escapedPrefix := strings.ReplaceAll(prefix, ".", `\.`)

		// Replace in definition names and $ref values
		re := regexp.MustCompile(escapedPrefix)
		content = re.ReplaceAllString(content, "")
	}

	return os.WriteFile(path, []byte(content), 0644)
}

func generateClient(swaggerPath string) error {
	// go-swagger generate client
	cmd := exec.Command("swagger", "generate", "client",
		"-f", swaggerPath,
		"-t", outputDir,
		"--skip-validation",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
