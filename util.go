package mig

import "fmt"

// ExecAll is an utility function to execute all the given migrations.
// This is specially useful for running a bunch of create tables and so on.
func ExecAll(db DB, stmts ...string) error {
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// DropAll is an utility function to drop all tables in the given order.
func DropAll(db DB, tables ...string) error {
	for _, t := range tables {
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE %s", t)); err != nil {
			return err
		}
	}
	return nil
}
