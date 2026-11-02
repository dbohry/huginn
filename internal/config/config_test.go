package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HUGINN_ADDR", "")
	t.Setenv("HUGINN_DATA_DIR", "")

	cfg := Load()
	if cfg.Addr != ":8080" || cfg.DataDir != "/data" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if got := cfg.DBPath(); got != "/data/huginn.db" {
		t.Fatalf("DBPath = %q", got)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("HUGINN_ADDR", "127.0.0.1:9000")
	t.Setenv("HUGINN_DATA_DIR", "/tmp/huginn")

	cfg := Load()
	if cfg.Addr != "127.0.0.1:9000" || cfg.DataDir != "/tmp/huginn" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestHealthURL(t *testing.T) {
	tests := map[string]string{
		":8080":          "http://127.0.0.1:8080/healthz",
		"0.0.0.0:9000":   "http://127.0.0.1:9000/healthz",
		"[::]:7000":      "http://127.0.0.1:7000/healthz",
		"not-an-address": "http://127.0.0.1:8080/healthz",
	}
	for addr, want := range tests {
		if got := (Config{Addr: addr}).HealthURL(); got != want {
			t.Errorf("HealthURL(%q) = %q, want %q", addr, got, want)
		}
	}
}

func TestBlockPrivateSwitch(t *testing.T) {
	for value, want := range map[string]bool{
		"": false, "false": false, "0": false, "no": false, "nonsense": false,
		"1": true, "true": true, "TRUE": true, " yes ": true, "on": true,
	} {
		t.Setenv("HUGINN_BLOCK_PRIVATE_NETWORKS", value)
		if got := Load().BlockPrivate; got != want {
			t.Errorf("HUGINN_BLOCK_PRIVATE_NETWORKS=%q: BlockPrivate = %v, want %v", value, got, want)
		}
	}
}
