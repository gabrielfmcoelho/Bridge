package sshtest

import "strings"

// NoPassword stands in for the sudo password on a host whose sudo asks for
// none (NOPASSWD in sudoers; typically a key-only host). Every
// `password != ""` guard lets the root path through, and the three places
// that talk to sudo (sudoInvocation, Privilege) run `sudo -n` instead of
// feeding a password. It holds a NUL byte, so it is never a real password.
const NoPassword = "\x00nopasswd"

// noPasswdProbeCmd answers "yes" when the login user reaches root through
// sudo without a password. A failed `sudo -n` is logged by the host like the
// scan's other `sudo -n` reads (docker, authorized_keys).
const noPasswdProbeCmd = `sudo -n true 2>/dev/null && echo yes || echo no`

func parseNoPasswdProbe(out string) bool {
	fields := strings.Fields(out)
	return len(fields) > 0 && fields[len(fields)-1] == "yes"
}

// elevationSecret is what the scan elevates with: the stored password, or
// NoPassword when there is none and sudo asks for none.
func elevationSecret(stored string, noPasswd bool) string {
	if stored == "" && noPasswd {
		return NoPassword
	}
	return stored
}

// sudoInvocation wraps inner in sudo: `sudo -n` for NoPassword, else `sudo -S`
// reading the password from stdin (feed), never from the command line. The
// empty -p prompt keeps sudo's own prompt from being glued onto the output.
func sudoInvocation(password, inner string) (cmd string, feed bool) {
	if password == NoPassword {
		return "sudo -n " + inner, false
	}
	return "sudo -S -p '' " + inner, true
}

// feedsPassword: root is reached with a stored password on sudo's stdin
// (not as root already, not passwordless).
func (p Privilege) feedsPassword() bool {
	return p.User != "root" && p.Password != "" && p.Password != NoPassword
}
