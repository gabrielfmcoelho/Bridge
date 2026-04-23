package grafana

import (
	"encoding/base64"
	"fmt"
)

// DefaultAgentVersion is the pinned Grafana Agent release. Bump intentionally —
// newer majors change the config schema (agent flow), so we lock to a known-good
// classic-mode release here.
const DefaultAgentVersion = "v0.40.3"

// AgentInstallParams is rendered into the install script template below.
// All string fields are embedded into a YAML config that is base64-encoded
// before it reaches the remote shell, so their contents cannot inject shell
// metacharacters.
type AgentInstallParams struct {
	HostLabel           string
	RemoteWriteURL      string
	RemoteWriteUsername string
	RemoteWritePassword string
	AgentVersion        string
}

// configYAMLTemplate is the grafana-agent config file content. The node_exporter
// integration ships built-in so we don't have to install a separate binary.
const configYAMLTemplate = `server:
  log_level: info

metrics:
  global:
    scrape_interval: 30s
    external_labels:
      host: %q
    remote_write:
      - url: %q
%s
  wal_directory: /var/lib/grafana-agent/wal
  configs:
    - name: integrations
      scrape_configs: []

integrations:
  node_exporter:
    enabled: true
    include_exporter_metrics: false
    rootfs_path: /
    sysfs_path: /sys
    procfs_path: /proc
    instance: %q
`

// systemdUnit is a minimal hardened systemd service definition for grafana-agent.
const systemdUnit = `[Unit]
Description=Grafana Agent
Documentation=https://grafana.com/docs/agent/latest/
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=grafana-agent
Group=grafana-agent
ExecStart=/usr/local/bin/grafana-agent -config.file=/etc/grafana-agent/config.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`

// installScriptTemplate is the shell script executed on the remote host via sudo.
// Format args, in order: version, config_b64, unit_b64.
//
// The caller injects this under runSudoCmd, which base64-encodes the whole script
// again before passing to the remote shell — that keeps us safe against any stray
// metacharacters that slipped through our own quoting.
const installScriptTemplate = `#!/bin/sh
set -e

AGENT_VERSION=%q
CONFIG_B64=%q
UNIT_B64=%q

ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  armv7l|armv6l) ARCH=armv6 ;;
  *) echo "grafana-agent: unsupported architecture $ARCH_RAW" >&2; exit 1 ;;
esac

# 1. Pre-reqs (curl + unzip are the only external deps)
if ! command -v curl >/dev/null 2>&1 || ! command -v unzip >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl unzip
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q curl unzip
  elif command -v yum >/dev/null 2>&1; then
    yum install -y -q curl unzip
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache curl unzip
  else
    echo "grafana-agent: no supported package manager found for curl/unzip" >&2
    exit 1
  fi
fi

# 2. Download + install binary
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
cd "$WORKDIR"
URL="https://github.com/grafana/agent/releases/download/${AGENT_VERSION}/grafana-agent-linux-${ARCH}.zip"
echo "grafana-agent: downloading ${URL}"
curl -fsSL "$URL" -o agent.zip
unzip -oq agent.zip
install -m 0755 "grafana-agent-linux-${ARCH}" /usr/local/bin/grafana-agent

# 3. Service user
if ! id grafana-agent >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /bin/false grafana-agent 2>/dev/null || \
    adduser -S -s /sbin/nologin grafana-agent
fi

# 4. Directories
mkdir -p /etc/grafana-agent /var/lib/grafana-agent/wal
chown -R grafana-agent:grafana-agent /var/lib/grafana-agent

# 5. Config + systemd unit (decoded from base64)
printf '%%s' "$CONFIG_B64" | base64 -d > /etc/grafana-agent/config.yaml
chown grafana-agent:grafana-agent /etc/grafana-agent/config.yaml
chmod 640 /etc/grafana-agent/config.yaml

printf '%%s' "$UNIT_B64" | base64 -d > /etc/systemd/system/grafana-agent.service
chmod 644 /etc/systemd/system/grafana-agent.service

# 6. Reload systemd, enable + start
systemctl daemon-reload
systemctl enable grafana-agent
systemctl restart grafana-agent

# 7. Verify
sleep 2
if systemctl is-active --quiet grafana-agent; then
  VERSION_OUT=$(/usr/local/bin/grafana-agent --version 2>&1 | head -n1 || true)
  echo "grafana-agent: OK — $VERSION_OUT"
  exit 0
else
  echo "grafana-agent: FAILED to start" >&2
  systemctl status grafana-agent --no-pager --lines=30 2>&1 || true
  journalctl -u grafana-agent --no-pager -n 30 2>&1 || true
  exit 2
fi
`

// RenderInstallScript produces the exact shell script that will be handed to
// the remote host's root shell. Caller must run it with sudo privileges.
//
// The config YAML is built in Go (with proper quoting via %q), then base64-
// encoded so the remote shell never sees the raw YAML — shell metacharacters
// in the remote_write password cannot corrupt the file.
func RenderInstallScript(p AgentInstallParams) string {
	if p.AgentVersion == "" {
		p.AgentVersion = DefaultAgentVersion
	}

	// Build the basic_auth block conditionally — some internal Prometheus setups
	// accept anonymous writes, in which case we just omit the block.
	var basicAuth string
	if p.RemoteWriteUsername != "" || p.RemoteWritePassword != "" {
		basicAuth = fmt.Sprintf("        basic_auth:\n          username: %q\n          password: %q\n",
			p.RemoteWriteUsername, p.RemoteWritePassword)
	}

	configYAML := fmt.Sprintf(configYAMLTemplate,
		p.HostLabel,
		p.RemoteWriteURL,
		basicAuth,
		p.HostLabel,
	)
	configB64 := base64.StdEncoding.EncodeToString([]byte(configYAML))
	unitB64 := base64.StdEncoding.EncodeToString([]byte(systemdUnit))

	return fmt.Sprintf(installScriptTemplate, p.AgentVersion, configB64, unitB64)
}
