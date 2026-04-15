package sshconfig

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RenderConfig generates SSH config text from a list of host entries.
func RenderConfig(entries []HostEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Host " + e.Host + "\n")
		if e.HostName != "" {
			b.WriteString("    HostName " + e.HostName + "\n")
		}
		if e.User != "" {
			b.WriteString("    User " + e.User + "\n")
		}
		if e.Port != "" && e.Port != "22" {
			b.WriteString("    Port " + e.Port + "\n")
		}
		if e.IdentityFile != "" {
			b.WriteString("    IdentityFile " + e.IdentityFile + "\n")
		}
		if e.IdentitiesOnly != "" {
			b.WriteString("    IdentitiesOnly " + e.IdentitiesOnly + "\n")
		}
		if e.ProxyJump != "" {
			b.WriteString("    ProxyJump " + e.ProxyJump + "\n")
		}
		if e.ForwardAgent != "" {
			b.WriteString("    ForwardAgent " + e.ForwardAgent + "\n")
		}
		for _, kv := range e.Extra {
			b.WriteString("    " + kv.Key + " " + kv.Value + "\n")
		}
	}
	return b.String()
}

// WriteFile writes the entries to the SSH config file with backup and atomic rename.
func WriteFile(path string, entries []HostEntry) error {
	// Get existing permissions (default 0600)
	perm := fs.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	if err := backupFile(path); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	content := RenderConfig(entries)

	// Write to temp file in same directory, then atomic rename
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ssh_config_tmp_*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// backupFile creates a timestamped backup of the given file.
func backupFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // nothing to back up
	}

	ts := time.Now().Format("20060102-150405")
	backupPath := path + ".backup." + ts

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Preserve original permissions
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, data, info.Mode().Perm())
}
