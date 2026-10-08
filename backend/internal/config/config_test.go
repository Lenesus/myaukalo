package config

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil): unexpected error: %v", err)
	}
	if cfg.Port != defaultPort || cfg.DBPath != defaultDB {
		t.Fatalf("expected defaults %d/%q, got %d/%q",
			defaultPort, defaultDB, cfg.Port, cfg.DBPath)
	}
}

func TestLoad_FlagOverridesDefault(t *testing.T) {
	cfg, err := Load([]string{"-port", "9090", "--db", "/tmp/mau.db"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 || cfg.DBPath != "/tmp/mau.db" {
		t.Fatalf("flags not applied: %+v", cfg)
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	t.Setenv(envPort, "7070")
	t.Setenv(envDB, "from-env.db")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 7070 || cfg.DBPath != "from-env.db" {
		t.Fatalf("env not applied: %+v", cfg)
	}
}

func TestLoad_FlagOverridesEnv(t *testing.T) {
	t.Setenv(envPort, "7070")

	cfg, err := Load([]string{"-port", "9090"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 {
		t.Fatalf("flag must win over env: got port %d, want 9090", cfg.Port)
	}
}

func TestLoad_InvalidEnvPort(t *testing.T) {
	t.Setenv(envPort, "не-число")

	_, err := Load(nil)
	if err == nil {
		t.Fatal("expected error for non-integer env port")
	}
	if !strings.Contains(err.Error(), envPort) {
		t.Fatalf("error should mention %s, got: %v", envPort, err)
	}
}

func TestLoad_PortOutOfRange(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"zero", []string{"-port", "0"}},
		{"negative", []string{"-port", "-1"}},
		{"too big", []string{"-port", "70000"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(tc.args); err == nil {
				t.Fatalf("expected error for %v, got nil", tc.args)
			}
		})
	}
}

func TestLoad_EmptyDBAfterTrim(t *testing.T) {
	if _, err := Load([]string{"-db", "   "}); err == nil {
		t.Fatal("expected error for blank database path")
	}
}

func TestLoad_HelpReturnsErrHelp(t *testing.T) {
	if _, err := Load([]string{"-h"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}
