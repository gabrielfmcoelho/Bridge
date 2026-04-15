package sshtest

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const fixDevNullCmd = `sudo -n sh -c 'if [ ! -e /dev/null ] || [ ! -c /dev/null ] || [ "$(stat -c "%a" /dev/null 2>/dev/null)" != "666" ] || [ "$(stat -c "%u:%g" /dev/null 2>/dev/null)" != "0:0" ]; then rm -f /dev/null && mknod -m 666 /dev/null c 1 3 && chown root:root /dev/null; fi'`

// VMInfo contains captured info from a remote machine.
type VMInfo struct {
	CPU            string   `json:"cpu"`
	CPUUsage       string   `json:"cpu_usage"`
	RAM            string   `json:"ram"`
	RAMUsed        string   `json:"ram_used"`
	RAMPercent     string   `json:"ram_percent"`
	Storage        string   `json:"storage"`
	StorageUsed    string   `json:"storage_used"`
	DiskPercent    string   `json:"disk_percent"`
	OS             string   `json:"os"`
	Kernel         string   `json:"kernel"`
	Uptime         string   `json:"uptime"`
	LastLogins     []string `json:"last_logins"`
	Services       []string `json:"services"`
	ServiceDetails []string `json:"service_details"`
	Containers     []string `json:"containers"`
	ContainerStats []string `json:"container_stats"`
	Ports          []string `json:"ports"`
	PublicIP       string   `json:"public_ip"`
	Hostname       string   `json:"hostname_remote"`
	Users          string   `json:"logged_users"`
	LoadAvg        string   `json:"load_avg"`
	SwapTotal      string   `json:"swap_total"`
	SwapUsed       string   `json:"swap_used"`
	Warnings       []string          `json:"warnings,omitempty"`
	ProcessDetails []ProcessDetail   `json:"process_details,omitempty"`
	SSHKeys        []SSHKeyInfo      `json:"ssh_keys,omitempty"`
}

// SSHKeyInfo holds info about an SSH key found on the remote host during scan.
type SSHKeyInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Source      string `json:"source"` // "authorized_keys" or "private_key"
	Managed     bool   `json:"managed,omitempty"`
	ManagedName string `json:"managed_name,omitempty"`
}

// ProcessDetail holds detailed info about an unidentified/interesting process.
type ProcessDetail struct {
	PID     string `json:"pid"`
	User    string `json:"user"`
	CPU     string `json:"cpu"`
	MEM     string `json:"mem"`
	Command string `json:"command"`
	CWD     string `json:"cwd"`
	StartedVia string `json:"started_via"` // systemd, cron, manual, unknown
	Venv    string `json:"venv,omitempty"`
	Ports   string `json:"ports,omitempty"`
}

// TestWithPassword tests SSH connectivity using password authentication.
func TestWithPassword(hostname, port, user, password string) error {
	if port == "" {
		port = "22"
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(hostname, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("SSH connect to %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	if _, err := session.CombinedOutput("echo OK"); err != nil {
		return fmt.Errorf("run test command: %w", err)
	}

	return nil
}

// TestWithPasswordCapture tests SSH + captures VM info.
func TestWithPasswordCapture(hostname, port, user, password string) (*VMInfo, error) {
	client, err := dialWithPassword(hostname, port, user, password)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return captureVMInfo(client)
}

// TestWithKeyCapture tests SSH + captures VM info.
func TestWithKeyCapture(hostname, port, user, keyPath string) (*VMInfo, error) {
	client, err := dialWithKey(hostname, port, user, keyPath)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return captureVMInfo(client)
}

// FixDevNullWithPassword attempts to repair /dev/null permissions on remote host.
func FixDevNullWithPassword(hostname, port, user, password string) (string, error) {
	client, err := dialWithPassword(hostname, port, user, password)
	if err != nil {
		return "", fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return fixDevNull(client)
}

// FixDevNullWithKey attempts to repair /dev/null permissions on remote host.
func FixDevNullWithKey(hostname, port, user, keyPath string) (string, error) {
	client, err := dialWithKey(hostname, port, user, keyPath)
	if err != nil {
		return "", fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return fixDevNull(client)
}

func runCmd(client *ssh.Client, cmd string) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cleanCommandOutput(string(out), nil))
}

func runCmdWithWarnings(client *ssh.Client, cmd string, warnings map[string]bool) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cleanCommandOutput(string(out), warnings))
}

