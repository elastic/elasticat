// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileConfig_GetProfile(t *testing.T) {
	cfg := &ProfileConfig{
		Profiles: map[string]Profile{
			"test": {
				Elasticsearch: ESProfile{URL: "http://test:9200"},
			},
		},
	}

	// Test getting existing profile
	p, err := cfg.GetProfile("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Elasticsearch.URL != "http://test:9200" {
		t.Errorf("URL = %q, want %q", p.Elasticsearch.URL, "http://test:9200")
	}

	// Test getting non-existent profile
	_, err = cfg.GetProfile("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestProfileConfig_SetProfile(t *testing.T) {
	cfg := &ProfileConfig{}

	profile := Profile{
		Elasticsearch: ESProfile{URL: "http://new:9200"},
		OTLP:          OTLPProfile{Endpoint: "new:4318"},
		Kibana:        KibanaProfile{URL: "http://new:5601"},
	}

	cfg.SetProfile("new", profile)

	p, err := cfg.GetProfile("new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Elasticsearch.URL != "http://new:9200" {
		t.Errorf("ES URL = %q, want %q", p.Elasticsearch.URL, "http://new:9200")
	}
	if p.OTLP.Endpoint != "new:4318" {
		t.Errorf("OTLP Endpoint = %q, want %q", p.OTLP.Endpoint, "new:4318")
	}
	if p.Kibana.URL != "http://new:5601" {
		t.Errorf("Kibana URL = %q, want %q", p.Kibana.URL, "http://new:5601")
	}
}

func TestProfileConfig_DeleteProfile(t *testing.T) {
	cfg := &ProfileConfig{
		CurrentProfile: "test",
		Profiles: map[string]Profile{
			"test":  {Elasticsearch: ESProfile{URL: "http://test:9200"}},
			"other": {Elasticsearch: ESProfile{URL: "http://other:9200"}},
		},
	}

	// Delete existing profile
	err := cfg.DeleteProfile("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deleted
	_, err = cfg.GetProfile("test")
	if err == nil {
		t.Error("expected error after delete")
	}

	// Verify current profile cleared
	if cfg.CurrentProfile != "" {
		t.Errorf("CurrentProfile = %q, want empty", cfg.CurrentProfile)
	}

	// Delete non-existent profile
	err = cfg.DeleteProfile("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestProfileConfig_ListProfiles(t *testing.T) {
	cfg := &ProfileConfig{
		Profiles: map[string]Profile{
			"a": {},
			"b": {},
			"c": {},
		},
	}

	names := cfg.ListProfiles()
	if len(names) != 3 {
		t.Errorf("len = %d, want 3", len(names))
	}

	// Verify all names present (order may vary)
	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !found[want] {
			t.Errorf("missing profile %q", want)
		}
	}
}

func TestProfileConfig_GetActiveProfile(t *testing.T) {
	cfg := &ProfileConfig{
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"default":  {Elasticsearch: ESProfile{URL: "http://default:9200"}},
			"override": {Elasticsearch: ESProfile{URL: "http://override:9200"}},
		},
	}

	// Test with flag override
	p, name := cfg.GetActiveProfile("override")
	if name != "override" {
		t.Errorf("name = %q, want %q", name, "override")
	}
	if p == nil || p.Elasticsearch.URL != "http://override:9200" {
		t.Error("wrong profile returned for flag override")
	}

	// Test with current profile
	p, name = cfg.GetActiveProfile("")
	if name != "default" {
		t.Errorf("name = %q, want %q", name, "default")
	}
	if p == nil || p.Elasticsearch.URL != "http://default:9200" {
		t.Error("wrong profile returned for current profile")
	}

	// Test with no profile
	cfg.CurrentProfile = ""
	p, name = cfg.GetActiveProfile("")
	if name != "" {
		t.Errorf("name = %q, want empty", name)
	}
	if p != nil {
		t.Error("expected nil profile")
	}
}

