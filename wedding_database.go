package main

import (
	"database/sql"
)

// RsvpDatabase provides thread-safe access to a database of Rsvps.
type WeddingDatabase interface {
	// GetRsvp retrieves a Rsvp by its ID.
	GetRsvp(id string) (*Rsvp, error)

	// UpdateRsvp updates the entry for a given Rsvp.
	UpdateRsvp(r *Rsvp) error

	// Close closes the database, freeing up any available resources.
	// TODO(cbro): Close() should return an error.
	Close()

	Exec(statement string) (sql.Result, error)

	DB() *sql.DB
}