func runCmdRaw(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	out, execErr := session.CombinedOutput(cmd)
	cleaned := strings.TrimSpace(cleanCommandOutput(string(out), nil))
	if execErr != nil {
		if cleaned == "" {
			return "", execErr
		}
		return cleaned, fmt.Errorf("%s: %w", cleaned, execErr)
	}
	return cleaned, nil
}

func dialWithPassword(hostname, port, user, password string) (*ssh.Client, error) {
	if port == "" {
		port = "22"
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(hostname, port), config)
}

func dialWithKey(hostname, port, user, keyPath string) (*ssh.Client, error) {
	if port == "" {
		port = "22"
	}
	if strings.HasPrefix(keyPath, "~") {
		home, _ := os.UserHomeDir()
		keyPath = home + keyPath[1:]
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(hostname, port), config)
}

func fixDevNull(client *ssh.Client) (string, error) {
	before := runCmd(client, "ls -l /dev/null 2>&1")
	if before == "" {
		before = runCmd(client, "stat /dev/null 2>&1")
	}

	if _, err := runCmdRaw(client, fixDevNullCmd); err != nil {
		return before, fmt.Errorf("automatic fix requires sudo permission (NOPASSWD) on remote host: %w", err)
	}

	after := runCmd(client, "ls -l /dev/null 2>&1")
	verify := runCmd(client, `test -c /dev/null && [ "$(stat -c "%a" /dev/null 2>/dev/null)" = "666" ] && [ "$(stat -c "%u:%g" /dev/null 2>/dev/null)" = "0:0" ] && echo OK || echo FAIL`)
	if verify != "OK" {
		if after == "" {
			after = "validation failed after fix attempt"
		}
		return strings.TrimSpace(before + "\n" + after), fmt.Errorf("fix command executed but /dev/null is still not in expected state")
	}

	if after == "" {
		after = "-rw-rw-rw- root root /dev/null"
	}
	return strings.TrimSpace(before + "\n" + after), nil
}

