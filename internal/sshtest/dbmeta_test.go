package sshtest

import (
	"strings"
	"testing"
)

// fixtureDBProbe is a synthetic probe stream in the host-35 shape: MariaDB, a
// candidate database exposed on every interface though the config pins
// 127.0.0.1, root reachable from anywhere, one empty-password account, failed
// logins from both source classes. b64() is the shared test helper (deepfacts_test.go).
func fixtureDBProbe() string {
	return strings.Join([]string{
		"@@DBBEGIN",
		"@@ver 10.3.39-MariaDB-0ubuntu0.20.04.1",
		"@@schema " + b64("concurso") + " 812 47",
		"@@schema " + b64("mysql") + " 2 31", // system schema: dropped
		"@@table " + b64("concurso") + " " + b64("candidatos"),
		"@@table " + b64("concurso") + " " + b64("inscricoes"),
		"@@pii " + b64("concurso"),
		"@@acct " + b64("root") + " " + b64("%") + " 0",
		"@@acct " + b64("app") + " " + b64("localhost") + " 1",
		"@@bind 0.0.0.0",
		"@@cfgbind 127.0.0.1",
		"@@binlog 86400",
		"@@denied 3 17",
		"@@DBEND",
	}, "\n")
}

func TestParseDBMeta(t *testing.T) {
	m := parseDBMeta(fixtureDBProbe())
	if m == nil {
		t.Fatal("parseDBMeta returned nil for a @@DBBEGIN stream")
	}
	if m.Engine != "mariadb" {
		t.Errorf("engine = %q, want mariadb", m.Engine)
	}
	if len(m.Schemas) != 1 || m.Schemas[0].Name != "concurso" {
		t.Fatalf("schemas = %+v, want one non-system schema 'concurso'", m.Schemas)
	}
	sc := m.Schemas[0]
	if sc.SizeMB != 812 || sc.Tables != 47 {
		t.Errorf("schema size/tables = %d/%d, want 812/47", sc.SizeMB, sc.Tables)
	}
	if !sc.PII {
		t.Error("schema concurso should be flagged PII (cpf column seen)")
	}
	if len(sc.Names) != 2 || sc.Names[0] != "candidatos" {
		t.Errorf("table names = %v, want [candidatos inscricoes]", sc.Names)
	}
	if len(m.Accounts) != 2 {
		t.Fatalf("accounts = %+v, want 2", m.Accounts)
	}
	if !m.Accounts[0].RootAny {
		t.Error("root@% should set RootAny")
	}
	if m.Accounts[0].EmptyPassword {
		t.Error("root@% here has a password (flag 0)")
	}
	if !m.Accounts[1].EmptyPassword {
		t.Error("app@localhost has an empty password (flag 1)")
	}
	if m.BindAddress != "0.0.0.0" || m.ConfigBind != "127.0.0.1" || !m.Exposed {
		t.Errorf("bind=%q cfg=%q exposed=%v, want 0.0.0.0 / 127.0.0.1 / true", m.BindAddress, m.ConfigBind, m.Exposed)
	}
	if m.BinlogSeconds != 86400 {
		t.Errorf("binlog = %d, want 86400", m.BinlogSeconds)
	}
	if m.DeniedPrivate != 3 || m.DeniedPublic != 17 {
		t.Errorf("denied = %d/%d, want 3/17", m.DeniedPrivate, m.DeniedPublic)
	}
}

func TestBindExposed(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{{"", true}, {"*", true}, {"0.0.0.0", true}, {"::", true}, {"127.0.0.1", false}, {"10.0.0.5", false}} {
		if got := bindExposed(tc.in); got != tc.want {
			t.Errorf("bindExposed(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
