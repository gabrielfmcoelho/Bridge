package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestDatabaseRepo_ScopeLinksInstances(t *testing.T) {
	d := openDB(t)
	ctx := context.Background()
	svcRepo := store.NewServiceRepo(d.SQL)
	mkSvc := func(nick, kind, subtype, image string) int64 {
		s := &models.Service{Nickname: nick, ServiceKind: kind, ServiceSubtype: subtype, ContainerImage: image}
		if err := svcRepo.Create(ctx, s); err != nil {
			t.Fatalf("create service %s: %v", nick, err)
		}
		return s.ID
	}
	pg := mkSvc("pg-main", "database", "PostgreSQL", "postgres:16")
	app := mkSvc("sei-app", "app", "", "")
	mkSvc("redis-cache", "cache", "", "redis:7")

	repo := store.NewDatabaseRepo(d.SQL)
	mk := func(name, dbName string, instance *int64) int64 {
		db := &models.Database{Name: name, DBName: dbName, Engine: "postgres", InstanceServiceID: instance, ExternalProvider: "ATI"}
		if err := repo.Create(ctx, db); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return db.ID
	}
	mine := mk("SEI", "sei_prod", &pg)
	other := mk("SIAPE", "", nil)

	// Same instance + db_name is one database.
	dup := &models.Database{Name: "SEI again", DBName: "sei_prod", Engine: "postgres", InstanceServiceID: &pg}
	if err := repo.Create(ctx, dup); !store.IsUniqueViolation(err) {
		t.Fatalf("duplicate create err = %v; want unique violation", err)
	}

	if err := repo.SetLinks(ctx, mine, []int64{app, app}, []int64{}); err != nil {
		t.Fatalf("set links: %v", err)
	}
	got, err := repo.Get(ctx, mine)
	if err != nil || got == nil {
		t.Fatalf("get: %+v, %v", got, err)
	}
	if len(got.ConsumerServiceIDs) != 1 || got.ConsumerServiceIDs[0] != app || len(got.ProjectIDs) != 0 {
		t.Fatalf("links = %v / %v; want [%d] / []", got.ConsumerServiceIDs, got.ProjectIDs, app)
	}
	if got.Instance == nil || got.Instance.ID != pg || got.Instance.Nickname != "pg-main" {
		t.Fatalf("instance = %+v; want pg-main", got.Instance)
	}
	if list, err := repo.List(ctx, models.DatabaseFilter{ConsumerServiceID: app}); err != nil || len(list) != 1 || list[0].ID != mine {
		t.Fatalf("consumer filter = %+v, %v", list, err)
	}

	insts, err := repo.Instances(ctx)
	if err != nil {
		t.Fatalf("instances: %v", err)
	}
	byNick := map[string]models.DatabaseInstance{}
	for _, in := range insts {
		byNick[in.Nickname] = in
	}
	if len(insts) != 2 || byNick["pg-main"].Engine != "postgres" || byNick["pg-main"].RegisteredCount != 1 ||
		byNick["redis-cache"].Engine != "redis" || byNick["redis-cache"].RegisteredCount != 0 {
		t.Fatalf("instances = %+v; want pg-main(postgres,1) and redis-cache(redis,0), no sei-app", insts)
	}

	// Entidade scope: a scoped caller sees only the granted row; the
	// instance summary needs the service visible too.
	var sga int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug='sga'`).Scan(&sga); err != nil {
		t.Fatalf("sga: %v", err)
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(ctx, d.SQL, store.AssetDatabase, mine, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	scoped := store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{sga}})
	list, err := repo.List(scoped, models.DatabaseFilter{})
	if err != nil || len(list) != 1 || list[0].ID != mine {
		t.Fatalf("scoped list = %+v, %v; want only %d", list, err, mine)
	}
	if list[0].Instance != nil {
		t.Fatalf("scoped instance = %+v; want nil (service not granted)", list[0].Instance)
	}
	if g, err := repo.Get(scoped, other); err != nil || g != nil {
		t.Fatalf("scoped get other = %+v, %v; want nil", g, err)
	}
	o, _ := repo.Get(ctx, other)
	o.Name = "renamed"
	if err := repo.Update(scoped, o); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("scoped update other err = %v; want ErrNoRows", err)
	}
	if insts, err := repo.Instances(scoped); err != nil || len(insts) != 0 {
		t.Fatalf("scoped instances = %+v, %v; want none", insts, err)
	}

	// Delete cascades through the registered parent; restore brings it back.
	if err := store.DeleteParent(ctx, d.SQL, models.ActorContext{Role: "admin"}, models.SecretScopeDatabase, other); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if g, _ := repo.Get(ctx, other); g != nil {
		t.Fatalf("deleted row still live")
	}
	if trash, err := repo.ListTrash(ctx); err != nil || len(trash) != 1 || trash[0].ID != other {
		t.Fatalf("trash = %+v, %v", trash, err)
	}
	if found, err := repo.Restore(ctx, other); err != nil || !found {
		t.Fatalf("restore: %v, %v", found, err)
	}
	if g, _ := repo.Get(ctx, other); g == nil || g.Name != "SIAPE" {
		t.Fatalf("restored = %+v", g)
	}
}

func TestDatabaseValidate(t *testing.T) {
	pg := int64(1)
	host := int64(2)
	port := 70000
	for _, c := range []struct {
		name string
		d    models.Database
		ok   bool
	}{
		{"on instance", models.Database{Name: "a", Engine: "postgres", InstanceServiceID: &pg}, true},
		{"external with provider", models.Database{Name: "a", Engine: "oracle", ExternalProvider: "SERPRO"}, true},
		{"external with nothing", models.Database{Name: "a", Engine: "oracle"}, false},
		{"no name", models.Database{Engine: "postgres", InstanceServiceID: &pg}, false},
		{"bad engine", models.Database{Name: "a", Engine: "db2", InstanceServiceID: &pg}, false},
		{"tunnel without host", models.Database{Name: "a", Engine: "postgres", InstanceServiceID: &pg, ConnectionMode: "ssh_tunnel"}, false},
		{"tunnel with host", models.Database{Name: "a", Engine: "postgres", InstanceServiceID: &pg, ConnectionMode: "ssh_tunnel", TunnelHostID: &host}, true},
		{"bad port", models.Database{Name: "a", Engine: "postgres", InstanceServiceID: &pg, Port: &port}, false},
		{"bad sensitivity", models.Database{Name: "a", Engine: "postgres", InstanceServiceID: &pg, Sensitivity: "secreto"}, false},
	} {
		c.d.ApplyDefaults()
		if err := c.d.Validate(); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
	for in, want := range map[[2]string]string{
		{"PostgreSQL", ""}: "postgres", {"", "mariadb:11"}: "mariadb", {"MySQL", "mysql:8"}: "mysql",
		{"", "trinodb/trino:450"}: "trino", {"Redis", ""}: "redis", {"", "nginx"}: "other",
	} {
		if got := models.InferDatabaseEngine(in[0], in[1]); got != want {
			t.Errorf("InferDatabaseEngine(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
