// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProfileConfig represents the top-level configuration file structure.
// Stored at ~/.config/elasticat/config.yaml
type ProfileConfig struct {
	CurrentProfile string             `yaml:"current-profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty"`
}

// Profile represents a named configuration profile containing
// connection settings for Elasticsearch, OTLP, and Kibana.
type Profile struct {
	Elasticsearch ESProfile     `yaml:"elasticsearch,omitempty"`
	OTLP          OTLPProfile   `yaml:"otlp,omitempty"`
	Kibana        KibanaProfile `yaml:"kibana,omitempty"`
}

// ESProfile holds Elasticsearch connection settings for a profile.
type ESProfile struct {
	URL      string `yaml:"url,omitempty"`
	APIKey   string `yaml:"api-key,omitempty"`   // Supports ${ENV_VAR} syntax
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"` // Supports ${ENV_VAR} syntax
}

// OTLPProfile holds OTLP connection settings for a profile.
type OTLPProfile struct {
	Endpoint string `yaml:"endpoint,omitempty"`
	Insecure *bool  `yaml:"insecure,omitempty"` // Pointer to distinguish unset from false
}

// KibanaProfile holds Kibana connection settings for a profile.
type KibanaProfile struct {
	URL   string `yaml:"url,omitempty"`
	Space string `yaml:"space,omitempty"`
}

// Default configuration directory and file names.
const (
	ConfigDirName  = "elasticat"
	ConfigFileName = "config.yaml"
)

// DefaultKibanaURL is the default Kibana URL when not configured.
const DefaultKibanaURL = "http://localhost:5601"

// GetConfigDir returns the path to the elasticat config directory.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config/elasticat
func GetConfigDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, ConfigDirName), nil
}

// GetConfigPath returns the full path to the config file.
func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// LoadProfiles loads the profile configuration from disk.
// Returns an empty ProfileConfig if the file doesn't exist.
// Warns to stderr if file permissions are insecure.
func LoadProfiles() (*ProfileConfig, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ProfileConfig{Profiles: make(map[string]Profile)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Check file permissions
	checkFilePermissions(path)

	var cfg ProfileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}

	return &cfg, nil
}

// SaveProfiles writes the profile configuration to disk.
// Creates the config directory if it doesn't exist.
// Sets file permissions to 0600 for security.
func SaveProfiles(cfg *ProfileConfig) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// GetProfile returns the named profile, or an error if it doesn't exist.
func (c *ProfileConfig) GetProfile(name string) (Profile, error) {
	if c.Profiles == nil {
		return Profile{}, fmt.Errorf("profile %q not found", name)
	}
	p, ok := c.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found", name)
	}
	return p, nil
}

// SetProfile creates or updates a named profile.
func (c *ProfileConfig) SetProfile(name string, profile Profile) {
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	c.Profiles[name] = profile
}

// DeleteProfile removes a named profile.
// Returns an error if the profile doesn't exist.
func (c *ProfileConfig) DeleteProfile(name string) error {
	if c.Profiles == nil {
		return fmt.Errorf("profile %q not found", name)
	}
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(c.Profiles, name)
	// Clear current profile if it was the deleted one
	if c.CurrentProfile == name {
		c.CurrentProfile = ""
	}
	return nil
}

// ListProfiles returns a list of all profile names.
func (c *ProfileConfig) ListProfiles() []string {
	if c.Profiles == nil {
		return nil
	}
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	return names
}

// GetActiveProfile returns the currently active profile.
// If profileFlag is set, uses that. Otherwise uses current-profile from config.
// Returns nil profile and empty name if no profile is active.
func (c *ProfileConfig) GetActiveProfile(profileFlag string) (*Profile, string) {
	name := profileFlag
	if name == "" {
		name = c.CurrentProfile
	}
	if name == "" {
		return nil, ""
	}
	p, err := c.GetProfile(name)
	if err != nil {
		return nil, ""
	}
	return &p, name
}

// envVarPattern matches ${VAR_NAME} patterns
var envVarPattern = regexp.MustCompile(`^\$\{([^}]+)\}$`)

// IsEnvRef returns true if the string is an environment variable reference.
func IsEnvRef(s string) bool {
	return envVarPattern.MatchString(s)
}

// expandEnvVar expands a single ${VAR} reference.
// Returns the expanded value and true if successful.
// Returns empty string and false if the env var is not set.
func expandEnvVar(s string) (string, bool) {
	matches := envVarPattern.FindStringSubmatch(s)
	if len(matches) != 2 {
		return s, true // Not an env var reference, return as-is
	}
	varName := matches[1]
	value, ok := os.LookupEnv(varName)
	return value, ok
}

