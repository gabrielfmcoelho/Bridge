package models

import (
	"database/sql"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

// ReconcileContainerServices matches discovered containers against existing
// auto/fixed services for a host. New containers create auto services; missing
// containers mark existing services offline.
func ReconcileContainerServices(db *sql.DB, hostID int64, containers []sshtest.ContainerInfo) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Load existing container-linked services for this host.
	existing, err := ListContainerServicesByHost(db, hostID)
	if err != nil {
		return err
	}

	// Build lookup: container_name -> *Service
	byName := make(map[string]*Service, len(existing))
	for i := range existing {
		byName[existing[i].ContainerName] = &existing[i]
	}

	now := time.Now()
	seen := make(map[string]bool) // track which existing services matched

	for _, c := range containers {
		if c.Name == "" {
			continue
		}

		if svc, ok := byName[c.Name]; ok {
			// Update existing service
			seen[c.Name] = true
			_, err := tx.Exec(
				`UPDATE services SET container_id = ?, container_image = ?, container_ports = ?,
					container_status = 'online', last_seen_at = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ?`,
				c.ID, c.Image, c.Ports, now, svc.ID,
			)
			if err != nil {
				return err
			}
			continue
		}

		// New container — create auto service.
		inf := sshtest.InferFromImage(c.Image, c.Name)

		// Extract first host port from port mapping for the service port field.
		port := extractFirstHostPort(c.Ports)

		var id int64
		err := tx.QueryRow(
			`INSERT INTO services (nickname, description, service_type, service_subtype,
				source, container_status, container_id, container_name, container_image, container_ports,
				port, orchestrator_managed, discovered_at, last_seen_at)
			VALUES (?, ?, ?, ?, 'auto', 'online', ?, ?, ?, ?, ?, 1, ?, ?)
			RETURNING id`,
			inf.Nickname, "Auto-discovered from container "+c.Name,
			inf.ServiceType, inf.ServiceSubtype,
			c.ID, c.Name, c.Image, c.Ports,
			port, now, now,
		).Scan(&id)
		if err != nil {
			return err
		}

		// Link service to host.
		if _, err := tx.Exec(
			`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`,
			id, hostID,
		); err != nil {
			return err
		}

		seen[c.Name] = true
	}

	// 3. Mark unseen auto/fixed services as offline.
	for _, svc := range existing {
		if seen[svc.ContainerName] {
			continue
		}
		if svc.ContainerStatus == "offline" {
			continue // already offline
		}
		if _, err := tx.Exec(
			`UPDATE services SET container_status = 'offline', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			svc.ID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// extractFirstHostPort pulls the first host-mapped port from a Docker ports
// string like "0.0.0.0:8080->80/tcp, 5432/tcp".
func extractFirstHostPort(ports string) string {
	for _, part := range strings.Split(ports, ",") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "->"); idx > 0 {
			hostPart := part[:idx]
			if colonIdx := strings.LastIndex(hostPart, ":"); colonIdx >= 0 {
				return hostPart[colonIdx+1:]
			}
			return hostPart
		}
	}
	return ""
}
