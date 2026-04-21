package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type backupHandlers struct {
	db *database.DB
}

// handleBackup streams a portable backup (gzipped JSON) to the client.
// The same format is produced regardless of whether the source is SQLite
// or Postgres, so backups are cross-dialect by construction: a file
// downloaded from a SQLite deployment can be restored into a Postgres
// deployment and vice versa.
func (h *backupHandlers) handleBackup(w http.ResponseWriter, r *http.Request) {
	name := fmt.Sprintf("sshcm_backup_%s.sshcmbak", time.Now().Format("2006-01-02_150405"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))

	if err := h.db.WritePortableBackup(w); err != nil {
		// Body is already being streamed so we can't rewrite the status.
		// Log and bail; the client will see a truncated gzip stream.
		fmt.Printf("backup stream error: %v\n", err)
	}
}

// handleRestore ingests a portable backup and replaces all user-data
// tables on the active database. Accepts the new .sshcmbak format
// regardless of which dialect produced it.
func (h *backupHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 500*1024*1024)
	if err := r.ParseMultipartForm(64 * 1024 * 1024); err != nil {
		jsonBadRequest(w, r, "file too large (max 500MB)", err)
		return
	}
	file, _, err := r.FormFile("backup")
	if err != nil {
		jsonBadRequest(w, r, "missing backup file", err)
		return
	}
	defer file.Close()

	backup, err := database.ReadPortableBackup(file)
	if err != nil {
		jsonBadRequest(w, r, fmt.Sprintf("invalid backup: %v", err), err)
		return
	}

	rowCount := 0
	for _, rows := range backup.Tables {
		rowCount += len(rows)
	}

	if err := h.db.RestorePortable(backup); err != nil {
		jsonServerError(w, r, fmt.Sprintf("restore failed: %v", err), err)
		return
	}

	jsonOK(w, map[string]any{
		"status":          "restored",
		"source_dialect":  backup.SourceDialect,
		"target_dialect":  dialectString(h.db.Dialect),
		"schema_version":  backup.SchemaVersion,
		"row_count":       rowCount,
		"cross_dialect":   backup.SourceDialect != dialectString(h.db.Dialect),
		"message":         "Database restored. Sessions may need to log in again.",
	})
}

func dialectString(k database.DialectKind) string {
	if k == database.DialectPostgres {
		return "postgres"
	}
	return "sqlite"
}
