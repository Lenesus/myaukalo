package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port   int
	DBPath string
}

const (
	defaultPort = 8080
	defaultDB   = "myaukalo.db"

	envPort = "MYAUKALO_PORT"
	envDB   = "MYAUKALO_DB"
)

func Load(args []string) (Config, error) {
	basePort, baseDB, err := envDefaults()
	if err != nil {
		return Config{}, err
	}

	fs := flag.NewFlagSet("myaukalo", flag.ContinueOnError)
	port := fs.Int("port", basePort, fmt.Sprintf("TCP port to listen on (env %s)", envPort))
	db := fs.String("db", baseDB, fmt.Sprintf("path to the SQLite database file (env %s)", envDB))

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:   *port,
		DBPath: strings.TrimSpace(*db),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envDefaults() (int, string, error) {
	port := defaultPort
	if v := strings.TrimSpace(os.Getenv(envPort)); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return 0, "", fmt.Errorf("env %s must be an integer, got %q: %w", envPort, v, err)
		}
		port = p
	}

	db := defaultDB
	if v := os.Getenv(envDB); v != "" {
		db = v
	}

	return port, db, nil
}

func (c Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port %d is out of range 1-65535", c.Port)
	}
	if c.DBPath == "" {
		return errors.New("database path must not be empty")
	}
	return nil
}
