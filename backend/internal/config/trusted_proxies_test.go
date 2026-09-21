package config

import "testing"

func TestConfig_TrustedProxies(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8, 2001:db8:ffff::/48 ")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0].String() != "10.0.0.0/8" {
			t.Fatalf("TrustedProxies = %v", cfg.TrustedProxies)
		}
	})
	t.Run("empty", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("TRUSTED_PROXIES", "  , ")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.TrustedProxies) != 0 {
			t.Fatalf("TrustedProxies = %v, want none", cfg.TrustedProxies)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, invalid")
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want invalid TRUSTED_PROXIES error")
		}
	})
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://identity")
	t.Setenv("RABBITMQ_URL", "amqp://identity")
	t.Setenv("JWT_SIGNING_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
}
