package mig

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestToVersion(t *testing.T) {
	tests := []struct {
		name         string
		oldVersion   int64
		version      int64
		tx           bool
		expectedType int
		expected     []int64
	}{
		{"same version", 1, 1, true, 0, nil},
		{"up", 1, 3, true, migrationUp, []int64{2, 3}},
		{"down", 3, 1, true, migrationDown, []int64{3, 2}},
	}

	migrations = generateMigrations(3)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, cleanup := initTest(t, tt.oldVersion)
			defer cleanup()

			oldVersion, newVersion, err := ToVersion(db, tt.tx, tt.version)
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			}

			if oldVersion != tt.oldVersion {
				t.Errorf("unexpected old version:\n\t(GOT): %d\n\t(WNT): %d", oldVersion, tt.oldVersion)
			}

			if newVersion != tt.version {
				t.Errorf("unexpected version:\n\t(GOT): %d\n\t(WNT): %d", newVersion, tt.version)
			}

			assertMigration(t, tt.expected, tt.expectedType, db)
		})
	}
}

func TestToVersion_NotFound(t *testing.T) {
	migrations = nil
	db, cleanup := initTest(t, 0)
	defer cleanup()

	_, _, err := ToVersion(db, true, 4)
	if err == nil {
		t.Errorf("expecting an error")
	}
}

func TestDown(t *testing.T) {
	migrations = generateMigrations(3)
	db, cleanup := initTest(t, 3)
	defer cleanup()

	oldVersion, newVersion, err := Down(db, true)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if oldVersion != 3 {
		t.Errorf("unexpected old version:\n\t(GOT): %d\n\t(WNT): %d", oldVersion, 3)
	}

	if newVersion != 2 {
		t.Errorf("unexpected version:\n\t(GOT): %d\n\t(WNT): %d", newVersion, 2)
	}

	assertMigration(t, []int64{3}, migrationDown, db)
}

func TestDown_NoMigrations(t *testing.T) {
	migrations = nil
	db, cleanup := initTest(t, 0)
	defer cleanup()

	_, _, err := Down(db, true)
	if err == nil {
		t.Errorf("expecting an error")
	}
}

func TestDown_ErrorMigration(t *testing.T) {
	migrations = []migration{
		{
			2,
			newMigrationFunc(2, migrationUp, fmt.Errorf("err")),
			newMigrationFunc(2, migrationDown, fmt.Errorf("err")),
			"2_test.go",
		},
	}

	tests := []struct {
		tx       bool
		expected []int64
	}{
		{true, nil},
		{false, []int64{2}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("tx:%v", tt.tx), func(t *testing.T) {
			db, cleanup := initTest(t, 2)
			defer cleanup()

			_, _, err := Down(db, tt.tx)
			if err == nil {
				t.Errorf("expected error")
			}

			assertMigration(t, tt.expected, migrationDown, db)
		})
	}
}

func TestUp(t *testing.T) {
	migrations = generateMigrations(3)
	db, cleanup := initTest(t, 0)
	defer cleanup()

	oldVersion, newVersion, err := Up(db, true)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if oldVersion != 0 {
		t.Errorf("unexpected old version:\n\t(GOT): %d\n\t(WNT): %d", oldVersion, 0)
	}

	if newVersion != 3 {
		t.Errorf("unexpected version:\n\t(GOT): %d\n\t(WNT): %d", newVersion, 3)
	}

	assertMigration(t, []int64{1, 2, 3}, migrationUp, db)
}

func TestUp_FromStartpoint(t *testing.T) {
	migrations = generateMigrations(3)
	db, cleanup := initTest(t, 1)
	defer cleanup()

	oldVersion, newVersion, err := Up(db, true)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if oldVersion != 1 {
		t.Errorf("unexpected old version:\n\t(GOT): %d\n\t(WNT): %d", oldVersion, 1)
	}

	if newVersion != 3 {
		t.Errorf("unexpected version:\n\t(GOT): %d\n\t(WNT): %d", newVersion, 3)
	}

	assertMigration(t, []int64{2, 3}, migrationUp, db)
}

