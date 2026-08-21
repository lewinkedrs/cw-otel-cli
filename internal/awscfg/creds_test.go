package awscfg

import (
	"context"
	"testing"
)

func TestLoadResolvesRegionFromEnvWhenUnset(t *testing.T) {
	// Isolate from any ambient AWS region config.
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_REGION", "eu-west-1")

	cfg, err := Load(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Region != "eu-west-1" {
		t.Errorf("region = %q, want eu-west-1 (from AWS_REGION)", cfg.Region)
	}
}

func TestLoadExplicitRegionOverridesEnv(t *testing.T) {
	t.Setenv("AWS_REGION", "eu-west-1")

	cfg, err := Load(context.Background(), "us-west-2", "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("region = %q, want us-west-2 (explicit overrides env)", cfg.Region)
	}
}
