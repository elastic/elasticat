// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDockerDir_UsesUserDataDir(t *testing.T) {
	tmp := t.TempDir()

	cwd := filepath.Join(tmp, "cwd")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	exePath := filepath.Join(tmp, "bin", "elasticat")
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("mkdir exe dir: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("write exe: %v", err)
	}

	dataDir := filepath.Join(tmp, "elasticat")
	dataDockerDir := filepath.Join(dataDir, "docker")
	if err := os.MkdirAll(dataDockerDir, 0o755); err != nil {
		t.Fatalf("mkdir data docker dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDockerDir, "docker-compose.yml"), []byte("services: {}"), 0o644); err != nil {
		t.Fatalf("write docker-compose.yml: %v", err)
	}

	got, err := findDockerDirWith(dockerDirSearch{
		getwd:      func() (string, error) { return cwd, nil },
		executable: func() (string, error) { return exePath, nil },
		stat:       os.Stat,
		elasticatDataDirFn: func() (string, error) {
			return dataDir, nil
		},
	})
	if err != nil {
		t.Fatalf("findDockerDirWith: %v", err)
	}
	if got != dataDockerDir {
		t.Fatalf("expected %q, got %q", dataDockerDir, got)
	}
}

func TestFindDockerDir_PrefersRepoDockerDirOverUserDataDir(t *testing.T) {
	tmp := t.TempDir()

	cwd := filepath.Join(tmp, "cwd")
	repoDockerDir := filepath.Join(cwd, "docker")
	if err := os.MkdirAll(repoDockerDir, 0o755); err != nil {
		t.Fatalf("mkdir repo docker dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDockerDir, "docker-compose.yml"), []byte("services: {}"), 0o644); err != nil {
		t.Fatalf("write repo docker-compose.yml: %v", err)
	}

	exePath := filepath.Join(tmp, "bin", "elasticat")
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("mkdir exe dir: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("write exe: %v", err)
	}

	dataDir := filepath.Join(tmp, "elasticat")
	dataDockerDir := filepath.Join(dataDir, "docker")
	if err := os.MkdirAll(dataDockerDir, 0o755); err != nil {
		t.Fatalf("mkdir data docker dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDockerDir, "docker-compose.yml"), []byte("services: {}"), 0o644); err != nil {
		t.Fatalf("write data docker-compose.yml: %v", err)
	}

	got, err := findDockerDirWith(dockerDirSearch{
		getwd:      func() (string, error) { return cwd, nil },
		executable: func() (string, error) { return exePath, nil },
		stat:       os.Stat,
		elasticatDataDirFn: func() (string, error) {
			return dataDir, nil
		},
	})
	if err != nil {
		t.Fatalf("findDockerDirWith: %v", err)
	}
	if got != repoDockerDir {
		t.Fatalf("expected %q, got %q", repoDockerDir, got)
	}
}
