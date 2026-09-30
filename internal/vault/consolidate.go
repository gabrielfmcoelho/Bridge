package vault

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// consolidate.go — the same password or key repeated on many VMs (one
// per-host copy each, from before credentials could be shared) becomes one
// shared credential the VMs link to. Grouped by the keyed value fingerprint
// plus the login user, so nothing is decrypted to find them. Admin action,
// previewed first; the copies are soft-deleted (restorable from the trash).

var consolidateMeta = []byte(`{"migration":"consolidate_host_creds"}`)

// ConsolidationGroup is one set of identical per-host copies.
type ConsolidationGroup struct {
	Key       string            `json:"key"` // opaque, stable while the copies don't change
	Type      models.SecretType `json:"type"`
	Username  string            `json:"username"` // the hosts' login user
	HostIDs   []int64           `json:"host_ids"`
	HostNames []string          `json:"host_names"`
	// Target is an existing shared credential with the same value (e.g. the
	// key migrated from the old library), reused instead of creating one.
	TargetID   *int64 `json:"target_id,omitempty"`
	TargetName string `json:"target_name,omitempty"`

	secretIDs []int64
	fp        []byte
}

// PlanHostCredentialConsolidation lists groups of 2+ hosts sharing a
// password or key copy.
func PlanHostCredentialConsolidation(ctx context.Context, db *sql.DB) ([]ConsolidationGroup, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.id, s.type, s.value_fingerprint, h.id, h.nickname, COALESCE(h.ssh_user, '')
		  FROM secrets s
		  JOIN hosts h ON h.id = s.parent_id AND h.deleted_at IS NULL
		 WHERE s.scope = 'host' AND s.visibility = 'shared' AND s.deleted_at IS NULL
		   AND s.type IN ('password', 'sshkey') AND s.value_fingerprint IS NOT NULL
		 ORDER BY h.nickname, h.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byKey := map[string]*ConsolidationGroup{}
	var order []string
	for rows.Next() {
		var sid, hid int64
		var typ, nick, user string
		var fp []byte
		if err := rows.Scan(&sid, &typ, &fp, &hid, &nick, &user); err != nil {
			return nil, err
		}
		sum := sha256.Sum256([]byte(typ + "\x1f" + string(fp) + "\x1f" + user))
		key := hex.EncodeToString(sum[:8])
		g, ok := byKey[key]
		if !ok {
			g = &ConsolidationGroup{Key: key, Type: models.SecretType(typ), Username: user, fp: fp}
			byKey[key] = g
			order = append(order, key)
		}
		g.HostIDs = append(g.HostIDs, hid)
		g.HostNames = append(g.HostNames, nick)
		g.secretIDs = append(g.secretIDs, sid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []ConsolidationGroup
	for _, k := range order {
		g := byKey[k]
		if len(g.HostIDs) < 2 {
			continue
		}
		var id int64
		var name string
		err := db.QueryRowContext(ctx, `
			SELECT id, name FROM secrets
			 WHERE scope = 'avulso' AND visibility = 'shared' AND type = ? AND value_fingerprint = ?
			   AND deleted_at IS NULL ORDER BY id LIMIT 1`, string(g.Type), g.fp).Scan(&id, &name)
		if err == nil {
			g.TargetID, g.TargetName = &id, name
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		out = append(out, *g)
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].HostIDs) > len(out[j].HostIDs) })
	return out, nil
}

// ConsolidationResult reports what ApplyHostCredentialConsolidation did.
type ConsolidationResult struct {
	Groups  int `json:"groups"`
	Created int `json:"created"` // new shared credentials (the rest reused one)
	Hosts   int `json:"hosts"`   // hosts now linked
}

