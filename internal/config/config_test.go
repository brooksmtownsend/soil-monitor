package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{BrokerURL: "tcp://localhost:1883", ClientID: "test-display", DatabasePath: "/tmp/soil.db", PIHatEnabled: true, SPIPort: "/dev/spidev0.0", HTTPEnabled: true, HTTPAddr: ":80", Debounce: 150 * time.Millisecond}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"valid", func(*Config) {}, ""},
		{"missing broker", func(c *Config) { c.BrokerURL = "" }, "SOIL_MQTT_BROKER is required"},
		{"bad broker scheme", func(c *Config) { c.BrokerURL = "http://localhost" }, "scheme"},
		{"empty database", func(c *Config) { c.DatabasePath = " " }, "SOIL_DATABASE_PATH"},
		{"HTTP address without port", func(c *Config) { c.HTTPAddr = "localhost" }, "host:port"},
		{"HTTP address with named port", func(c *Config) { c.HTTPAddr = ":http" }, "integer"},
		{"HTTP address with zero port", func(c *Config) { c.HTTPAddr = ":0" }, "1 through 65535"},
		{"disabled HTTP permits blank address", func(c *Config) { c.HTTPEnabled = false; c.HTTPAddr = "" }, ""},
		{"disabled HAT permits blank settings", func(c *Config) { c.PIHatEnabled = false; c.SPIPort = ""; c.Debounce = 0 }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error=%v want %q", err, tt.wantErr)
			}
		})
	}
}

func TestFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("SOIL_MQTT_BROKER", "tcp://broker.local:1883")
	t.Setenv("SOIL_BUTTON_DEBOUNCE_MS", "200")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Debounce != 200*time.Millisecond || cfg.ClientID != defaultClientID || cfg.DatabasePath != defaultDatabasePath || cfg.HTTPAddr != defaultHTTPAddr || !cfg.PIHatEnabled || !cfg.HTTPEnabled {
		t.Fatalf("config=%#v", cfg)
	}
}

func TestLoadPrecedenceAndQuotedFileValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soil-monitor.env")
	contents := "# file values\nSOIL_MQTT_BROKER='tcp://file:1883'\nSOIL_DATABASE_PATH='/file/soil.db'\nSOIL_HTTP_ENABLED=false\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	environ := []string{"SOIL_MQTT_BROKER=tcp://environment:1883", "SOIL_DATABASE_PATH=/environment/soil.db", "SOIL_HTTP_ENABLED=true"}
	cfg, err := Load([]string{"--config", path, "--mqtt-broker=tcp://flag:1883", "--database-path=/flag/soil.db", "--http=false", "--pi-hat=false"}, environ, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BrokerURL != "tcp://flag:1883" || cfg.DatabasePath != "/flag/soil.db" || cfg.HTTPEnabled || cfg.PIHatEnabled {
		t.Fatalf("config=%#v", cfg)
	}
}

func TestLoadEnvironmentOnlyAndDisabledValidation(t *testing.T) {
	environ := []string{"SOIL_MQTT_BROKER=tcp://broker:1883", "SOIL_DATABASE_PATH=/tmp/soil.db", "SOIL_PI_HAT_ENABLED=false", "SOIL_SPI_PORT=", "SOIL_BUTTON_DEBOUNCE_MS=invalid", "SOIL_HTTP_ENABLED=false", "SOIL_HTTP_ADDR="}
	cfg, err := Load(nil, environ, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PIHatEnabled || cfg.HTTPEnabled {
		t.Fatalf("config=%#v", cfg)
	}
}

func TestLoadRejectsMissingUnsafeAndEmptyDatabase(t *testing.T) {
	baseArgs := []string{"--pi-hat=false", "--http=false", "--mqtt-broker=tcp://broker:1883", "--database-path=/tmp/soil.db"}
	_, err := Load(append([]string{"--config", filepath.Join(t.TempDir(), "missing")}, baseArgs...), nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "config file") {
		t.Fatalf("missing file error=%v", err)
	}
	path := filepath.Join(t.TempDir(), "unsafe.env")
	if err := os.WriteFile(path, []byte("SOIL_MQTT_BROKER=$(touch nope)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(append([]string{"--config", path}, baseArgs...), nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("unsafe file error=%v", err)
	}
	if _, err := Load([]string{"--mqtt-broker=tcp://broker:1883", "--database-path="}, nil, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "SOIL_DATABASE_PATH") {
		t.Fatalf("empty database error=%v", err)
	}
}

func TestLoadRejectsInvalidBooleanAndArgument(t *testing.T) {
	environ := []string{"SOIL_MQTT_BROKER=tcp://broker:1883", "SOIL_PI_HAT_ENABLED=perhaps"}
	if _, err := Load(nil, environ, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "SOIL_PI_HAT_ENABLED") {
		t.Fatalf("error=%v", err)
	}
	if _, err := Load([]string{"--unknown"}, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown argument accepted")
	}
}

func TestFlagsOverrideMalformedLowerPriorityValues(t *testing.T) {
	environ := []string{"SOIL_MQTT_BROKER=tcp://broker:1883", "SOIL_PI_HAT_ENABLED=invalid", "SOIL_BUTTON_DEBOUNCE_MS=invalid", "SOIL_HTTP_ENABLED=invalid"}
	cfg, err := Load([]string{"--pi-hat=false", "--http=false"}, environ, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PIHatEnabled || cfg.HTTPEnabled {
		t.Fatalf("config=%#v", cfg)
	}
}

func TestLoadHelpDoesNotInitializeOrReadFile(t *testing.T) {
	var output bytes.Buffer
	_, err := Load([]string{"--config", filepath.Join(t.TempDir(), "missing"), "--help"}, nil, &output)
	if !IsHelp(err) {
		t.Fatalf("help error=%v", err)
	}
	for _, expected := range []string{"Configuration precedence", "SOIL_DATABASE_PATH", "--pi-hat=false", "-database-path"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("help missing %q:\n%s", expected, output.String())
		}
	}
}

func TestFromEnvHTTPAddressOverride(t *testing.T) {
	t.Setenv("SOIL_MQTT_BROKER", "tcp://broker.local:1883")
	t.Setenv("SOIL_HTTP_ADDR", "127.0.0.1:9090")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Fatalf("HTTPAddr=%q", cfg.HTTPAddr)
	}
}
