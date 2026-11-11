package sshtest

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// OS update backups: the host's files as one compressed tar on stdout, read
// by Bridge and streamed to MinIO as it comes (nothing is written on the
// host, no credentials go to it). Pseudo, container and network filesystems
// are left out from /proc/mounts; the configured excludes on top.

// pseudoFS are mount types never backed up: kernel views, memory, container
// layers and network shares.
const pseudoFS = "proc|sysfs|devtmpfs|devpts|tmpfs|ramfs|cgroup|cgroup2|securityfs|pstore|bpf|debugfs|tracefs|configfs|fusectl|mqueue|hugetlbfs|autofs|binfmt_misc|overlay|nsfs|squashfs|efivarfs|selinuxfs|rpc_pipefs|fuse.lxcfs|nfs|nfs4|cifs|smb3|fuse.sshfs|glusterfs|ceph|9p"

// osBackupScript writes the archive to stdout and "@@tar_exit N" to stderr.
// GNU tar exit 1 means files changed while being read (live system): fine.
const osBackupScript = `set --
for p in %[1]s; do set -- "$@" --exclude="$p"; done
while read -r dev mnt fstype rest; do
  case "$fstype" in %[2]s) [ "$mnt" = / ] || set -- "$@" --exclude="$mnt" ;; esac
done < /proc/mounts
if tar --version 2>/dev/null | grep -q GNU; then
  set -- --numeric-owner --sparse --acls --xattrs --warning=no-file-changed --ignore-failed-read "$@"
fi
if command -v zstd >/dev/null 2>&1; then Z="zstd -q -T0 -3"; else Z="gzip -1"; fi
{ tar -cf - "$@" %[3]s 2>/dev/null; echo "@@tar_exit $?" >&2; } | $Z
`

// BackupExt is the archive's extension on a host: zstd when it has it.
func BackupExt(client *ssh.Client) string {
	if out, _ := runCmdRaw(client, "command -v zstd >/dev/null 2>&1 && echo zst || echo gz"); strings.TrimSpace(out) == "zst" {
		return "tar.zst"
	}
	return "tar.gz"
}

// shQuote makes v one literal shell word.
func shQuote(v string) string { return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'" }

func backupScript(include, exclude []string) string {
	q := func(xs []string) string {
		out := make([]string, len(xs))
		for i, x := range xs {
			out[i] = shQuote(x)
		}
		return strings.Join(out, " ")
	}
	return fmt.Sprintf(osBackupScript, q(exclude), pseudoFS, q(include))
}

var tarExitRe = regexp.MustCompile(`@@tar_exit (\d+)`)

// StreamBackup writes the host's backup of include (minus exclude) to w, as
// root. It returns when the archive is complete, ctx ends (the session is
// closed) or the write side fails.
func StreamBackup(ctx context.Context, client *ssh.Client, p Privilege, include, exclude []string, w io.Writer) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	var stderr bytes.Buffer
	session.Stdout, session.Stderr = w, &stderr

	b64 := base64.StdEncoding.EncodeToString([]byte(backupScript(include, exclude)))
	inner := fmt.Sprintf("sh -c 'echo %s | base64 -d | sh'", b64)
	cmd := inner
	switch {
	case p.User == "root":
	case p.Password != "":
		stdin, err := session.StdinPipe()
		if err != nil {
			return err
		}
		go func() { io.WriteString(stdin, p.Password+"\n"); stdin.Close() }()
		cmd = "sudo -S -p '' " + inner
	default:
		cmd = "sudo -n " + inner
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(cmd) }()
	select {
	case <-ctx.Done():
		session.Close()
		<-done
		return ctx.Err()
	case err = <-done:
	}
	return backupResult(err, stderr.String())
}

// backupResult judges a finished backup from the session error and stderr.
func backupResult(runErr error, stderr string) error {
	m := tarExitRe.FindStringSubmatch(stderr)
	if m == nil {
		msg := strings.TrimSpace(stderr)
		if strings.Contains(msg, "password is required") {
			return fmt.Errorf("sudo needs a password: store the host's password or enable passwordless sudo")
		}
		if runErr != nil {
			return fmt.Errorf("backup did not run: %v %s", runErr, msg)
		}
		return fmt.Errorf("backup did not report its result")
	}
	if code, _ := strconv.Atoi(m[1]); code > 1 {
		return fmt.Errorf("tar exit %d", code)
	}
	if runErr != nil {
		return fmt.Errorf("compression failed: %v", runErr)
	}
	return nil
}