func TestUp_NoMigrations(t *testing.T) {
	migrations = nil
	db, cleanup := initTest(t, 0)
	defer cleanup()

	_, _, err := Up(db, true)
	if err == nil {
		t.Errorf("expecting an error")
	}
}

func TestUp_ErrorMigration(t *testing.T) {
	migrations = []migration{
		{
			1,
			newMigrationFunc(1, migrationUp, nil),
			newMigrationFunc(1, migrationDown, nil),
			"1_test.go",
		},
		{
			2,
			newMigrationFunc(2, migrationUp, fmt.Errorf("err")),
			newMigrationFunc(2, migrationDown, fmt.Errorf("err")),
			"2_test.go",
		},
		{
			3,
			newMigrationFunc(3, migrationUp, nil),
			newMigrationFunc(3, migrationDown, nil),
			"3_test.go",
		},
	}

	tests := []struct {
		tx       bool
		expected []int64
	}{
		{true, nil},
		{false, []int64{1, 2}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("tx:%v", tt.tx), func(t *testing.T) {
			db, cleanup := initTest(t, 0)
			defer cleanup()

			_, _, err := Up(db, tt.tx)
			if err == nil {
				t.Errorf("expected error")
			}

			assertMigration(t, tt.expected, migrationUp, db)
		})
	}
}

func generateMigrations(n int64) []migration {
	var migrations = make([]migration, int(n))
	for i := 0; i < int(n); i++ {
		j := int64(i + 1)
		migrations[i] = migration{
			j,
			newMigrationFunc(j, migrationUp, nil),
			newMigrationFunc(j, migrationDown, nil),
			fmt.Sprintf("%d_test.go", j),
		}
	}
	return migrations
}

func newMigrationFunc(version int64, typ int, err error) MigrationFunc {
	return func(db DB) error {
		_, execErr := db.Exec(fmt.Sprintf("INSERT INTO migrations_run (version, migration_type) VALUES (%d, %d)", version, typ))
		if execErr != nil {
			return execErr
		}

		return err
	}
}

func assertMigration(t *testing.T, expected []int64, typ int, db *sql.DB) {
	rows, err := db.Query(fmt.Sprintf("SELECT version FROM migrations_run WHERE migration_type = %d ORDER BY id ASC", typ))
	if err != nil {
		t.Fatalf("unable to retrieve rows: %s", err)
	}

	var result []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("unable to scan version: %s", err)
		}

		result = append(result, v)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("unexpected result:\n\t(GOT): %v\n\t(WNT): %v", result, expected)
	}
}

func TestVersionFromFile(t *testing.T) {
	tests := []struct {
		file    string
		version int64
		ok      bool
	}{
		{"00000_foo.go", 0, true},
		{"00001_foo.go", 1, true},
		{"1_foo.go", 1, true},
		{"00016_foo.go", 16, true},
		{"foo.go", 0, false},
		{"00001_foo.sql", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			v, err := versionFromFile(tt.file)
			if err != nil && tt.ok {
				t.Error("unexpected error")
			} else if err == nil && !tt.ok {
				t.Error("expecting error")
			} else if tt.version != v {
				t.Errorf("unexpected version:\n\t(GOT): %d\n\t(WNT): %d", v, tt.version)
			}
		})
	}
}

const (
	migrationUp   = 0
	migrationDown = 1
)

func initTest(t *testing.T, version int64) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	_, err = db.Exec(`create table migrations_run (
		id integer auto increment,
		version bigint,
		migration_type integer,
		primary key (id)
	)`)
	if err != nil {
		t.Fatalf("unable to create test table: %s", err)
	}

	if err := setup(db); err != nil {
		t.Fatalf("unable to setup db: %s", err)
	}

	if version > 0 {
		if err := SetVersion(db, version); err != nil {
			t.Fatalf("unable to initialize version: %s", err)
		}
	}

	return db, func() {
		if err := db.Close(); err != nil {
			t.Fatalf("can't close db connection: %s", err)
		}
	}
}
