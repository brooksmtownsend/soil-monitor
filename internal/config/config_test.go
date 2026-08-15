package config

import (
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		BrokerURL: "tcp://localhost:1883",
		ClientID:  "test-display",
		SPIPort:   "/dev/spidev0.0",
		HTTPAddr:  ":8080",
		Debounce:  150 * time.Millisecond,
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
	if cfg.Debounce != 200*time.Millisecond || cfg.ClientID != defaultClientID || cfg.SPIPort != defaultSPIPort || cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("unexpected config: %#v", cfg)
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
