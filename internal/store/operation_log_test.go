package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestOperationLogRepo_CreateAndListByHost(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	// host_operation_logs has FKs to hosts and users; ListByHost JOINs users.
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	var userID int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role, display_name) VALUES ('u','x','admin','User U') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	repo := store.NewOperationLogRepo(d.SQL)
	method := "key"
	ol := &models.OperationLog{HostID: hostID, UserID: userID, OperationType: "ssh-test", AuthMethod: &method, Status: "ok", Output: "done"}
	if err := repo.Create(ctx, ol); err != nil {
		t.Fatalf("create: %v", err)
	}
	if ol.ID == 0 {
		t.Fatal("create did not set ID")
	}

	logs, err := repo.ListByHost(ctx, hostID, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 || logs[0].OperationType != "ssh-test" || logs[0].UserName != "User U" {
		t.Fatalf("logs = %+v, want one ssh-test by 'User U'", logs)
	}

	// A different host has no logs.
	other, _ := repo.ListByHost(ctx, hostID+999, 10)
	if len(other) != 0 {
		t.Fatalf("other-host logs = %d, want 0", len(other))
	}
}