func TestIsEnvRef(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"${MY_VAR}", true},
		{"${API_KEY}", true},
		{"${}", false}, // Empty var name - doesn't match pattern
		{"$MY_VAR", false},
		{"MY_VAR", false},
		{"${MY_VAR", false},
		{"MY_VAR}", false},
		{"plain text", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsEnvRef(tt.input)
			if got != tt.want {
				t.Errorf("IsEnvRef(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestProfile_Resolve(t *testing.T) {
	// Set up test env vars
	t.Setenv("TEST_API_KEY", "secret-key")
	t.Setenv("TEST_USER", "testuser")
	t.Setenv("TEST_PASS", "testpass")

	profile := Profile{
		Elasticsearch: ESProfile{
			URL:      "http://test:9200",
			APIKey:   "${TEST_API_KEY}",
			Username: "${TEST_USER}",
			Password: "${TEST_PASS}",
		},
	}

	resolved, err := profile.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Elasticsearch.APIKey != "secret-key" {
		t.Errorf("APIKey = %q, want %q", resolved.Elasticsearch.APIKey, "secret-key")
	}
	if resolved.Elasticsearch.Username != "testuser" {
		t.Errorf("Username = %q, want %q", resolved.Elasticsearch.Username, "testuser")
	}
	if resolved.Elasticsearch.Password != "testpass" {
		t.Errorf("Password = %q, want %q", resolved.Elasticsearch.Password, "testpass")
	}

	// Test with undefined env var
	profile.Elasticsearch.APIKey = "${UNDEFINED_VAR}"
	_, err = profile.Resolve()
	if err == nil {
		t.Error("expected error for undefined env var")
	}
}

func TestProfile_HasCredentials(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    bool
	}{
		{
			name:    "no credentials",
			profile: Profile{Elasticsearch: ESProfile{URL: "http://test:9200"}},
			want:    false,
		},
		{
			name:    "api key",
			profile: Profile{Elasticsearch: ESProfile{APIKey: "key"}},
			want:    true,
		},
		{
			name:    "username",
			profile: Profile{Elasticsearch: ESProfile{Username: "user"}},
			want:    true,
		},
		{
			name:    "password",
			profile: Profile{Elasticsearch: ESProfile{Password: "pass"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.profile.HasCredentials()
			if got != tt.want {
				t.Errorf("HasCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfile_HasPlainTextCredentials(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    bool
	}{
		{
			name:    "no credentials",
			profile: Profile{},
			want:    false,
		},
		{
			name:    "env var api key",
			profile: Profile{Elasticsearch: ESProfile{APIKey: "${MY_KEY}"}},
			want:    false,
		},
		{
			name:    "plain text api key",
			profile: Profile{Elasticsearch: ESProfile{APIKey: "plain-key"}},
			want:    true,
		},
		{
			name:    "plain text password",
			profile: Profile{Elasticsearch: ESProfile{Password: "secret"}},
			want:    true,
		},
		{
			name:    "mixed env and plain",
			profile: Profile{Elasticsearch: ESProfile{APIKey: "${KEY}", Password: "plain"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.profile.HasPlainTextCredentials()
			if got != tt.want {
				t.Errorf("HasPlainTextCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfile_MaskCredentials(t *testing.T) {
	profile := Profile{
		Elasticsearch: ESProfile{
			URL:      "http://test:9200",
			APIKey:   "secret-key",
			Username: "user",
			Password: "${ENV_PASS}",
		},
	}

	masked := profile.MaskCredentials()

	// URL should not be masked
	if masked.Elasticsearch.URL != "http://test:9200" {
		t.Errorf("URL = %q, want unchanged", masked.Elasticsearch.URL)
	}

	// Plain text credentials should be masked
	if masked.Elasticsearch.APIKey != "****" {
		t.Errorf("APIKey = %q, want ****", masked.Elasticsearch.APIKey)
	}
	if masked.Elasticsearch.Username != "****" {
		t.Errorf("Username = %q, want ****", masked.Elasticsearch.Username)
	}

	// Env var reference should be shown
	if masked.Elasticsearch.Password != "${ENV_PASS}" {
		t.Errorf("Password = %q, want ${ENV_PASS}", masked.Elasticsearch.Password)
	}
}

func TestProfileConfig_SaveAndLoad(t *testing.T) {
	// Use a temp directory for testing
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cfg := &ProfileConfig{
		CurrentProfile: "test",
		Profiles: map[string]Profile{
			"test": {
				Elasticsearch: ESProfile{
					URL:    "http://test:9200",
					APIKey: "${TEST_KEY}",
				},
				OTLP: OTLPProfile{
					Endpoint: "test:4318",
				},
				Kibana: KibanaProfile{
					URL: "http://test:5601",
				},
			},
		},
	}

	// Save
	err := SaveProfiles(cfg)
	if err != nil {
		t.Fatalf("SaveProfiles error: %v", err)
	}

	// Verify file permissions
	path, _ := GetConfigPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %04o, want 0600", info.Mode().Perm())
	}

	// Load
	loaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles error: %v", err)
	}

	if loaded.CurrentProfile != "test" {
		t.Errorf("CurrentProfile = %q, want %q", loaded.CurrentProfile, "test")
	}

	p, err := loaded.GetProfile("test")
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}
	if p.Elasticsearch.URL != "http://test:9200" {
		t.Errorf("ES URL = %q, want %q", p.Elasticsearch.URL, "http://test:9200")
	}
	if p.Elasticsearch.APIKey != "${TEST_KEY}" {
		t.Errorf("APIKey = %q, want %q", p.Elasticsearch.APIKey, "${TEST_KEY}")
	}
}

func TestLoadProfiles_NonExistent(t *testing.T) {
	// Use a temp directory with no config file
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cfg, err := LoadProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty profiles, got %d", len(cfg.Profiles))
	}
}

func TestGetConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tempDir, "elasticat", "config.yaml")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestProfileConfig_String(t *testing.T) {
	cfg := ProfileConfig{
		CurrentProfile: "test",
		Profiles: map[string]Profile{
			"test": {
				Elasticsearch: ESProfile{
					URL:    "http://test:9200",
					APIKey: "secret",
				},
			},
		},
	}

	str := cfg.String()

	// Should contain the URL (not sensitive)
	if !contains(str, "http://test:9200") {
		t.Error("expected URL in output")
	}

	// Should NOT contain the actual secret
	if contains(str, "secret") {
		t.Error("secret should be masked")
	}

	// Should contain masked value
	if !contains(str, "****") {
		t.Error("expected masked credentials")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
