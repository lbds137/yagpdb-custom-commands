package main

import "testing"

// TestProveConfig pins prove's CI propagation: the RunnerConfig prove builds must
// carry CI from the environment, so a missing snapshot fails under CI instead of
// being written (loader.RunnerConfig.CI, consumed in snapshot.go). The other fields
// pass through unchanged. The schema argument is nil here: proveConfig only stores
// it in the config, so the pointer's value is irrelevant to what this test pins.
func TestProveConfig(t *testing.T) {
	t.Setenv("CI", "true")
	cfg := proveConfig("base/dir", nil, "template/root")
	if !cfg.CI {
		t.Errorf("CI=true: cfg.CI = false, want true")
	}
	if cfg.BaseDir != "base/dir" {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, "base/dir")
	}
	if cfg.TemplateRoot != "template/root" {
		t.Errorf("TemplateRoot = %q, want %q", cfg.TemplateRoot, "template/root")
	}

	t.Setenv("CI", "")
	cfg = proveConfig("base/dir", nil, "template/root")
	if cfg.CI {
		t.Errorf("CI unset: cfg.CI = true, want false")
	}
}