// Resolve returns a copy of the profile with all ${ENV_VAR} references expanded.
// Returns an error if any referenced environment variable is undefined.
func (p Profile) Resolve() (Profile, error) {
	resolved := p

	// Resolve ES credentials
	if IsEnvRef(p.Elasticsearch.APIKey) {
		val, ok := expandEnvVar(p.Elasticsearch.APIKey)
		if !ok {
			return Profile{}, fmt.Errorf("undefined environment variable in api-key: %s", p.Elasticsearch.APIKey)
		}
		resolved.Elasticsearch.APIKey = val
	}
	if IsEnvRef(p.Elasticsearch.Username) {
		val, ok := expandEnvVar(p.Elasticsearch.Username)
		if !ok {
			return Profile{}, fmt.Errorf("undefined environment variable in username: %s", p.Elasticsearch.Username)
		}
		resolved.Elasticsearch.Username = val
	}
	if IsEnvRef(p.Elasticsearch.Password) {
		val, ok := expandEnvVar(p.Elasticsearch.Password)
		if !ok {
			return Profile{}, fmt.Errorf("undefined environment variable in password: %s", p.Elasticsearch.Password)
		}
		resolved.Elasticsearch.Password = val
	}

	return resolved, nil
}

// HasCredentials returns true if the profile contains any authentication credentials.
func (p Profile) HasCredentials() bool {
	return p.Elasticsearch.APIKey != "" ||
		p.Elasticsearch.Username != "" ||
		p.Elasticsearch.Password != ""
}

// HasPlainTextCredentials returns true if the profile contains credentials
// that are not environment variable references.
func (p Profile) HasPlainTextCredentials() bool {
	if p.Elasticsearch.APIKey != "" && !IsEnvRef(p.Elasticsearch.APIKey) {
		return true
	}
	if p.Elasticsearch.Username != "" && !IsEnvRef(p.Elasticsearch.Username) {
		return true
	}
	if p.Elasticsearch.Password != "" && !IsEnvRef(p.Elasticsearch.Password) {
		return true
	}
	return false
}

// checkFilePermissions warns to stderr if the config file has insecure permissions.
func checkFilePermissions(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	mode := info.Mode().Perm()
	if mode&0077 != 0 { // group or world can read
		fmt.Fprintf(os.Stderr, "Warning: %s has permissions %04o, should be 0600 for security\n", path, mode)
	}
}

// MaskCredentials returns a copy of the profile with credentials masked for display.
// Environment variable references are shown as-is, plain text values are replaced with "****".
func (p Profile) MaskCredentials() Profile {
	masked := p

	if p.Elasticsearch.APIKey != "" {
		if IsEnvRef(p.Elasticsearch.APIKey) {
			masked.Elasticsearch.APIKey = p.Elasticsearch.APIKey
		} else {
			masked.Elasticsearch.APIKey = "****"
		}
	}
	if p.Elasticsearch.Username != "" {
		if IsEnvRef(p.Elasticsearch.Username) {
			masked.Elasticsearch.Username = p.Elasticsearch.Username
		} else {
			masked.Elasticsearch.Username = "****"
		}
	}
	if p.Elasticsearch.Password != "" {
		if IsEnvRef(p.Elasticsearch.Password) {
			masked.Elasticsearch.Password = p.Elasticsearch.Password
		} else {
			masked.Elasticsearch.Password = "****"
		}
	}

	return masked
}

// MaskAllCredentials returns a copy of the config with all profile credentials masked.
func (c ProfileConfig) MaskAllCredentials() ProfileConfig {
	masked := ProfileConfig{
		CurrentProfile: c.CurrentProfile,
		Profiles:       make(map[string]Profile),
	}
	for name, profile := range c.Profiles {
		masked.Profiles[name] = profile.MaskCredentials()
	}
	return masked
}

// String returns a YAML representation of the config with credentials masked.
func (c ProfileConfig) String() string {
	masked := c.MaskAllCredentials()
	data, err := yaml.Marshal(masked)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return strings.TrimSpace(string(data))
}

// PlainTextCredentialWarning returns a warning message if any profiles contain
// plain text credentials.
func PlainTextCredentialWarning() string {
	return "Warning: Storing credentials in plain text. Consider using environment\n" +
		"variable references (e.g., api-key: ${MY_API_KEY}) for better security."
}
