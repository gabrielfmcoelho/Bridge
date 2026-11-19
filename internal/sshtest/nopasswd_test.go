package sshtest

import (
	"strings"
	"testing"
)

// A host whose sudo asks for no password: run `sudo -n`, feed nothing.
func TestSudoInvocationWithoutPassword(t *testing.T) {
	cmd, feed := sudoInvocation(NoPassword, "sh -c X")
	if cmd != "sudo -n sh -c X" || feed {
		t.Fatalf("cmd=%q feed=%v", cmd, feed)
	}
}

func TestSudoInvocationWithPassword(t *testing.T) {
	cmd, feed := sudoInvocation("s3cret", "sh -c X")
	if cmd != "sudo -S -p '' sh -c X" || !feed {
		t.Fatalf("cmd=%q feed=%v", cmd, feed)
	}
	if strings.Contains(cmd, "s3cret") {
		t.Fatal("the password must never be in the command line")
	}
}

// A stored password always wins; the stand-in is only for a host with none.
func TestElevationSecret(t *testing.T) {
	for _, c := range []struct {
		stored   string
		noPasswd bool
		want     string
	}{
		{"", true, NoPassword},
		{"", false, ""},
		{"s3cret", true, "s3cret"},
		{"s3cret", false, "s3cret"},
	} {
		if got := elevationSecret(c.stored, c.noPasswd); got != c.want {
			t.Errorf("elevationSecret(%q, %v) = %q, want %q", c.stored, c.noPasswd, got, c.want)
		}
	}
}

// The probe's answer is the last word of its section: anything but "yes" is no.
func TestParseNoPasswdProbe(t *testing.T) {
	for out, want := range map[string]bool{"yes": true, " yes\n": true, "no": false, "": false, "sudo: a password is required\nno": false} {
		if got := parseNoPasswdProbe(out); got != want {
			t.Errorf("parseNoPasswdProbe(%q) = %v, want %v", out, got, want)
		}
	}
}

// Privilege reaches root without a password whether it is empty or the stand-in.
func TestPrivilegeNeedsPassword(t *testing.T) {
	if (Privilege{User: "ops", Password: NoPassword}).feedsPassword() || (Privilege{User: "ops"}).feedsPassword() {
		t.Fatal("no stored password: nothing to feed")
	}
	if !(Privilege{User: "ops", Password: "s3cret"}).feedsPassword() {
		t.Fatal("a stored password is fed to sudo -S")
	}
}
