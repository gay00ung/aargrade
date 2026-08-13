package matrix

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gay00ung/aargrade/internal/toolchain"
	"go.yaml.in/yaml/v3"
)

var cellNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func loadConfig(options Options) (Config, string, error) {
	if options.ConfigPath == "" {
		options.ConfigPath = "aargrade.yml"
	}
	absolute, err := filepath.Abs(options.ConfigPath)
	if err != nil {
		return Config{}, "", fmt.Errorf("resolve matrix config: %w", err)
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return Config{}, "", fmt.Errorf("read matrix config: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, "", fmt.Errorf("parse matrix config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return Config{}, "", fmt.Errorf("parse matrix config: multiple YAML documents are not allowed")
	} else if !errors.Is(err, io.EOF) {
		return Config{}, "", fmt.Errorf("parse matrix config trailing document: %w", err)
	}
	if config.SchemaVersion != SchemaVersion {
		return Config{}, "", fmt.Errorf("unsupported matrix schemaVersion %d (expected %d)", config.SchemaVersion, SchemaVersion)
	}
	if options.CandidateAAR != "" {
		config.CandidateAAR = options.CandidateAAR
	}
	if options.BaselineAAR != "" {
		config.BaselineAAR = options.BaselineAAR
	}
	configDirectory := filepath.Dir(absolute)
	config.CandidateAAR = resolveConfigPath(configDirectory, config.CandidateAAR)
	config.BaselineAAR = resolveConfigPath(configDirectory, config.BaselineAAR)
	if err := validateConfig(config, options.SelectedCells); err != nil {
		return Config{}, "", err
	}
	return config, absolute, nil
}

func validateConfig(config Config, selected []string) error {
	if config.CandidateAAR == "" {
		return fmt.Errorf("candidateAar is required in the config or --candidate-aar")
	}
	if len(config.Cells) == 0 {
		return fmt.Errorf("matrix config must contain at least one cell")
	}
	seen := map[string]bool{}
	for index, cell := range config.Cells {
		if !cellNamePattern.MatchString(cell.Name) {
			return fmt.Errorf("cells[%d].name %q must match %s", index, cell.Name, cellNamePattern)
		}
		if seen[cell.Name] {
			return fmt.Errorf("duplicate matrix cell name %q", cell.Name)
		}
		seen[cell.Name] = true
		compatibility, err := toolchain.ForAGP(cell.AGP)
		if err != nil {
			return fmt.Errorf("cell %s: %w", cell.Name, err)
		}
		configuredGradle, err := toolchain.ParseVersion(cell.Gradle)
		if err != nil {
			return fmt.Errorf("cell %s: invalid Gradle version: %w", cell.Name, err)
		}
		minimumGradle, _ := toolchain.ParseVersion(compatibility.MinimumGradle)
		if configuredGradle.Compare(minimumGradle) < 0 {
			return fmt.Errorf("cell %s: Gradle %s is lower than AGP %s minimum %s", cell.Name, cell.Gradle, cell.AGP, compatibility.MinimumGradle)
		}
		if cell.JDK < compatibility.RecommendedJDK {
			return fmt.Errorf("cell %s: JDK %d is lower than AGP %s requirement %d", cell.Name, cell.JDK, cell.AGP, compatibility.RecommendedJDK)
		}
		if cell.CompileSDK <= 0 {
			return fmt.Errorf("cell %s: compileSdk must be positive", cell.Name)
		}
		if cell.MinSDK < 0 || cell.MinSDK > cell.CompileSDK {
			return fmt.Errorf("cell %s: minSdk must be omitted or between 1 and compileSdk", cell.Name)
		}
		language := strings.ToLower(cell.Language)
		if language != "java" && language != "kotlin" {
			return fmt.Errorf("cell %s: language must be java or kotlin", cell.Name)
		}
		agp, _ := toolchain.ParseVersion(cell.AGP)
		if language == "kotlin" && agp.Major < 9 && cell.Kotlin == "" {
			return fmt.Errorf("cell %s: kotlin version is required for Kotlin consumers before AGP 9", cell.Name)
		}
		if cell.Kotlin != "" {
			if _, err := toolchain.ParseVersion(cell.Kotlin); err != nil {
				return fmt.Errorf("cell %s: invalid Kotlin version: %w", cell.Name, err)
			}
		}
	}
	if len(selected) > 0 {
		var missing []string
		for _, name := range selected {
			if !seen[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return fmt.Errorf("selected matrix cell(s) not found: %s", strings.Join(missing, ", "))
		}
	}
	return nil
}

func resolveConfigPath(directory, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(directory, value))
}
