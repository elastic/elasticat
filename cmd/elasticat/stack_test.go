// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elastic/elasticat/internal/config"
)

func TestEnsureStartLocalProfile_CreatesProfile(t *testing.T) {
	// Use a temp directory for testing
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Ensure no profile exists initially
	cfg, err := config.LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles error: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Fatalf("expected empty profiles, got %d", len(cfg.Profiles))
	}

	// Run ensureStartLocalProfile
	if err := ensureStartLocalProfile(); err != nil {
		t.Fatalf("ensureStartLocalProfile error: %v", err)
	}

	// Verify profile was created
	cfg, err = config.LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles error: %v", err)
	}

	p, err := cfg.GetProfile(config.StartLocalProfileName)
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}

	if p.Source != config.ProfileSourceStartLocal {
		t.Errorf("Source = %q, want %q", p.Source, config.ProfileSourceStartLocal)
	}

	if cfg.CurrentProfile != config.StartLocalProfileName {
		t.Errorf("CurrentProfile = %q, want %q", cfg.CurrentProfile, config.StartLocalProfileName)
	}
}

func TestEnsureStartLocalProfile_DoesNotOverwrite(t *testing.T) {
	// Use a temp directory for testing
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Create a profile with the same name but different source
	cfg := &config.ProfileConfig{
		Profiles: map[string]config.Profile{
			config.StartLocalProfileName: {
				Source: config.ProfileSourceStartLocal,
				Elasticsearch: config.ESProfile{
					URL: "http://custom:9200",
				},
			},
		},
	}
	if err := config.SaveProfiles(cfg); err != nil {
		t.Fatalf("SaveProfiles error: %v", err)
	}

	// Run ensureStartLocalProfile
	if err := ensureStartLocalProfile(); err != nil {
		t.Fatalf("ensureStartLocalProfile error: %v", err)
	}

	// Verify profile was NOT overwritten (still has source)
	cfg, err := config.LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles error: %v", err)
	}

	p, err := cfg.GetProfile(config.StartLocalProfileName)
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}

	// Source should still be start-local
	if p.Source != config.ProfileSourceStartLocal {
		t.Errorf("Source = %q, want %q", p.Source, config.ProfileSourceStartLocal)
	}
}

func TestIsStartLocalInstalled(t *testing.T) {
	// Use a temp home directory
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tempDir)
	defer func() { os.Setenv("HOME", origHome) }()

	// Initially should not be installed
	if config.IsStartLocalInstalled() {
		t.Error("expected IsStartLocalInstalled() = false")
	}

	// Create the .env file
	startLocalDir := filepath.Join(tempDir, ".elasticat", "elastic-start-local")
	if err := os.MkdirAll(startLocalDir, 0755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	envPath := filepath.Join(startLocalDir, ".env")
	if err := os.WriteFile(envPath, []byte("ES_LOCAL_URL=http://localhost:9200"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Now should be installed
	if !config.IsStartLocalInstalled() {
		t.Error("expected IsStartLocalInstalled() = true")
	}
}
