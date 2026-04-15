package sshconfig

import (
	"os"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// knownKeys are the directives we map to named HostEntry fields.
var knownKeys = map[string]bool{
	"hostname":       true,
	"user":           true,
	"port":           true,
	"identityfile":   true,
	"identitiesonly": true,
	"proxyjump":      true,
	"forwardagent":   true,
}

// ParseFile reads an SSH config file and returns its Host entries.
// Wildcard hosts (e.g. Host *) are skipped.
func ParseFile(path string) ([]HostEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, err
	}

	var entries []HostEntry
	for _, host := range cfg.Hosts {
		patterns := host.Patterns
		if len(patterns) == 0 {
			continue
		}
		// Skip wildcard-only hosts
		name := patterns[0].String()
		if name == "*" || name == "" {
			continue
		}

		entry := HostEntry{Host: name}
		for _, node := range host.Nodes {
			kv, ok := node.(*ssh_config.KV)
			if !ok {
				continue
			}
			key := strings.ToLower(kv.Key)
			val := kv.Value
			switch key {
			case "hostname":
				entry.HostName = val
			case "user":
				entry.User = val
			case "port":
				entry.Port = val
			case "identityfile":
				entry.IdentityFile = val
			case "identitiesonly":
				entry.IdentitiesOnly = val
			case "proxyjump":
				entry.ProxyJump = val
			case "forwardagent":
				entry.ForwardAgent = val
			default:
				entry.Extra = append(entry.Extra, KVPair{Key: kv.Key, Value: val})
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
