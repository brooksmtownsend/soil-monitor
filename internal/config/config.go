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
	"strconv"
	"strings"
)

const (
	defaultClientID     = "soil-monitor"
	defaultHTTPAddr     = ":80"
	defaultDatabasePath = "/var/lib/soil-monitor/soil-monitor.db"
)

var knownSettings = map[string]struct{}{
	"SOIL_MQTT_BROKER":    {},
	"SOIL_MQTT_CLIENT_ID": {},
	"SOIL_DATABASE_PATH":  {},
	"SOIL_HTTP_ENABLED":   {},
	"SOIL_HTTP_ADDR":      {},
}

type Config struct {
	BrokerURL    string
	ClientID     string
	DatabasePath string
	HTTPEnabled  bool
	HTTPAddr     string
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
	databasePath := values["SOIL_DATABASE_PATH"]
	httpValue := values["SOIL_HTTP_ENABLED"]
	httpAddr := values["SOIL_HTTP_ADDR"]

	fs := flag.NewFlagSet("soil-monitor", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() { writeUsage(output, fs) }
	fs.StringVar(&configPath, "config", configPath, "optional KEY=VALUE configuration file (no environment equivalent)")
	fs.StringVar(&brokerURL, "mqtt-broker", brokerURL, "MQTT broker URL (SOIL_MQTT_BROKER; required)")
	fs.StringVar(&clientID, "mqtt-client-id", clientID, "MQTT client identifier (SOIL_MQTT_CLIENT_ID)")
	fs.StringVar(&databasePath, "database-path", databasePath, "SQLite database path (SOIL_DATABASE_PATH)")
	fs.StringVar(&httpValue, "http", httpValue, "enable the embedded HTTP display: true or false (SOIL_HTTP_ENABLED)")
	fs.StringVar(&httpAddr, "http-addr", httpAddr, "HTTP listen address (SOIL_HTTP_ADDR)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	httpEnabled, err := boolean("SOIL_HTTP_ENABLED/--http", httpValue)
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(httpAddr) == "" {
		httpAddr = defaultHTTPAddr
	}

	cfg := Config{
		BrokerURL:    strings.TrimSpace(brokerURL),
		ClientID:     strings.TrimSpace(clientID),
		DatabasePath: strings.TrimSpace(databasePath),
		HTTPEnabled:  httpEnabled,
		HTTPAddr:     strings.TrimSpace(httpAddr),
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
		"SOIL_MQTT_CLIENT_ID": defaultClientID,
		"SOIL_DATABASE_PATH":  defaultDatabasePath,
		"SOIL_HTTP_ENABLED":   "true",
		"SOIL_HTTP_ADDR":      defaultHTTPAddr,
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
		if _, known := knownSettings[key]; known {
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

func writeUsage(output io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(output, "Usage: soil-monitor [options]")
	fmt.Fprintln(output, "Configuration precedence: flags > environment > --config file > defaults.")
	fmt.Fprintln(output, "Defaults: HTTP=true, client ID=soil-monitor, database=/var/lib/soil-monitor/soil-monitor.db, HTTP=:80.")
	fmt.Fprintln(output, "Environment: SOIL_MQTT_BROKER, SOIL_MQTT_CLIENT_ID, SOIL_DATABASE_PATH,")
	fmt.Fprintln(output, "  SOIL_HTTP_ENABLED, SOIL_HTTP_ADDR.")
	fmt.Fprintln(output, "Example: SOIL_MQTT_BROKER=tcp://127.0.0.1:1883 SOIL_DATABASE_PATH=./soil-monitor.db soil-monitor")
	fmt.Fprintln(output, "Options:")
	fs.PrintDefaults()
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
	if strings.TrimSpace(c.DatabasePath) == "" {
		return fmt.Errorf("SOIL_DATABASE_PATH must not be empty")
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
	return nil
}

func IsHelp(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}
