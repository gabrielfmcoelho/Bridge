package sshconfig

// KVPair holds an SSH config directive not covered by HostEntry's named fields.
type KVPair struct {
	Key   string
	Value string
}

// HostEntry represents a single Host block in ~/.ssh/config.
type HostEntry struct {
	Host           string
	HostName       string
	User           string
	Port           string
	IdentityFile   string
	IdentitiesOnly string
	ProxyJump      string
	ForwardAgent   string
	Extra          []KVPair
}
