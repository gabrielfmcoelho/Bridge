package sshtest

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Auth represents an SSH authentication method — either password or in-memory
// private key. Use PasswordAuth or KeyAuth to construct one.
type Auth struct {
	method string // "password" or "key"
	password string
	keyPEM   []byte
}

// Method returns "password" or "key".
func (a Auth) Method() string { return a.method }

// Password returns the stored password (available for both auth types when the
// host has a saved password — needed for sudo operations even on key-auth hosts).
func (a Auth) Password() string { return a.password }

// PasswordAuth creates an Auth that connects with password.
func PasswordAuth(password string) Auth {
	return Auth{method: "password", password: password}
}

// KeyAuth creates an Auth that connects with a private key. The password is
// still carried for sudo operations on the remote host.
func KeyAuth(keyPEM []byte, sudoPassword string) Auth {
	return Auth{method: "key", keyPEM: keyPEM, password: sudoPassword}
}

// Dial opens an SSH connection using the provided auth method.
func Dial(hostname, port, user string, auth Auth) (*ssh.Client, error) {
	if port == "" {
		port = "22"
	}

	var methods []ssh.AuthMethod
	switch auth.method {
	case "key":
		if len(auth.keyPEM) == 0 {
			return nil, fmt.Errorf("no private key bytes provided")
		}
		signer, err := ssh.ParsePrivateKey(auth.keyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		methods = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		methods = []ssh.AuthMethod{ssh.Password(auth.password)}
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(hostname, port), config)
}

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
	ParsedContainers  []ContainerInfo    `json:"parsed_containers,omitempty"`
	Warnings          []string           `json:"warnings,omitempty"`
	ProcessDetails    []ProcessDetail    `json:"process_details,omitempty"`
	SSHKeys           []SSHKeyInfo       `json:"ssh_keys,omitempty"`
	SystemdServices   []SystemdService   `json:"systemd_services,omitempty"`
	InstalledPackages []InstalledPackage  `json:"installed_packages,omitempty"`
	CronJobs          []string           `json:"cron_jobs,omitempty"`
	FirewallStatus    string             `json:"firewall_status,omitempty"`
}

// ContainerInfo holds structured data about a running Docker container.
type ContainerInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
	Ports  string `json:"ports"`
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

// SystemdService holds info about a running systemd unit.
type SystemdService struct {
	Unit        string `json:"unit"`
	Description string `json:"description,omitempty"`
	IsNative    bool   `json:"is_native"`
}

// InstalledPackage holds info about a key package found on the host.
type InstalledPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"` // "apt", "rpm"
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

