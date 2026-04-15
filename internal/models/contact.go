package models

import (
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type Contact struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
	Entity string `json:"entity"`
}

func ListContacts(db *sql.DB) ([]Contact, error) {
	rows, err := db.Query(`SELECT id, name, phone, role, entity FROM contacts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Role, &c.Entity); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func CreateContact(db *sql.DB, c *Contact) error {
	// Upsert: if (name, phone) already exists, return that row's id so
	// callers can reliably continue. The dummy `SET name = EXCLUDED.name`
	// is portable between SQLite and Postgres and guarantees RETURNING id
	// yields a row even on conflict.
	id, err := database.InsertReturningID(db,
		`INSERT INTO contacts (name, phone, role, entity) VALUES (?, ?, ?, ?)
		 ON CONFLICT(name, phone) DO UPDATE SET name = EXCLUDED.name`,
		c.Name, c.Phone, c.Role, c.Entity,
	)
	if err != nil {
		return err
	}
	c.ID = id
	return nil
}

func UpdateContact(db *sql.DB, c *Contact) error {
	_, err := db.Exec(
		`UPDATE contacts SET name = ?, phone = ?, role = ?, entity = ? WHERE id = ?`,
		c.Name, c.Phone, c.Role, c.Entity, c.ID,
	)
	return err
}

func DeleteContact(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM contacts WHERE id = ?`, id)
	return err
}

// EnsureContact creates a contact if it doesn't exist, or updates role/entity if it does.
func EnsureContact(db *sql.DB, name, phone, role, entity string) error {
	_, err := db.Exec(
		`INSERT INTO contacts (name, phone, role, entity) VALUES (?, ?, ?, ?)
		 ON CONFLICT(name, phone) DO UPDATE SET role = excluded.role, entity = excluded.entity`,
		name, phone, role, entity,
	)
	return err
}
