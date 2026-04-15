package sshkeys

import (
	"os"
	"path/filepath"
	"strings"
)

// KeyInfo holds the path to a private SSH key.
type KeyInfo struct {
	PrivatePath string // tilde path, e.g. ~/.ssh/id_ed25519
}

// ListKeys scans ~/.ssh/ for private keys that have a matching .pub file.
func ListKeys() ([]KeyInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshDir := filepath.Join(home, ".ssh")

	entries, err := os.ReadDir(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	skip := map[string]bool{
		"config": true, "known_hosts": true, "known_hosts.old": true,
		"authorized_keys": true,
	}

	var keys []KeyInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if strings.HasSuffix(name, ".pub") || strings.HasSuffix(name, ".old") || strings.HasSuffix(name, ".bak") {
			continue
		}
		if skip[name] {
			continue
		}

		// Check that a .pub counterpart exists
		pubPath := filepath.Join(sshDir, name+".pub")
		if _, err := os.Stat(pubPath); err != nil {
			continue
		}

		keys = append(keys, KeyInfo{
			PrivatePath: "~/.ssh/" + name,
		})
	}
	return keys, nil
}
