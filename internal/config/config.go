package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultClientID = "soil-monitor"
	defaultSPIPort  = "/dev/spidev0.0"
	defaultHTTPAddr = ":8080"
	defaultDebounce = 150 * time.Millisecond
)

var plantIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Plant struct {
	ID   string
	Name string
}

type Config struct {
	BrokerURL string
	ClientID  string
	SPIPort   string
	HTTPAddr  string
	Debounce  time.Duration
	Plants    []Plant
}

func FromEnv() (Config, error) {
	debounce := defaultDebounce
	if raw := strings.TrimSpace(os.Getenv("SOIL_BUTTON_DEBOUNCE_MS")); raw != "" {
		milliseconds, err := strconv.Atoi(raw)
		if err != nil || milliseconds < 1 {
			return Config{}, fmt.Errorf("SOIL_BUTTON_DEBOUNCE_MS must be a positive integer")
		}
		debounce = time.Duration(milliseconds) * time.Millisecond
	}

	cfg := Config{
		BrokerURL: strings.TrimSpace(os.Getenv("SOIL_MQTT_BROKER")),
		ClientID:  envOrDefault("SOIL_MQTT_CLIENT_ID", defaultClientID),
		SPIPort:   envOrDefault("SOIL_SPI_PORT", defaultSPIPort),
		HTTPAddr:  envOrDefault("SOIL_HTTP_ADDR", defaultHTTPAddr),
		Debounce:  debounce,
	}

	plants, err := ParsePlants(os.Getenv("SOIL_PLANTS"))
	if err != nil {
		return Config{}, err
	}
	cfg.Plants = plants
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func ParsePlants(raw string) ([]Plant, error) {
	parts := strings.Split(raw, ",")
	if strings.TrimSpace(raw) == "" || len(parts) != 3 {
		return nil, fmt.Errorf("SOIL_PLANTS must contain exactly three comma-separated id:name entries")
	}

	plants := make([]Plant, 0, 3)
	for i, part := range parts {
		fields := strings.SplitN(part, ":", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("SOIL_PLANTS entry %d must use id:name format", i+1)
		}
		plants = append(plants, Plant{
			ID:   strings.TrimSpace(fields[0]),
			Name: strings.TrimSpace(fields[1]),
		})
	}
	return plants, nil
}

func (c Config) Validate() error {
	if c.BrokerURL == "" {
		return fmt.Errorf("SOIL_MQTT_BROKER is required")
	}
	parsed, err := url.Parse(c.BrokerURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("SOIL_MQTT_BROKER must be a valid MQTT URL")
	}
	switch parsed.Scheme {
	case "tcp", "tls", "ws", "wss":
	default:
		return fmt.Errorf("SOIL_MQTT_BROKER scheme must be tcp, tls, ws, or wss")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("SOIL_MQTT_CLIENT_ID must not be empty")
	}
	if strings.TrimSpace(c.SPIPort) == "" {
		return fmt.Errorf("SOIL_SPI_PORT must not be empty")
	}
	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil || port == "" {
		return fmt.Errorf("SOIL_HTTP_ADDR must be a valid host:port listen address")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("SOIL_HTTP_ADDR port must be an integer from 1 through 65535")
	}
	if c.Debounce <= 0 {
		return fmt.Errorf("button debounce must be positive")
	}
	if len(c.Plants) != 3 {
		return fmt.Errorf("exactly three plants are required")
	}

	seen := make(map[string]struct{}, len(c.Plants))
	for i, plant := range c.Plants {
		if !plantIDPattern.MatchString(plant.ID) {
			return fmt.Errorf("plant %d identifier %q must be kebab-case", i+1, plant.ID)
		}
		if strings.TrimSpace(plant.Name) == "" {
			return fmt.Errorf("plant %q must have a non-empty display name", plant.ID)
		}
		if _, exists := seen[plant.ID]; exists {
			return fmt.Errorf("plant identifier %q is duplicated", plant.ID)
		}
		seen[plant.ID] = struct{}{}
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
