package config

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
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

var knownSettings = map[string]struct{}{
	"SOIL_MQTT_BROKER":        {},
	"SOIL_MQTT_CLIENT_ID":     {},
	"SOIL_PLANTS":             {},
	"SOIL_PI_HAT_ENABLED":     {},
	"SOIL_SPI_PORT":           {},
	"SOIL_BUTTON_DEBOUNCE_MS": {},
	"SOIL_HTTP_ENABLED":       {},
	"SOIL_HTTP_ADDR":          {},
}

type Plant struct {
	ID   string
	Name string
}

type Config struct {
	BrokerURL    string
	ClientID     string
	PIHatEnabled bool
	SPIPort      string
	HTTPEnabled  bool
	HTTPAddr     string
	Debounce     time.Duration
	Plants       []Plant
}

// FromEnv preserves the original environment-only entry point.
func FromEnv() (Config, error) {
	return Load(nil, os.Environ(), io.Discard)
}

// Load merges defaults, an explicitly selected file, the supplied environment,
// and explicit command-line flags, in that order.
func Load(args, environ []string, output io.Writer) (Config, error) {
	configPath, err := findConfigPath(args)
	if err != nil {
		return Config{}, err
	}

	values := defaultValues()
	if configPath != "" && !helpRequested(args) {
		fileValues, err := readFile(configPath)
		if err != nil {
			return Config{}, fmt.Errorf("config file %q: %w", configPath, err)
		}
		merge(values, fileValues)
	}
	merge(values, environmentValues(environ))

	brokerURL := values["SOIL_MQTT_BROKER"]
	clientID := values["SOIL_MQTT_CLIENT_ID"]
	plantsValue := values["SOIL_PLANTS"]
	piHatValue := values["SOIL_PI_HAT_ENABLED"]
	spiPort := values["SOIL_SPI_PORT"]
	debounceValue := values["SOIL_BUTTON_DEBOUNCE_MS"]
	httpValue := values["SOIL_HTTP_ENABLED"]
	httpAddr := values["SOIL_HTTP_ADDR"]

	fs := flag.NewFlagSet("soil-monitor", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() { writeUsage(output, fs) }
	fs.StringVar(&configPath, "config", configPath, "optional KEY=VALUE configuration file (no environment equivalent)")
	fs.StringVar(&brokerURL, "mqtt-broker", brokerURL, "MQTT broker URL (SOIL_MQTT_BROKER; required)")
	fs.StringVar(&clientID, "mqtt-client-id", clientID, "MQTT client identifier (SOIL_MQTT_CLIENT_ID)")
	fs.StringVar(&plantsValue, "plants", plantsValue, "three comma-separated id:name entries (SOIL_PLANTS; required)")
	fs.StringVar(&piHatValue, "pi-hat", piHatValue, "enable Raspberry Pi HAT display and buttons: true or false (SOIL_PI_HAT_ENABLED)")
	fs.StringVar(&spiPort, "spi-port", spiPort, "Pi HAT SPI device path (SOIL_SPI_PORT)")
	fs.StringVar(&debounceValue, "button-debounce-ms", debounceValue, "Pi HAT button debounce in milliseconds (SOIL_BUTTON_DEBOUNCE_MS)")
	fs.StringVar(&httpValue, "http", httpValue, "enable the embedded HTTP display: true or false (SOIL_HTTP_ENABLED)")
	fs.StringVar(&httpAddr, "http-addr", httpAddr, "HTTP listen address (SOIL_HTTP_ADDR)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	piHatEnabled, err := boolean("SOIL_PI_HAT_ENABLED/--pi-hat", piHatValue)
	if err != nil {
		return Config{}, err
	}
	debounceMS := int(defaultDebounce / time.Millisecond)
	if parsedDebounce, parseErr := positiveInteger("SOIL_BUTTON_DEBOUNCE_MS/--button-debounce-ms", debounceValue); parseErr == nil {
		debounceMS = parsedDebounce
	} else if piHatEnabled {
		return Config{}, parseErr
	}
	httpEnabled, err := boolean("SOIL_HTTP_ENABLED/--http", httpValue)
	if err != nil {
		return Config{}, err
	}

	plants, err := ParsePlants(plantsValue)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		BrokerURL:    strings.TrimSpace(brokerURL),
		ClientID:     strings.TrimSpace(clientID),
		PIHatEnabled: piHatEnabled,
		SPIPort:      strings.TrimSpace(spiPort),
		HTTPEnabled:  httpEnabled,
		HTTPAddr:     strings.TrimSpace(httpAddr),
		Debounce:     time.Duration(debounceMS) * time.Millisecond,
		Plants:       plants,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "-help" {
			return true
		}
	}
	return false
}

func defaultValues() map[string]string {
	return map[string]string{
		"SOIL_MQTT_CLIENT_ID":     defaultClientID,
		"SOIL_PI_HAT_ENABLED":     "true",
		"SOIL_SPI_PORT":           defaultSPIPort,
		"SOIL_BUTTON_DEBOUNCE_MS": strconv.Itoa(int(defaultDebounce / time.Millisecond)),
		"SOIL_HTTP_ENABLED":       "true",
		"SOIL_HTTP_ADDR":          defaultHTTPAddr,
	}
}

func findConfigPath(args []string) (string, error) {
	for i, arg := range args {
		if arg == "--config" || arg == "-config" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a path", arg)
			}
			return args[i+1], nil
		}
		if value, ok := strings.CutPrefix(arg, "--config="); ok {
			if value == "" {
				return "", fmt.Errorf("--config requires a path")
			}
			return value, nil
		}
	}
	return "", nil
}

func readFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("line %d must use KEY=VALUE format", lineNumber)
		}
		if _, known := knownSettings[key]; !known {
			return nil, fmt.Errorf("line %d contains unknown setting %q", lineNumber, key)
		}
		value = strings.TrimSpace(value)
		if strings.ContainsAny(value, "`$") {
			return nil, fmt.Errorf("line %d contains unsupported shell expansion or command syntax", lineNumber)
		}
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		} else if strings.HasPrefix(value, "'") || strings.HasPrefix(value, "\"") || strings.HasSuffix(value, "'") || strings.HasSuffix(value, "\"") {
			return nil, fmt.Errorf("line %d contains an unmatched quote", lineNumber)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func environmentValues(environ []string) map[string]string {
	values := make(map[string]string)
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, known := knownSettings[key]; known && strings.TrimSpace(value) != "" {
			values[key] = value
		}
	}
	return values
}

func merge(destination, source map[string]string) {
	for key, value := range source {
		destination[key] = value
	}
}

func boolean(name, raw string) (bool, error) {
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return value, nil
}

func positiveInteger(name, raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func writeUsage(output io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(output, "Usage: soil-monitor [options]")
	fmt.Fprintln(output, "Configuration precedence: flags > environment > --config file > defaults.")
	fmt.Fprintln(output, "Defaults: Pi HAT=true, HTTP=true, client ID=soil-monitor, SPI=/dev/spidev0.0, debounce=150ms, HTTP=:8080.")
	fmt.Fprintln(output, "Environment: SOIL_MQTT_BROKER, SOIL_MQTT_CLIENT_ID, SOIL_PLANTS,")
	fmt.Fprintln(output, "  SOIL_PI_HAT_ENABLED, SOIL_SPI_PORT, SOIL_BUTTON_DEBOUNCE_MS,")
	fmt.Fprintln(output, "  SOIL_HTTP_ENABLED, SOIL_HTTP_ADDR.")
	fmt.Fprintln(output, "Example: SOIL_MQTT_BROKER=tcp://127.0.0.1:1883 SOIL_PLANTS='one:One,two:Two,three:Three' soil-monitor --pi-hat=false")
	fmt.Fprintln(output, "Options:")
	fs.PrintDefaults()
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
		plants = append(plants, Plant{ID: strings.TrimSpace(fields[0]), Name: strings.TrimSpace(fields[1])})
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
	if c.PIHatEnabled {
		if strings.TrimSpace(c.SPIPort) == "" {
			return fmt.Errorf("SOIL_SPI_PORT must not be empty when Pi HAT support is enabled")
		}
		if c.Debounce <= 0 {
			return fmt.Errorf("button debounce must be positive when Pi HAT support is enabled")
		}
	}
	if c.HTTPEnabled {
		_, port, err := net.SplitHostPort(c.HTTPAddr)
		if err != nil || port == "" {
			return fmt.Errorf("SOIL_HTTP_ADDR must be a valid host:port listen address")
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return fmt.Errorf("SOIL_HTTP_ADDR port must be an integer from 1 through 65535")
		}
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

func IsHelp(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}
