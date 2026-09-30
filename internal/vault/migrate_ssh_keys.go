package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// migrate_ssh_keys.go — "Credenciais de host" (the ssh_keys library) become
// vault credentials: each row turns into an avulso, shared secret — an sshkey
// for a key pair, a password for a password, both when a row carries both —
// keeping its name, description, username and entidade grants. Hosts that
// referenced the row through host_remote_users.ssh_key_id now link the new
// secret. Runs at startup; idempotent through secrets.legacy_ssh_key_id.

var migrateSSHKeysMeta = []byte(`{"migration":"ssh_keys_v91"}`)

// SSHKeyMigrationStats reports what MigrateSSHKeysToVault did.
type SSHKeyMigrationStats struct {
	Created int // new secrets
	Reused  int // same-named secret with the same value, adopted
	Renamed int // created under a suffixed name (name taken by another value)
	Links   int // host_remote_users rows pointed at a migrated secret
}

type legacyKey struct {
	id                int64
	name, user, desc  string
	pubCT, pubNonce   []byte
	privCT, privNonce []byte
	passCT, passNonce []byte
}

// MigrateSSHKeysToVault copies every ssh_keys row not yet migrated into the
// vault, in one transaction.
func MigrateSSHKeysToVault(ctx context.Context, db *sql.DB, enc *database.Encryptor, actorUserID int64) (SSHKeyMigrationStats, error) {
	var st SSHKeyMigrationStats
	if actorUserID <= 0 || !legacyTableExists(ctx, db) {
		return st, nil // no users yet (next boot retries), or already dropped
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, COALESCE(username, ''), COALESCE(description, ''),
		        pub_key_ciphertext, pub_key_nonce, priv_key_ciphertext, priv_key_nonce,
		        password_ciphertext, password_nonce
		   FROM ssh_keys k
		  WHERE NOT EXISTS (SELECT 1 FROM secrets s WHERE s.legacy_ssh_key_id = k.id)
		  ORDER BY id`)
	if err != nil {
		return st, err
	}
	var keys []legacyKey
	for rows.Next() {
		var k legacyKey
		if err := rows.Scan(&k.id, &k.name, &k.user, &k.desc, &k.pubCT, &k.pubNonce,
			&k.privCT, &k.privNonce, &k.passCT, &k.passNonce); err != nil {
			rows.Close()
			return st, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(keys) == 0 {
		return st, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return st, err
	}
	defer tx.Rollback()

	for _, k := range keys {
		dec := func(ct, nonce []byte) (string, error) {
			if len(ct) == 0 {
				return "", nil
			}
			return enc.Decrypt(ct, nonce)
		}
		pub, err := dec(k.pubCT, k.pubNonce)
		if err != nil {
			return st, fmt.Errorf("ssh_key %d public key: %w", k.id, err)
		}
		priv, err := dec(k.privCT, k.privNonce)
		if err != nil {
			return st, fmt.Errorf("ssh_key %d private key: %w", k.id, err)
		}
		pass, err := dec(k.passCT, k.passNonce)
		if err != nil {
			return st, fmt.Errorf("ssh_key %d password: %w", k.id, err)
		}

		type item struct {
			typ   models.SecretType
			name  string
			plain string
		}
		var items []item
		if priv != "" || pub != "" {
			payload, _ := json.Marshal(HostSSHKey{Username: k.user, PrivateKeyPEM: priv, PublicKey: pub})
			items = append(items, item{models.SecretTypeSSHKey, k.name, NormalizeSSHKeyPayload(string(payload))})
		}
		if pass != "" {
			// Shared names are unique per scope whatever the type.
			name := k.name
			if len(items) > 0 {
				name = k.name + " (senha)"
			}
			items = append(items, item{models.SecretTypePassword, name, pass})
		}

		for _, it := range items {
			id, how, err := migrateOneKey(ctx, tx, enc, actorUserID, k, it.typ, it.name, it.plain)
			if err != nil {
				return st, fmt.Errorf("ssh_key %d (%s): %w", k.id, it.typ, err)
			}
			switch how {
			case "reused":
				st.Reused++
			case "renamed":
				st.Renamed++
				st.Created++
			default:
				st.Created++
			}
			col := "secret_id"
			if it.typ == models.SecretTypeSSHKey {
				col = "key_secret_id"
			}
			res, err := tx.ExecContext(ctx,
				`UPDATE host_remote_users SET `+col+` = ?, updated_at = CURRENT_TIMESTAMP
				  WHERE ssh_key_id = ? AND `+col+` IS NULL`, id, k.id)
			if err != nil {
				return st, err
			}
			n, _ := res.RowsAffected()
			st.Links += int(n)
		}
	}
	if err := tx.Commit(); err != nil {
		return st, err
	}
	return st, nil
}

// migrateOneKey finds or creates the vault secret for one part of a legacy
// row. A live shared avulso secret with the same name and the same value is
// adopted; a name held by a different value gets a "-2", "-3"… suffix.
func migrateOneKey(ctx context.Context, tx *sql.Tx, enc *database.Encryptor, actor int64, k legacyKey,
	typ models.SecretType, name, plain string) (int64, string, error) {
	d, err := deriveCols(enc, typ, plain)
	if err != nil {
		return 0, "", err
	}
	how := "created"
	for n := 1; ; n++ {
		candidate := name
		if n > 1 {
			candidate = fmt.Sprintf("%s-%d", name, n)
		}
		var id int64
		var fp []byte
		var etyp string
		err := tx.QueryRowContext(ctx,
			`SELECT id, type, value_fingerprint FROM secrets
			  WHERE scope = 'avulso' AND visibility = 'shared' AND name = ?
			    AND COALESCE(group_label, '') = '' AND deleted_at IS NULL`, candidate).Scan(&id, &etyp, &fp)
		if err == nil {
			if etyp == string(typ) && fp != nil && string(fp) == string(d.valueFP) {
				if _, err := tx.ExecContext(ctx, `UPDATE secrets SET legacy_ssh_key_id = ? WHERE id = ?`, k.id, id); err != nil {
					return 0, "", err
				}
				return id, "reused", nil
			}
			how = "renamed"
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, "", err
		}
		if how == "renamed" {
			log.Printf("[vault] ssh_key %d %q migrated as %q (name taken by another value)", k.id, name, candidate)
		}
		ct, nonce, err := enc.Encrypt(plain)
		if err != nil {
			return 0, "", err
		}
		var desc *string
		if k.desc != "" {
			desc = &k.desc
		}
		id, err = database.InsertReturningID(tx,
			`INSERT INTO secrets
			   (type, scope, visibility, parent_id, owner_user_id, name, description,
			    payload_ciphertext, payload_nonce, key_version, created_by, legacy_ssh_key_id)
			 VALUES (?, 'avulso', 'shared', NULL, ?, ?, ?, ?, ?, 1, ?, ?)`,
			string(typ), actor, candidate, desc, ct, nonce, actor, k.id)
		if err != nil {
			return 0, "", err
		}
		if err := stampDerived(ctx, tx, enc, id, typ, plain); err != nil {
			return 0, "", err
		}
		if k.user != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE secrets SET username = ? WHERE id = ?`, k.user, id); err != nil {
				return 0, "", err
			}
		}
		// Same audience as before: the key's entidade grants (none = admin-only).
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation)
			 SELECT 'secret', ?, entidade_id, relation FROM asset_entidades
			  WHERE asset_type = 'ssh_key' AND asset_id = ?
			 ON CONFLICT DO NOTHING`, id, k.id); err != nil {
			return 0, "", err
		}
		if err := writeAuditTx(tx, id, models.SecretAuditActionCreate,
			ActorContext{UserID: actor, Role: "admin"}, nil, migrateSSHKeysMeta); err != nil {
			return 0, "", err
		}
		return id, how, nil
	}
}

// legacyTableExists reports whether ssh_keys is still there.
func legacyTableExists(ctx context.Context, db *sql.DB) bool {
	var exists bool
	_ = db.QueryRowContext(ctx, `SELECT to_regclass('ssh_keys') IS NOT NULL`).Scan(&exists)
	return exists
}

// DropLegacySSHKeys removes the old library once every row holding a key or
// password has a vault copy: the ssh_keys table, host_remote_users.ssh_key_id
// and the library's entidade grants. Returns false (nothing dropped) while
// any row is left to migrate. The mapping for old clients lives on in
// secrets.legacy_ssh_key_id.
func DropLegacySSHKeys(ctx context.Context, db *sql.DB) (bool, error) {
	if !legacyTableExists(ctx, db) {
		return false, nil
	}
	var pending int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ssh_keys k
		  WHERE (length(k.priv_key_ciphertext) > 0 OR length(k.pub_key_ciphertext) > 0 OR length(k.password_ciphertext) > 0)
		    AND NOT EXISTS (SELECT 1 FROM secrets s WHERE s.legacy_ssh_key_id = k.id)`).Scan(&pending); err != nil {
		return false, err
	}
	if pending > 0 {
		return false, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM asset_entidades WHERE asset_type = 'ssh_key'`,
		`ALTER TABLE host_remote_users DROP COLUMN IF EXISTS ssh_key_id`,
		`DROP TABLE IF EXISTS ssh_keys`,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// RunSSHKeyMigration is the startup hook: stamp derived columns, migrate the
// old library, then drop it once nothing is left behind. Failures are
// logged, not fatal — hosts keep resolving through their per-host copies.
func RunSSHKeyMigration(ctx context.Context, db *database.DB) {
	// Stamp first: adopting a same-named secret compares value fingerprints.
	if n, err := BackfillDerived(ctx, db.SQL, db.Encryptor); err != nil {
		log.Printf("[vault] backfill derived columns: %v", err)
	} else if n > 0 {
		log.Printf("[vault] stamped %d secret(s) with derived columns", n)
	}
	actor := resolveSystemActor(ctx, db)
	st, err := MigrateSSHKeysToVault(ctx, db.SQL, db.Encryptor, actor)
	if err != nil {
		log.Printf("[vault] migrate ssh_keys: %v", err)
		return
	}
	if st.Created+st.Reused > 0 {
		log.Printf("[vault] ssh_keys → vault: %d created (%d renamed), %d reused, %d host links", st.Created, st.Renamed, st.Reused, st.Links)
	}
	if dropped, err := DropLegacySSHKeys(ctx, db.SQL); err != nil {
		log.Printf("[vault] drop legacy ssh_keys: %v", err)
	} else if dropped {
		log.Printf("[vault] legacy ssh_keys table dropped (all rows live in the vault)")
	}
}
