package store

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

func TestHostServiceRowsInstanceKey(t *testing.T) {
	rows := hostServiceRows([]sshtest.DiscoveredService{{
		Name: "jboss", Label: "JBoss AS", Instance: "compras1.service", HostRunning: true, HostPorts: []int{8080},
	}})
	if len(rows) != 1 || rows[0].key != "jboss@compras1.service" || rows[0].port != "8080" || rows[0].nickname != "JBoss AS · compras1" {
		t.Fatalf("rows = %+v", rows)
	}
}