// SetupSudoNopasswdWithPassword connects via password and configures NOPASSWD.
func SetupSudoNopasswdWithPassword(hostname, port, user, password string) (string, error) {
	client, err := dialWithPassword(hostname, port, user, password)
	if err != nil {
		return "", fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return setupSudoNopasswd(client, user, password)
}

// SetupSudoNopasswdWithKey connects via key and uses the stored password for sudo -S.
func SetupSudoNopasswdWithKey(hostname, port, user, keyPath, password string) (string, error) {
	client, err := dialWithKey(hostname, port, user, keyPath)
	if err != nil {
		return "", fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return setupSudoNopasswd(client, user, password)
}

func setupSudoNopasswd(client *ssh.Client, user, password string) (string, error) {
	// Use -S to pipe password via stdin (no tty required)
	file := fmt.Sprintf("/etc/sudoers.d/%s-nopasswd", user)
	rule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", user)

	// Check if already configured
	check := fmt.Sprintf(`sudo -n true 2>/dev/null && echo ALREADY_OK || echo NEEDS_SETUP`)
	checkOut := runCmd(client, check)
	if strings.TrimSpace(checkOut) == "ALREADY_OK" {
		return "NOPASSWD already configured for " + user, nil
	}

	// Write sudoers drop-in using echo password | sudo -S
	cmd := fmt.Sprintf(
		`echo '%s' | sudo -S sh -c 'echo "%s" > %s && chmod 440 %s && visudo -cf %s'`,
		password, rule, file, file, file,
	)
	output, err := runCmdRaw(client, cmd)
	if err != nil {
		return output, fmt.Errorf("failed to configure sudoers: %w", err)
	}

	// Verify it works
	verify := runCmd(client, `sudo -n true 2>/dev/null && echo OK || echo FAIL`)
	if strings.TrimSpace(verify) == "OK" {
		return fmt.Sprintf("Created %s — NOPASSWD enabled for %s", file, user), nil
	}

	// Drop-in didn't take effect. Append the rule directly as the last line
	// of /etc/sudoers — last matching rule always wins regardless of includes.
	directCmd := fmt.Sprintf(
		`echo '%s' | sudo -S sh -c '`+
			`grep -qxF "%s" /etc/sudoers 2>/dev/null || `+
			`(cp /etc/sudoers /etc/sudoers.bak && echo "%s" >> /etc/sudoers && visudo -cf /etc/sudoers)'`,
		password, rule, rule,
	)
	directOut, directErr := runCmdRaw(client, directCmd)
	if directErr != nil {
		return output + "\n" + directOut, fmt.Errorf("failed to append rule to /etc/sudoers: %w", directErr)
	}

	verify2 := runCmd(client, `sudo -n true 2>/dev/null && echo OK || echo FAIL`)
	if strings.TrimSpace(verify2) != "OK" {
		return output + "\n" + directOut + "\nNOPASSWD still not effective",
			fmt.Errorf("rule written to both %s and /etc/sudoers but sudo -n still fails — check PAM config", file)
	}

	return fmt.Sprintf("Appended NOPASSWD rule to /etc/sudoers — enabled for %s", user), nil
}

// CreateRemoteUserWithPassword connects via password and creates a user on the remote host
// with an authorized public key and passwordless sudo.
func CreateRemoteUserWithPassword(hostname, port, loginUser, password, newUser, pubKey string) (string, error) {
	client, err := dialWithPassword(hostname, port, loginUser, password)
	if err != nil {
		return "", fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return createRemoteUser(client, password, newUser, pubKey)
}

// CreateRemoteUserWithKey connects via key and creates a user on the remote host
// with an authorized public key and passwordless sudo.
func CreateRemoteUserWithKey(hostname, port, loginUser, keyPath, password, newUser, pubKey string) (string, error) {
	client, err := dialWithKey(hostname, port, loginUser, keyPath)
	if err != nil {
		return "", fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return createRemoteUser(client, password, newUser, pubKey)
}

func createRemoteUser(client *ssh.Client, sudoPassword, newUser, pubKey string) (string, error) {
	var out strings.Builder

	// 1. Check if user already exists
	check := fmt.Sprintf(`id %s 2>/dev/null && echo EXISTS || echo MISSING`, newUser)
	if strings.TrimSpace(runCmd(client, check)) == "EXISTS" {
		out.WriteString(fmt.Sprintf("User %s already exists — skipping creation.\n", newUser))
	} else {
		// Create user with home dir, no password login
		cmd := fmt.Sprintf(
			`echo '%s' | sudo -S useradd -m -s /bin/bash %s 2>&1`,
			sudoPassword, newUser,
		)
		res, err := runCmdRaw(client, cmd)
		if err != nil {
			return res, fmt.Errorf("failed to create user %s: %w", newUser, err)
		}
		out.WriteString(fmt.Sprintf("Created user %s.\n", newUser))
	}

	// 2. Set up authorized_keys
	sshDir := fmt.Sprintf("/home/%s/.ssh", newUser)
	authKeys := fmt.Sprintf("%s/authorized_keys", sshDir)
	setupCmd := fmt.Sprintf(
		`echo '%s' | sudo -S sh -c '`+
			`mkdir -p %s && `+
			`grep -qF %q %s 2>/dev/null || echo %q >> %s && `+
			`chown -R %s:%s %s && `+
			`chmod 700 %s && chmod 600 %s'`,
		sudoPassword,
		sshDir,
		pubKey, authKeys, pubKey, authKeys,
		newUser, newUser, sshDir,
		sshDir, authKeys,
	)
	res, err := runCmdRaw(client, setupCmd)
	if err != nil {
		return out.String() + res, fmt.Errorf("failed to setup authorized_keys: %w", err)
	}
	out.WriteString(fmt.Sprintf("Authorized key added to %s.\n", authKeys))

	// 3. Configure passwordless sudo
	sudoFile := fmt.Sprintf("/etc/sudoers.d/%s-nopasswd", newUser)
	sudoRule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", newUser)
	sudoCmd := fmt.Sprintf(
		`echo '%s' | sudo -S sh -c 'echo "%s" > %s && chmod 440 %s && visudo -cf %s' 2>&1`,
		sudoPassword, sudoRule, sudoFile, sudoFile, sudoFile,
	)
	res, err = runCmdRaw(client, sudoCmd)
	if err != nil {
		return out.String() + res, fmt.Errorf("failed to configure sudoers: %w", err)
	}
	out.WriteString(fmt.Sprintf("NOPASSWD rule created at %s.\n", sudoFile))

	return out.String(), nil
}

// DockerStatus holds info about Docker installation on the remote host.
type DockerStatus struct {
	Installed        bool   `json:"installed"`
	DockerVersion    string `json:"docker_version,omitempty"`
	ComposeVersion   string `json:"compose_version,omitempty"`
	UserInGroup      bool   `json:"user_in_group"`
	NeedsSudo        bool   `json:"needs_sudo"`
	GroupFixApplied  bool   `json:"group_fix_applied,omitempty"`
	Message          string `json:"message"`
}

// CheckAndFixDockerGroupWithPassword connects via password.
func CheckAndFixDockerGroupWithPassword(hostname, port, user, password string, fix bool) (*DockerStatus, error) {
	client, err := dialWithPassword(hostname, port, user, password)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return checkAndFixDockerGroup(client, user, password, fix)
}

// CheckAndFixDockerGroupWithKey connects via key.
func CheckAndFixDockerGroupWithKey(hostname, port, user, keyPath, password string, fix bool) (*DockerStatus, error) {
	client, err := dialWithKey(hostname, port, user, keyPath)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return checkAndFixDockerGroup(client, user, password, fix)
}

func checkAndFixDockerGroup(client *ssh.Client, user, password string, fix bool) (*DockerStatus, error) {
	// Batch: docker version, compose version, group membership, sudo check
	raw := runCmd(client, `echo "$(docker --version 2>/dev/null)"`+
		`; echo "---SEP---"`+
		`; echo "$(docker compose version 2>/dev/null || docker-compose --version 2>/dev/null)"` +
		`; echo "---SEP---"`+
		`; id -nG 2>/dev/null`+
		`; echo "---SEP---"`+
		`; docker ps >/dev/null 2>&1 && echo OK || echo NEEDS_SUDO`)

	parts := strings.Split(raw, "---SEP---")
	field := func(i int) string {
		if i < len(parts) {
			return strings.TrimSpace(parts[i])
		}
		return ""
	}

	status := &DockerStatus{}

	status.DockerVersion = field(0)
	status.ComposeVersion = field(1)
	groups := field(2)
	dockerCheck := field(3)

	if status.DockerVersion == "" {
		status.Message = "Docker is not installed on this host."
		return status, nil
	}
	status.Installed = true

	// Check if user is in docker group
	for _, g := range strings.Fields(groups) {
		if g == "docker" {
			status.UserInGroup = true
			break
		}
	}

	status.NeedsSudo = dockerCheck == "NEEDS_SUDO"

	if !status.NeedsSudo {
		status.Message = fmt.Sprintf("Docker OK. User %s can run docker without sudo.", user)
		return status, nil
	}

	if !fix {
		if status.UserInGroup {
			status.Message = fmt.Sprintf("User %s is in docker group but still needs sudo. Try logging out and back in, or run: newgrp docker", user)
		} else {
			status.Message = fmt.Sprintf("User %s is not in the docker group. Run this operation with fix enabled to add them.", user)
		}
		return status, nil
	}

	// Fix: add user to docker group
	if !status.UserInGroup {
		var addCmd string
		if password != "" {
			addCmd = fmt.Sprintf(`echo '%s' | sudo -S usermod -aG docker %s`, password, user)
		} else {
			addCmd = fmt.Sprintf(`sudo -n usermod -aG docker %s`, user)
		}
		_, err := runCmdRaw(client, addCmd)
		if err != nil {
			status.Message = fmt.Sprintf("Failed to add %s to docker group: %s", user, err)
			return status, nil
		}
		status.UserInGroup = true
		status.GroupFixApplied = true
	}

	status.Message = fmt.Sprintf("User %s added to docker group. A new SSH session is required for the change to take effect.", user)
	return status, nil
}

func captureProcessDetails(client *ssh.Client) []ProcessDetail {
	// Single SSH command that collects everything for all matching processes.
	// Output format per process: PID|USER|%CPU|%MEM|CMD||CWD|EXE|PPID_COMM|PORTS
	// Processes separated by newlines, "||" separates ps fields from /proc fields.
	raw := runCmd(client, `ps aux 2>/dev/null | awk '$11 ~ /(python|node|java|ruby|php|gunicorn|uvicorn|celery|pm2)/ && $11 !~ /grep|ps|awk/ {printf "%s|%s|%s|%s|", $2, $1, $3, $4; for(i=11;i<=NF;i++) printf "%s ", $i; print ""}' | head -20`)
	if raw == "" {
		return nil
	}

	// Collect PIDs
	var pids []string
	var psLines []string
	for _, line := range splitLines(raw) {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) >= 5 && strings.TrimSpace(parts[4]) != "" {
			pids = append(pids, parts[0])
			psLines = append(psLines, line)
		}
	}
	if len(pids) == 0 {
		return nil
	}

	// Single batch command to get cwd, exe, parent comm, and ports for all PIDs
	batchScript := `for pid in ` + strings.Join(pids, " ") + `; do
		cwd=$(readlink -f /proc/$pid/cwd 2>/dev/null)
		exe=$(readlink -f /proc/$pid/exe 2>/dev/null)
		ppid=$(awk '/^PPid:/{print $2}' /proc/$pid/status 2>/dev/null)
		pcomm=$(cat /proc/$ppid/comm 2>/dev/null)
		ports=$(ss -tlnp 2>/dev/null | grep "pid=$pid," | awk '{split($4,a,":"); print a[length(a)]}' | sort -nu | tr '\n' ',' | sed 's/,$//')
		echo "$pid||$cwd||$exe||$pcomm||$ports"
	done`
	batchRaw := runCmd(client, batchScript)

	// Parse batch results into a map by PID
	procInfo := map[string][4]string{} // cwd, exe, parentComm, ports
	for _, line := range splitLines(batchRaw) {
		parts := strings.Split(line, "||")
		if len(parts) >= 5 {
			procInfo[parts[0]] = [4]string{parts[1], parts[2], parts[3], parts[4]}
		}
	}

	var details []ProcessDetail
	for i, line := range psLines {
		parts := strings.SplitN(line, "|", 5)
		pid := pids[i]
		cmd := strings.TrimSpace(parts[4])
		info := procInfo[pid]
		cwd, exe, parentComm, ports := info[0], info[1], info[2], info[3]

		// Detect venv from exe path
		venv := ""
		for _, marker := range []string{"/venv/", "/.venv/", "/virtualenv/", "/env/"} {
			if idx := strings.Index(exe, marker); idx >= 0 {
				venv = exe[:idx+len(marker)-1]
				break
			}
		}

		// Classify start method from parent process
		startedVia := "unknown"
		switch parentComm {
		case "systemd", "init":
			startedVia = "systemd"
		case "cron", "crond", "anacron":
			startedVia = "cron"
		case "sshd", "bash", "sh", "zsh", "tmux: server", "screen":
			startedVia = "manual"
		case "supervisord":
			startedVia = "supervisor"
		case "containerd-shim", "docker":
			startedVia = "docker"
		default:
			if parentComm != "" {
				startedVia = "parent:" + parentComm
			}
		}

		details = append(details, ProcessDetail{
			PID:        pid,
			User:       parts[1],
			CPU:        parts[2] + "%",
			MEM:        parts[3] + "%",
			Command:    cmd,
			CWD:        cwd,
			StartedVia: startedVia,
			Venv:       venv,
			Ports:      ports,
		})
	}
	return details
}

// RemoteKeyInfo holds information about an SSH key found on the remote host.
type RemoteKeyInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Source      string `json:"source"` // "authorized_keys" or "private_key"
}

// ListRemoteKeysWithPassword connects via password and lists SSH keys on the remote host.
func ListRemoteKeysWithPassword(hostname, port, user, password string) ([]RemoteKeyInfo, error) {
	client, err := dialWithPassword(hostname, port, user, password)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return listRemoteKeys(client)
}

// ListRemoteKeysWithKey connects via key and lists SSH keys on the remote host.
func ListRemoteKeysWithKey(hostname, port, user, keyPath string) ([]RemoteKeyInfo, error) {
	client, err := dialWithKey(hostname, port, user, keyPath)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	return listRemoteKeys(client)
}

func listRemoteKeys(client *ssh.Client) ([]RemoteKeyInfo, error) {
	var keys []RemoteKeyInfo

	// List authorized_keys entries with fingerprints
	authKeys := runCmd(client, `while IFS= read -r line; do
		echo "$line" | ssh-keygen -lf - 2>/dev/null
	done < ~/.ssh/authorized_keys 2>/dev/null`)
	for _, line := range splitLines(authKeys) {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			name := ""
			if len(parts) >= 4 {
				name = parts[len(parts)-1]
				// Remove parenthesized key type at end if present
				if strings.HasPrefix(name, "(") {
					name = ""
					if len(parts) >= 5 {
						name = parts[len(parts)-2]
					}
				}
			}
			keys = append(keys, RemoteKeyInfo{
				Name:        name,
				Type:        strings.Trim(parts[len(parts)-1], "()"),
				Fingerprint: parts[1],
				Source:      "authorized_keys",
			})
		}
	}

	// List private key files in ~/.ssh/
	privKeys := runCmd(client, `for f in ~/.ssh/id_*; do
		[ -f "$f" ] && [ ! "${f%.pub}" != "$f" ] || continue
		[ -f "$f.pub" ] || continue
		fp=$(ssh-keygen -lf "$f.pub" 2>/dev/null)
		[ -n "$fp" ] && echo "$(basename $f)|$fp"
	done 2>/dev/null`)
	for _, line := range splitLines(privKeys) {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		fpParts := strings.Fields(parts[1])
		if len(fpParts) < 3 {
			continue
		}
		keys = append(keys, RemoteKeyInfo{
			Name:        parts[0],
			Type:        strings.Trim(fpParts[len(fpParts)-1], "()"),
			Fingerprint: fpParts[1],
			Source:      "private_key",
		})
	}

	return keys, nil
}

// captureDocker tries a docker command without sudo first. If the output
// contains "permission denied" or "connect: ...", it retries with sudo -n
// and sets a warning flag so the scan records the issue.
func captureDocker(client *ssh.Client, cmd, sudoCmd string, warnings map[string]bool) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()

	out, execErr := session.CombinedOutput(cmd)
	raw := strings.TrimSpace(string(out))
	lower := strings.ToLower(raw)

	// Docker not installed or not in PATH — skip silently
	if strings.Contains(lower, "not found") || strings.Contains(lower, "no such file") {
		return ""
	}

	// Permission / daemon issues — try with sudo
	needsSudo := execErr != nil &&
		(strings.Contains(lower, "permission denied") ||
			strings.Contains(lower, "connect:") ||
			strings.Contains(lower, "cannot connect to the docker daemon"))

	if needsSudo {
		// Try with sudo — use runCmdRaw to distinguish "succeeded with empty output"
		// (0 containers) from "sudo also failed"
		sudoOut, sudoErr := runCmdRaw(client, sudoCmd)
		if sudoErr == nil {
			// sudo worked (even if output is empty = 0 containers)
			warnings["docker_requires_sudo"] = true
			return sudoOut
		}
		warnings["docker_permission_denied"] = true
		return ""
	}

	// Command failed for other reasons (daemon stopped, etc.) — skip silently
	if execErr != nil {
		return ""
	}

	return cleanCommandOutput(raw, warnings)
}