// ApplyHostCredentialConsolidation consolidates the groups whose keys are
// given (as returned by the plan), in one transaction: find or create the
// shared credential, grant it to the hosts' entidades, link every host, and
// soft-delete the copies.
func ApplyHostCredentialConsolidation(ctx context.Context, db *sql.DB, enc *database.Encryptor, actorUserID int64, keys []string) (ConsolidationResult, error) {
	var res ConsolidationResult
	plan, err := PlanHostCredentialConsolidation(ctx, db)
	if err != nil {
		return res, err
	}
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()
	actor := ActorContext{UserID: actorUserID, Role: "admin"}

	for _, g := range plan {
		if !want[g.Key] {
			continue
		}
		target := int64(0)
		if g.TargetID != nil {
			target = *g.TargetID
		} else {
			var ct, nonce []byte
			if err := tx.QueryRowContext(ctx, `SELECT payload_ciphertext, payload_nonce FROM secrets WHERE id = ?`, g.secretIDs[0]).Scan(&ct, &nonce); err != nil {
				return res, err
			}
			plain, err := enc.Decrypt(ct, nonce)
			if err != nil {
				return res, fmt.Errorf("decrypt secret %d: %w", g.secretIDs[0], err)
			}
			if target, err = createSharedFrom(ctx, tx, enc, actorUserID, g, plain); err != nil {
				return res, err
			}
			res.Created++
		}
		// Whoever could see one of the hosts can see the credential they share.
		for _, hid := range g.HostIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation)
				SELECT 'secret', ?, entidade_id, 'responsible' FROM asset_entidades
				 WHERE asset_type = 'host' AND asset_id = ? AND relation <> 'global'
				ON CONFLICT DO NOTHING`, target, hid); err != nil {
				return res, err
			}
			if err := LinkHostCredential(ctx, tx, hid, target, g.Type); err != nil {
				return res, err
			}
		}
		for _, sid := range g.secretIDs {
			if _, err := tx.ExecContext(ctx,
				`UPDATE secrets SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, sid); err != nil {
				return res, err
			}
			if err := writeAuditTx(tx, sid, models.SecretAuditActionDelete, actor, nil, consolidateMeta); err != nil {
				return res, err
			}
		}
		res.Groups++
		res.Hosts += len(g.HostIDs)
	}
	return res, tx.Commit()
}

// createSharedFrom stores the group's value as an avulso shared credential
// named after the login user ("deploy", "deploy-2"… when taken).
func createSharedFrom(ctx context.Context, tx *sql.Tx, enc *database.Encryptor, actor int64, g ConsolidationGroup, plain string) (int64, error) {
	base := g.Username
	if base == "" {
		base = "credencial"
	}
	if g.Type == models.SecretTypeSSHKey {
		base += "-key"
	}
	name := base
	for n := 2; ; n++ {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM secrets WHERE scope = 'avulso' AND visibility = 'shared'
			AND name = ? AND COALESCE(group_label, '') = '' AND deleted_at IS NULL)`, name).Scan(&exists); err != nil {
			return 0, err
		}
		if !exists {
			break
		}
		name = fmt.Sprintf("%s-%d", base, n)
	}
	ct, nonce, err := enc.Encrypt(plain)
	if err != nil {
		return 0, err
	}
	id, err := database.InsertReturningID(tx, `
		INSERT INTO secrets (type, scope, visibility, parent_id, owner_user_id, name,
		                     payload_ciphertext, payload_nonce, key_version, created_by)
		VALUES (?, 'avulso', 'shared', NULL, ?, ?, ?, ?, 1, ?)`,
		string(g.Type), actor, name, ct, nonce, actor)
	if err != nil {
		return 0, err
	}
	if err := stampDerived(ctx, tx, enc, id, g.Type, plain); err != nil {
		return 0, err
	}
	if g.Username != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE secrets SET username = COALESCE(username, ?) WHERE id = ?`, g.Username, id); err != nil {
			return 0, err
		}
	}
	return id, writeAuditTx(tx, id, models.SecretAuditActionCreate, ActorContext{UserID: actor, Role: "admin"}, nil, consolidateMeta)
}