// Test runs a simple "echo OK" over SSH to verify connectivity.
func Test(client *ssh.Client) error {
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

// TestCapture tests SSH connectivity and captures VM info.
func TestCapture(client *ssh.Client) (*VMInfo, error) {
	return captureVMInfo(client)
}

// FixDevNull attempts to repair /dev/null permissions on the remote host.
func FixDevNull(client *ssh.Client) (string, error) {
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

// runSudoCmd executes a command under sudo on the remote host, feeding the
// password via the session's stdin pipe. This avoids interpolating the password
// into the command string (which would expose it in /proc/*/cmdline and break
// on passwords containing shell metacharacters like single quotes).
func runSudoCmd(client *ssh.Client, password, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}

	// Feed the password followed by a newline to sudo -S, then close stdin
	// so the wrapped command's own stdin is EOF (safe for non-interactive use).
	go func() {
		fmt.Fprintln(stdin, password)
		stdin.Close()
	}()

	fullCmd := fmt.Sprintf("sudo -S sh -c %q 2>&1", cmd)
	out, execErr := session.CombinedOutput(fullCmd)
	cleaned := strings.TrimSpace(cleanCommandOutput(string(out), nil))
	if execErr != nil {
		if cleaned == "" {
			return "", execErr
		}
		return cleaned, fmt.Errorf("%s: %w", cleaned, execErr)
	}
	return cleaned, nil
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

// SetupSudoNopasswd configures passwordless sudo for the given user.
func SetupSudoNopasswd(client *ssh.Client, user, password string) (string, error) {
	return setupSudoNopasswd(client, user, password)
}

func setupSudoNopasswd(client *ssh.Client, user, password string) (string, error) {
	file := fmt.Sprintf("/etc/sudoers.d/%s-nopasswd", user)
	rule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", user)

	// Check if already configured
	checkOut, _ := runCmdRaw(client, `sudo -n true 2>/dev/null && echo ALREADY_OK || echo NEEDS_SETUP`)
	if strings.TrimSpace(checkOut) == "ALREADY_OK" {
		return "NOPASSWD already configured for " + user, nil
	}

	// Write sudoers drop-in via stdin-piped sudo (password never in command string)
	script := fmt.Sprintf(
		`echo "%s" > %s && chmod 440 %s && visudo -cf %s`,
		rule, file, file, file,
	)
	output, err := runSudoCmd(client, password, script)
	if err != nil {
		return output, fmt.Errorf("failed to configure sudoers: %w", err)
	}

	// Verify it works
	verifyOut, _ := runCmdRaw(client, `sudo -n true 2>/dev/null && echo OK || echo FAIL`)
	if strings.TrimSpace(verifyOut) == "OK" {
		return fmt.Sprintf("Created %s — NOPASSWD enabled for %s", file, user), nil
	}

	// Drop-in didn't take effect — try appending directly to /etc/sudoers.
	directScript := fmt.Sprintf(
		`grep -qxF "%s" /etc/sudoers 2>/dev/null || `+
			`(echo "%s" >> /etc/sudoers && visudo -cf /etc/sudoers)`,
		rule, rule,
	)
	directOut, directErr := runSudoCmd(client, password, directScript)
	if directErr != nil {
		return output + "\n" + directOut, fmt.Errorf("failed to append rule to /etc/sudoers: %w", directErr)
	}

	verifyOut2, _ := runCmdRaw(client, `sudo -n true 2>/dev/null && echo OK || echo FAIL`)
	if strings.TrimSpace(verifyOut2) != "OK" {
		return output + "\n" + directOut + "\nNOPASSWD still not effective",
			fmt.Errorf("rule written to both %s and /etc/sudoers but sudo -n still fails — check PAM config", file)
	}

	return fmt.Sprintf("Appended NOPASSWD rule to /etc/sudoers — enabled for %s", user), nil
}

// CreateRemoteUserWithPassword connects via password and creates a user on the remote host
// with an authorized public key and passwordless sudo.
// CreateRemoteUser creates a new user on the remote host with an authorized
// public key and passwordless sudo.
func CreateRemoteUser(client *ssh.Client, password, newUser, pubKey string, force bool) (string, error) {
	return createRemoteUser(client, password, newUser, pubKey, force)
}

// ErrUserExists is returned when the target user already exists on the remote
// host and force was not set. The caller can inspect this to prompt the user.
var ErrUserExists = fmt.Errorf("user already exists")

func createRemoteUser(client *ssh.Client, sudoPassword, newUser, pubKey string, force bool) (string, error) {
	var out strings.Builder

	// 1. Check if user already exists (use runCmdRaw so transient errors surface)
	check := fmt.Sprintf(`id %s 2>/dev/null && echo EXISTS || echo MISSING`, newUser)
	checkRes, checkErr := runCmdRaw(client, check)
	if checkErr != nil {
		return checkRes, fmt.Errorf("failed to check if user %s exists: %w", newUser, checkErr)
	}

	userExists := strings.TrimSpace(checkRes) == "EXISTS"
	if userExists && !force {
		return fmt.Sprintf("User %s already exists on the remote host. Use the force option to proceed anyway (this will add SSH key access and NOPASSWD sudo to the existing account).", newUser), ErrUserExists
	}
	if userExists {
		out.WriteString(fmt.Sprintf("User %s already exists — skipping creation.\n", newUser))
	} else {
		// Create user with home dir, no password login
		res, err := runSudoCmd(client, sudoPassword,
			fmt.Sprintf("useradd -m -s /bin/bash %s", newUser))
		if err != nil {
			return res, fmt.Errorf("failed to create user %s: %w", newUser, err)
		}
		out.WriteString(fmt.Sprintf("Created user %s.\n", newUser))
	}

	// 2. Set up authorized_keys
	sshDir := fmt.Sprintf("/home/%s/.ssh", newUser)
	authKeys := fmt.Sprintf("%s/authorized_keys", sshDir)
	setupScript := fmt.Sprintf(
		`mkdir -p %s && `+
			`grep -qF %q %s 2>/dev/null || echo %q >> %s && `+
			`chown -R %s:%s %s && `+
			`chmod 700 %s && chmod 600 %s`,
		sshDir,
		pubKey, authKeys, pubKey, authKeys,
		newUser, newUser, sshDir,
		sshDir, authKeys,
	)
	res, err := runSudoCmd(client, sudoPassword, setupScript)
	if err != nil {
		return out.String() + res, fmt.Errorf("failed to setup authorized_keys: %w", err)
	}
	out.WriteString(fmt.Sprintf("Authorized key added to %s.\n", authKeys))

	// 3. Configure passwordless sudo
	sudoFile := fmt.Sprintf("/etc/sudoers.d/%s-nopasswd", newUser)
	sudoRule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", newUser)
	sudoScript := fmt.Sprintf(
		`echo "%s" > %s && chmod 440 %s && visudo -cf %s`,
		sudoRule, sudoFile, sudoFile, sudoFile,
	)
	res, err = runSudoCmd(client, sudoPassword, sudoScript)
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

// CheckAndFixDockerGroup checks Docker installation and optionally adds user to docker group.
func CheckAndFixDockerGroup(client *ssh.Client, user, password string, fix bool) (*DockerStatus, error) {
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
		cmd := fmt.Sprintf("usermod -aG docker %s", user)
		var err error
		if password != "" {
			_, err = runSudoCmd(client, password, cmd)
		} else {
			_, err = runCmdRaw(client, fmt.Sprintf("sudo -n %s", cmd))
		}
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

// NginxCleanupStatus holds the result of a nginx cleanup operation.
type NginxCleanupStatus struct {
	Found          bool               `json:"found"`
	IsNative       bool               `json:"is_native"`
	IsContainer    bool               `json:"is_container"`
	BackupPath     string             `json:"backup_path,omitempty"`
	Steps          []NginxCleanupStep `json:"steps"`
	PackageManager string             `json:"package_manager,omitempty"`
	Message        string             `json:"message"`
}

// NginxCleanupStep represents one step in the cleanup process.
type NginxCleanupStep struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "success", "failed", "skipped"
	Output string `json:"output,omitempty"`
}

// CleanupNginx detects and removes non-containerized nginx from a remote host.
func CleanupNginx(client *ssh.Client, password string, purge bool) (result *NginxCleanupStatus, retErr error) {
	defer func() {
		if rv := recover(); rv != nil {
			result = &NginxCleanupStatus{
				Message: fmt.Sprintf("internal error during cleanup: %v", rv),
			}
			retErr = fmt.Errorf("panic: %v", rv)
		}
	}()

	status := &NginxCleanupStatus{}

	// Step 1: Detect nginx — systemctl, which, and docker check
	const sep = "---SEP---"
	detectCmd := `systemctl is-active nginx 2>/dev/null || echo inactive` +
		`; echo '` + sep + `'` +
		`; which nginx 2>/dev/null || echo not_found` +
		`; echo '` + sep + `'` +
		`; docker ps --filter name=nginx --format '{{.Names}}' 2>/dev/null || sudo -n docker ps --filter name=nginx --format '{{.Names}}' 2>/dev/null` +
		`; echo '` + sep + `'` +
		`; which apt 2>/dev/null && echo apt || (which dnf 2>/dev/null && echo dnf) || (which yum 2>/dev/null && echo yum) || echo unknown`

	detectRaw := runCmd(client, detectCmd)
	parts := strings.Split(detectRaw, sep)
	detectField := func(i int) string {
		if i < len(parts) {
			return strings.TrimSpace(parts[i])
		}
		return ""
	}

	systemctlActive := detectField(0)
	whichNginx := detectField(1)
	dockerNginx := detectField(2)
	pkgMgr := detectField(3)
	// Extract just the package manager name from last line
	pkgLines := splitLines(pkgMgr)
	if len(pkgLines) > 0 {
		pkgMgr = pkgLines[len(pkgLines)-1]
	}
	status.PackageManager = pkgMgr

	hasNativeNginx := systemctlActive == "active" || (whichNginx != "" && whichNginx != "not_found")
	hasContainerNginx := dockerNginx != ""

	if !hasNativeNginx && !hasContainerNginx {
		status.Found = false
		status.Message = "Nginx not found on this host (neither native nor containerized)."
		return status, nil
	}

	status.Found = true
	status.IsContainer = hasContainerNginx
	status.IsNative = hasNativeNginx

	if !hasNativeNginx && hasContainerNginx {
		status.Message = fmt.Sprintf("Nginx is running only inside Docker container(s): %s. Use Docker to manage it instead.", dockerNginx)
		return status, nil
	}

	if hasContainerNginx {
		status.Message = "Warning: nginx is running both natively and in containers. Proceeding to clean up the native installation only."
	}

	// Step 2: Backup /etc/nginx/
	backupFile := fmt.Sprintf("/tmp/nginx-backup-%s.tar.gz", strings.ReplaceAll(
		strings.ReplaceAll(runCmd(client, "date +%Y%m%d%H%M%S"), "\n", ""), " ", ""))
	if backupFile == "/tmp/nginx-backup-.tar.gz" {
		backupFile = "/tmp/nginx-backup-manual.tar.gz"
	}

	backupOut, backupErr := runSudoCmd(client, password,
		fmt.Sprintf("tar -czf %s /etc/nginx/ 2>&1", backupFile))
	if backupErr != nil {
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Backup /etc/nginx/", Status: "failed", Output: backupOut,
		})
		status.Message = "Backup failed — aborting cleanup to preserve configuration."
		return status, nil
	}
	status.BackupPath = backupFile
	status.Steps = append(status.Steps, NginxCleanupStep{
		Name: "Backup /etc/nginx/", Status: "success", Output: backupFile,
	})

	// Step 3: Stop nginx service
	stopOut, stopErr := runSudoCmd(client, password, "systemctl stop nginx 2>&1")
	if stopErr != nil {
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Stop nginx service", Status: "failed", Output: stopOut,
		})
		status.Message = "Failed to stop nginx — aborting further steps."
		return status, nil
	}
	status.Steps = append(status.Steps, NginxCleanupStep{
		Name: "Stop nginx service", Status: "success", Output: stopOut,
	})

	// Step 4: Disable nginx service
	disableOut, disableErr := runSudoCmd(client, password, "systemctl disable nginx 2>&1")
	if disableErr != nil {
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Disable nginx service", Status: "failed", Output: disableOut,
		})
	} else {
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Disable nginx service", Status: "success", Output: disableOut,
		})
	}

	// Step 5: Purge (optional)
	if !purge {
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Purge nginx packages", Status: "skipped", Output: "Purge not requested",
		})
		status.Message = fmt.Sprintf("Nginx stopped and disabled. Config backed up to %s.", backupFile)
		return status, nil
	}

	var purgeCmd string
	switch pkgMgr {
	case "apt":
		purgeCmd = "apt purge nginx nginx-common nginx-full nginx-core nginx-light -y 2>&1 && apt autoremove -y 2>&1"
	case "dnf":
		purgeCmd = "dnf remove nginx -y 2>&1"
	case "yum":
		purgeCmd = "yum remove nginx -y 2>&1"
	default:
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Purge nginx packages", Status: "failed", Output: "Unknown package manager: " + pkgMgr,
		})
		status.Message = fmt.Sprintf("Nginx stopped and disabled but could not purge (unknown package manager). Config backed up to %s.", backupFile)
		return status, nil
	}

	purgeOut, purgeErr := runSudoCmd(client, password, purgeCmd)
	if purgeErr != nil {
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Purge nginx packages", Status: "failed", Output: purgeOut,
		})
		status.Message = fmt.Sprintf("Nginx stopped and disabled but purge failed. Config backed up to %s.", backupFile)
	} else {
		status.Steps = append(status.Steps, NginxCleanupStep{
			Name: "Purge nginx packages", Status: "success", Output: purgeOut,
		})
		status.Message = fmt.Sprintf("Nginx fully removed. Config backed up to %s.", backupFile)
	}

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
// ListRemoteKeys lists SSH keys found on the remote host.
func ListRemoteKeys(client *ssh.Client) ([]RemoteKeyInfo, error) {
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

func safeIndex(parts []string, i int) string {
	if i < len(parts) {
		return parts[i]
	}
	return ""
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
	// Enhanced format captures container ID and port mappings.
	dockerRaw := captureDocker(client,
		`docker ps --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'`,
		`sudo -n docker ps --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null`,
		warnFlags,
	)
	// Populate old string field for backward compatibility and parse structured data.
	for _, line := range splitLines(dockerRaw) {
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) >= 3 {
			// Old format: "name (image) status"
			info.Containers = append(info.Containers, parts[1]+" ("+parts[2]+") "+safeIndex(parts, 3))
			info.ParsedContainers = append(info.ParsedContainers, ContainerInfo{
				ID:     parts[0],
				Name:   parts[1],
				Image:  parts[2],
				Status: safeIndex(parts, 3),
				Ports:  safeIndex(parts, 4),
			})
		} else {
			info.Containers = append(info.Containers, line)
		}
	}
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

	// ── Session 6: enhanced service discovery ──
	const delimEnhanced = "---ENHANCED---"
	enhancedCmd := `systemctl list-units --type=service --state=running --no-pager --plain 2>/dev/null | awk 'NR>0 && $1 ~ /\.service$/ {unit=$1; desc=""; for(i=5;i<=NF;i++) desc=desc" "$i; print unit "\t" desc}'` +
		`; echo '` + delimEnhanced + `'` +
		`; dpkg-query -W -f='${Package}\t${Version}\n' nginx apache2 postgresql mysql-server redis-server nodejs python3 php docker-ce 2>/dev/null || rpm -qa --queryformat '%{NAME}\t%{VERSION}\n' 2>/dev/null | grep -iE '(nginx|httpd|postgresql|mysql|redis|nodejs|python3|php|docker-ce)'` +
		`; echo '` + delimEnhanced + `'` +
		`; crontab -l 2>/dev/null` +
		`; echo '` + delimEnhanced + `'` +
		`; ufw status 2>/dev/null || iptables -L -n 2>/dev/null | head -20 || echo 'no firewall detected'`

	enhancedRaw := runCmd(client, enhancedCmd)
	enhancedSections := strings.Split(enhancedRaw, delimEnhanced)
	enhancedSection := func(i int) string {
		if i < len(enhancedSections) {
			return strings.TrimSpace(enhancedSections[i])
		}
		return ""
	}

	// Parse systemd services
	for _, line := range splitLines(enhancedSection(0)) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		desc := ""
		if len(parts) > 1 {
			desc = strings.TrimSpace(parts[1])
		}
		info.SystemdServices = append(info.SystemdServices, SystemdService{
			Unit:        parts[0],
			Description: desc,
			IsNative:    true, // default; will cross-reference with containers below
		})
	}

	// Cross-reference systemd services with docker containers to flag native vs container
	containerNames := map[string]bool{}
	for _, c := range info.Containers {
		parts := strings.SplitN(c, " ", 2)
		if len(parts) > 0 {
			containerNames[strings.ToLower(parts[0])] = true
		}
	}
	for i, svc := range info.SystemdServices {
		unitBase := strings.TrimSuffix(strings.ToLower(svc.Unit), ".service")
		for cname := range containerNames {
			if strings.Contains(unitBase, cname) || strings.Contains(cname, unitBase) {
				info.SystemdServices[i].IsNative = false
				break
			}
		}
	}

	// Parse installed packages
	pkgSource := "apt"
	for _, line := range splitLines(enhancedSection(1)) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		// Detect if this looks like rpm output (no tab-separated version from dpkg)
		if strings.Contains(parts[1], ".el") || strings.Contains(parts[1], ".fc") {
			pkgSource = "rpm"
		}
		info.InstalledPackages = append(info.InstalledPackages, InstalledPackage{
			Name:    parts[0],
			Version: parts[1],
			Source:  pkgSource,
		})
	}

	// Parse cron jobs
	info.CronJobs = splitLines(enhancedSection(2))

	// Firewall status
	info.FirewallStatus = enhancedSection(3)

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