func cleanCommandOutput(raw string, warnings map[string]bool) string {
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "/dev/null") && strings.Contains(lower, "permission denied") {
			if warnings != nil {
				warnings["remote_dev_null_permission"] = true
			}
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(filtered, "\n")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func captureVMInfo(client *ssh.Client) (*VMInfo, error) {
	warnFlags := map[string]bool{}

	// ── Session 0: measure CPU usage BEFORE the heavy scan commands ──
	// Uses /proc/stat delta over 1 second for an accurate, uncontaminated reading.
	// Falls back to top -bn2 (second iteration) if /proc/stat is unavailable.
	cpuUsage := strings.TrimSpace(runCmd(client,
		`if [ -f /proc/stat ]; then `+
			`c1=$(awk '/^cpu /{print $2+$3+$4, $2+$3+$4+$5}' /proc/stat); `+
			`sleep 1; `+
			`c2=$(awk '/^cpu /{print $2+$3+$4, $2+$3+$4+$5}' /proc/stat); `+
			`echo "$c1 $c2" | awk '{printf "%.0f%%", ($3-$1)/($4-$2)*100}'; `+
			`else `+
			`top -bn2 2>/dev/null | grep '^%Cpu' | tail -1 | awk '{printf "%.0f%%", 100-$8}'; `+
			`fi`))

	// ── Session 1: batch all hardware/system metrics in one command ──
	// Each field separated by a unique delimiter "---FIELD---"
	const delim = "---FIELD---"
	batchCmd := `echo "$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null)"` +
		`; echo '` + delim + `'` +
		`; free -h 2>/dev/null | awk '/^Mem:/{print $2}'` +
		`; echo '` + delim + `'` +
		`; free -h 2>/dev/null | awk '/^Mem:/{print $3}'` +
		`; echo '` + delim + `'` +
		"; free 2>/dev/null | awk '/^Mem:/{printf \"%.0f%%\", $3/$2*100}'" +
		`; echo '` + delim + `'` +
		`; df -h / 2>/dev/null | awk 'NR==2{print $2}'` +
		`; echo '` + delim + `'` +
		`; df -h / 2>/dev/null | awk 'NR==2{print $3}'` +
		`; echo '` + delim + `'` +
		`; df -h / 2>/dev/null | awk 'NR==2{print $5}'` +
		`; echo '` + delim + `'` +
		`; free -h 2>/dev/null | awk '/^Swap:/{print $2}'` +
		`; echo '` + delim + `'` +
		`; free -h 2>/dev/null | awk '/^Swap:/{print $3}'` +
		`; echo '` + delim + `'` +
		`; cat /etc/os-release 2>/dev/null | grep '^PRETTY_NAME=' | cut -d'"' -f2 || uname -s` +
		`; echo '` + delim + `'` +
		`; uname -r` +
		`; echo '` + delim + `'` +
		`; uptime -p 2>/dev/null || uptime` +
		`; echo '` + delim + `'` +
		`; hostname -f 2>/dev/null || hostname` +
		`; echo '` + delim + `'` +
		`; cat /proc/loadavg 2>/dev/null | awk '{print $1, $2, $3}'` +
		`; echo '` + delim + `'` +
		`; who 2>/dev/null | wc -l`

	batchRaw := runCmdWithWarnings(client, batchCmd, warnFlags)
	fields := strings.Split(batchRaw, delim)
	field := func(i int) string {
		if i < len(fields) {
			return strings.TrimSpace(fields[i])
		}
		return ""
	}

	info := &VMInfo{
		CPU:         field(0) + " vCPU",
		CPUUsage:    cpuUsage,
		RAM:         field(1),
		RAMUsed:     field(2),
		RAMPercent:  field(3),
		Storage:     field(4),
		StorageUsed: field(5),
		DiskPercent: field(6),
		SwapTotal:   field(7),
		SwapUsed:    field(8),
		OS:          field(9),
		Kernel:      field(10),
		Uptime:      field(11),
		Hostname:    field(12),
		LoadAvg:     field(13),
		Users:       field(14),
	}

	// ── Session 2: public IP (separate because curl has a timeout) ──
	info.PublicIP = runCmdWithWarnings(client, "curl -s --max-time 3 ifconfig.me 2>/dev/null || curl -s --max-time 3 icanhazip.com 2>/dev/null", warnFlags)

	// ── Session 3: logins + services + ports (multi-line outputs, batched) ──
	const delim2 = "---SECTION---"
	listCmd := `last -5 -w 2>/dev/null | head -5` +
		`; echo '` + delim2 + `'` +
		`; ps aux 2>/dev/null | grep -E '(nginx|apache|httpd|php-fpm|node|python|java|postgres|mysql|mariadb|redis|mongo|docker|containerd|caddy|haproxy|traefik|pm2|gunicorn|uvicorn|supervisord)' | grep -v grep | awk '{print $11}' | sort -u | head -15` +
		`; echo '` + delim2 + `'` +
		"; ps aux 2>/dev/null | grep -E '(nginx|apache|httpd|php-fpm|node|python|java|postgres|mysql|mariadb|redis|mongo|caddy|haproxy|traefik|pm2|gunicorn|uvicorn|supervisord)' | grep -v grep | awk '{printf \"%s  CPU:%.1f%%  MEM:%.1f%%  RSS:%sMB\\n\", $11, $3, $4, int($6/1024)}' | head -15" +
		`; echo '` + delim2 + `'` +
		`; ss -tlnp 2>/dev/null | awk 'NR>1{split($4,a,":"); port=a[length(a)]; proc=$7; gsub(/.*users:\(\("/,"",proc); gsub(/".*/,"",proc); if(port+0>0) print port " " proc}' | sort -n -u | head -20`
	listRaw := runCmdWithWarnings(client, listCmd, warnFlags)
	sections := strings.Split(listRaw, delim2)
	section := func(i int) string {
		if i < len(sections) {
			return strings.TrimSpace(sections[i])
		}
		return ""
	}
	info.LastLogins = splitLines(section(0))
	info.Services = splitLines(section(1))
	info.ServiceDetails = splitLines(section(2))
	info.Ports = splitLines(section(3))

	// ── Sessions 4-5: Docker (needs special handling for sudo fallback) ──
	info.Containers = splitLines(captureDocker(client,
		`docker ps --format '{{.Names}} ({{.Image}}) {{.Status}}'`,
		`sudo -n docker ps --format '{{.Names}} ({{.Image}}) {{.Status}}' 2>/dev/null`,
		warnFlags,
	))
	info.ContainerStats = splitLines(captureDocker(client,
		`docker stats --no-stream --format '{{.Name}}  CPU:{{.CPUPerc}}  MEM:{{.MemUsage}}  NET:{{.NetIO}}'`,
		`sudo -n docker stats --no-stream --format '{{.Name}}  CPU:{{.CPUPerc}}  MEM:{{.MemUsage}}  NET:{{.NetIO}}' 2>/dev/null`,
		warnFlags,
	))
	// Detailed process discovery (python, node, java — long-running interpreters)
	info.ProcessDetails = captureProcessDetails(client)

	// SSH keys on the remote host (private keys in ~/.ssh/)
	keysRaw := runCmd(client, `ls ~/.ssh/id_* 2>/dev/null | grep -v '\.pub$' | while read -r f; do
		[ -f "$f.pub" ] || continue
		fp=$(ssh-keygen -lf "$f.pub" 2>/dev/null)
		[ -n "$fp" ] && echo "$(basename "$f")|$fp"
	done; true`)
	for _, line := range splitLines(keysRaw) {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		fpParts := strings.Fields(parts[1])
		if len(fpParts) < 3 {
			continue
		}
		info.SSHKeys = append(info.SSHKeys, SSHKeyInfo{
			Name:        parts[0],
			Type:        strings.Trim(fpParts[len(fpParts)-1], "()"),
			Fingerprint: fpParts[1],
			Source:      "private_key",
		})
	}

	// Authorized keys — who can log in to this host
	authKeysRaw := runCmd(client, `while IFS= read -r line; do
		echo "$line" | ssh-keygen -lf - 2>/dev/null
	done < ~/.ssh/authorized_keys 2>/dev/null`)
	for _, line := range splitLines(authKeysRaw) {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		// ssh-keygen output: "<bits> <fingerprint> <comment> (<type>)"
		// Comment may be multi-word or "no comment" when absent.
		keyType := strings.Trim(parts[len(parts)-1], "()")
		name := strings.Join(parts[2:len(parts)-1], " ")
		if name == "no comment" {
			name = ""
		}
		info.SSHKeys = append(info.SSHKeys, SSHKeyInfo{
			Name:        name,
			Type:        keyType,
			Fingerprint: parts[1],
			Source:      "authorized_keys",
		})
	}

	if info.CPU == " vCPU" {
		info.CPU = ""
	}
	if info.SwapTotal == "0B" || info.SwapTotal == "" {
		info.SwapTotal = ""
		info.SwapUsed = ""
	}
	if warnFlags["remote_dev_null_permission"] {
		info.Warnings = append(info.Warnings, "Remote host returned '/dev/null: Permission denied' while collecting metrics. Resource values may be incomplete.")
	}
	if warnFlags["docker_permission_denied"] {
		info.Warnings = append(info.Warnings, "Docker requires sudo and passwordless sudo is not configured. Container data could not be collected.")
	} else if warnFlags["docker_requires_sudo"] {
		info.Warnings = append(info.Warnings, "Docker requires sudo to run. Container data was collected via sudo -n. Consider adding the user to the docker group.")
	}
	return info, nil
}

// TestWithKey tests SSH connectivity using public key authentication.
func TestWithKey(hostname, port, user, keyPath string) error {
	if port == "" {
		port = "22"
	}

	addr := net.JoinHostPort(hostname, port)
	client, err := dialWithKey(hostname, port, user, keyPath)
	if err != nil {
		return fmt.Errorf("SSH connect to %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	if _, err := session.CombinedOutput("echo OK"); err != nil {
		return fmt.Errorf("run test command: %w", err)
	}

	return nil
}
