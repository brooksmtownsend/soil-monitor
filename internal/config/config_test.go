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
	return Config{
		BrokerURL:    "tcp://localhost:1883",
		ClientID:     "test-display",
		PIHatEnabled: true,
		SPIPort:      "/dev/spidev0.0",
		HTTPEnabled:  true,
		HTTPAddr:     ":8080",
		Debounce:     150 * time.Millisecond,
		Plants: []Plant{
			{ID: "money-tree", Name: "Money Tree"},
			{ID: "pothos", Name: "Pothos"},
			{ID: "snake-plant", Name: "Snake Plant"},
		},
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "valid", mutate: func(*Config) {}},
		{name: "missing broker", mutate: func(c *Config) { c.BrokerURL = "" }, wantErr: "SOIL_MQTT_BROKER is required"},
		{name: "bad broker scheme", mutate: func(c *Config) { c.BrokerURL = "http://localhost" }, wantErr: "scheme"},
		{name: "HTTP address without port", mutate: func(c *Config) { c.HTTPAddr = "localhost" }, wantErr: "host:port"},
		{name: "HTTP address with named port", mutate: func(c *Config) { c.HTTPAddr = ":http" }, wantErr: "integer"},
		{name: "HTTP address with zero port", mutate: func(c *Config) { c.HTTPAddr = ":0" }, wantErr: "1 through 65535"},
		{name: "disabled HTTP permits blank address", mutate: func(c *Config) { c.HTTPEnabled = false; c.HTTPAddr = "" }},
		{name: "disabled HAT permits blank hardware settings", mutate: func(c *Config) { c.PIHatEnabled = false; c.SPIPort = ""; c.Debounce = 0 }},
		{name: "wrong count", mutate: func(c *Config) { c.Plants = c.Plants[:2] }, wantErr: "exactly three"},
		{name: "bad id", mutate: func(c *Config) { c.Plants[0].ID = "Money Tree" }, wantErr: "kebab-case"},
		{name: "blank name", mutate: func(c *Config) { c.Plants[0].Name = " " }, wantErr: "non-empty"},
		{name: "duplicate", mutate: func(c *Config) { c.Plants[1].ID = c.Plants[0].ID }, wantErr: "duplicated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestParsePlants(t *testing.T) {
	plants, err := ParsePlants("money-tree:Money Tree,pothos:Pothos,snake-plant:Snake Plant")
	if err != nil {
		t.Fatal(err)
	}
	if len(plants) != 3 || plants[2].ID != "snake-plant" || plants[0].Name != "Money Tree" {
		t.Fatalf("unexpected plants: %#v", plants)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("SOIL_MQTT_BROKER", "tcp://broker.local:1883")
	t.Setenv("SOIL_PLANTS", "money-tree:Money Tree,pothos:Pothos,snake-plant:Snake Plant")
	t.Setenv("SOIL_BUTTON_DEBOUNCE_MS", "200")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Debounce != 200*time.Millisecond || cfg.ClientID != defaultClientID || cfg.SPIPort != defaultSPIPort || cfg.HTTPAddr != defaultHTTPAddr || !cfg.PIHatEnabled || !cfg.HTTPEnabled {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadPrecedenceAndQuotedFileValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soil-monitor.env")
	contents := "# file values\nSOIL_MQTT_BROKER='tcp://file:1883'\nSOIL_PLANTS=one:One,two:Two,three:Three\nSOIL_HTTP_ENABLED=false\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	environ := []string{"SOIL_MQTT_BROKER=tcp://environment:1883", "SOIL_HTTP_ENABLED=true"}
	cfg, err := Load([]string{"--config", path, "--mqtt-broker=tcp://flag:1883", "--http=false", "--pi-hat=false"}, environ, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BrokerURL != "tcp://flag:1883" || cfg.HTTPEnabled || cfg.PIHatEnabled || cfg.Plants[0].Name != "One" {
		t.Fatalf("unexpected merged config: %#v", cfg)
	}
}

func TestLoadEnvironmentOnlyAndDisabledValidation(t *testing.T) {
	environ := []string{
		"SOIL_MQTT_BROKER=tcp://broker:1883",
		"SOIL_PLANTS=one:One,two:Two,three:Three",
		"SOIL_PI_HAT_ENABLED=false",
		"SOIL_SPI_PORT=",
		"SOIL_BUTTON_DEBOUNCE_MS=invalid",
		"SOIL_HTTP_ENABLED=false",
		"SOIL_HTTP_ADDR=invalid",
	}
	cfg, err := Load(nil, environ, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PIHatEnabled || cfg.HTTPEnabled {
		t.Fatalf("unexpected enabled adapters: %#v", cfg)
	}
}

func TestLoadRejectsMissingAndUnsafeFiles(t *testing.T) {
	baseArgs := []string{"--pi-hat=false", "--http=false", "--mqtt-broker=tcp://broker:1883", "--plants=one:One,two:Two,three:Three"}
	_, err := Load(append([]string{"--config", filepath.Join(t.TempDir(), "missing")}, baseArgs...), nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "config file") {
		t.Fatalf("missing file error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "unsafe.env")
	if err := os.WriteFile(path, []byte("SOIL_MQTT_BROKER=$(touch nope)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(append([]string{"--config", path}, baseArgs...), nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("unsafe file error = %v", err)
	}
}

func TestLoadRejectsInvalidBooleanAndArgument(t *testing.T) {
	environ := []string{"SOIL_MQTT_BROKER=tcp://broker:1883", "SOIL_PLANTS=one:One,two:Two,three:Three", "SOIL_PI_HAT_ENABLED=perhaps"}
	if _, err := Load(nil, environ, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "SOIL_PI_HAT_ENABLED") {
		t.Fatalf("invalid boolean error = %v", err)
	}
	if _, err := Load([]string{"--unknown"}, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown argument was accepted")
	}
}

func TestFlagsOverrideMalformedLowerPriorityValues(t *testing.T) {
	environ := []string{
		"SOIL_MQTT_BROKER=tcp://broker:1883",
		"SOIL_PLANTS=one:One,two:Two,three:Three",
		"SOIL_PI_HAT_ENABLED=invalid",
		"SOIL_BUTTON_DEBOUNCE_MS=invalid",
		"SOIL_HTTP_ENABLED=invalid",
	}
	cfg, err := Load([]string{"--pi-hat=false", "--http=false"}, environ, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PIHatEnabled || cfg.HTTPEnabled {
		t.Fatalf("unexpected merged config: %#v", cfg)
	}
}

func TestLoadHelp(t *testing.T) {
	var output bytes.Buffer
	_, err := Load([]string{"--help"}, nil, &output)
	if !IsHelp(err) {
		t.Fatalf("help error = %v", err)
	}
	for _, expected := range []string{"Configuration precedence", "SOIL_PI_HAT_ENABLED", "--pi-hat=false", "-mqtt-broker"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("help missing %q:\n%s", expected, output.String())
		}
	}
}

func TestLoadHelpDoesNotReadSelectedFile(t *testing.T) {
	var output bytes.Buffer
	_, err := Load([]string{"--config", filepath.Join(t.TempDir(), "missing"), "--help"}, nil, &output)
	if !IsHelp(err) {
		t.Fatalf("help error = %v", err)
	}
}

func TestFromEnvHTTPAddressOverride(t *testing.T) {
	t.Setenv("SOIL_MQTT_BROKER", "tcp://broker.local:1883")
	t.Setenv("SOIL_PLANTS", "money-tree:Money Tree,pothos:Pothos,snake-plant:Snake Plant")
	t.Setenv("SOIL_HTTP_ADDR", "127.0.0.1:9090")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
}
