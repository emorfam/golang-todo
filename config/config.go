package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Config holds all application configuration.
type Config struct {
	Port             string `koanf:"PORT"`
	LogFormat        string `koanf:"LOG_FORMAT"`
	Env              string `koanf:"ENV"`
	DBDriver         string `koanf:"DB_DRIVER"`
	DatabaseURL      string `koanf:"DATABASE_URL"`
	JWTPublicKeyPath string `koanf:"JWT_PUBLIC_KEY_PATH"`
	OTLPEndpoint     string `koanf:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// Load reads configuration from environment variables with sensible defaults.
// If a .env file exists in the current directory it is loaded first; actual
// environment variables always take precedence over values in the file.
func Load() (*Config, error) {
	loadDotEnv(".env")
	k := koanf.New(".")

	// Set defaults by loading a raw map first.
	defaults := map[string]any{
		"PORT":                        "8080",
		"LOG_FORMAT":                  "text",
		"ENV":                         "development",
		"DB_DRIVER":                   "sqlite",
		"DATABASE_URL":                "./dev.db",
		"JWT_PUBLIC_KEY_PATH":         "./keys/public.pem",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317",
	}
	if err := k.Load(rawMapProvider(defaults), nil); err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// Overlay with environment variables.
	if err := k.Load(env.Provider("", ".", func(s string) string {
		return strings.ToUpper(s)
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env config: %w", err)
	}

	cfg := &Config{}
	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return fmt.Errorf("DB_DRIVER must be 'sqlite' or 'postgres', got %q", c.DBDriver)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	return nil
}

// loadDotEnv reads KEY=VALUE pairs from path and sets them as environment
// variables. Lines that are blank or start with # are ignored. Inline comments
// (anything after the first " #" or "\t#") are stripped. Variables that are
// already present in the environment are never overwritten.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file is optional; missing is not an error
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip inline comment: first occurrence of " #" or "\t#".
		for _, sep := range []string{" #", "\t#"} {
			if idx := strings.Index(line, sep); idx != -1 {
				line = strings.TrimSpace(line[:idx])
			}
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		// Real environment variables take precedence.
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}

// rawMapProvider implements koanf.Provider for a static map of defaults.
type rawMapProvider map[string]interface{}

func (r rawMapProvider) ReadBytes() ([]byte, error) { return nil, nil }

func (r rawMapProvider) Read() (map[string]interface{}, error) {
	return map[string]interface{}(r), nil
}
