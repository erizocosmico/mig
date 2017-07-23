package mig

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	migrations []migration
	tableName  = "__version"
)

// SetTableName sets the name of the table used to store the migrations
// information in your database.
func SetTableName(name string) {
	tableName = name
}

// DB is an interface that both a database instance and a transaction satisfy.
// It should be able to execute and perform queries.
type DB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// MigrationFunc is a function that receives a database instance and runs a
// migration, either an up or a down.
type MigrationFunc func(DB) error

// Register adds a new migration. Its order will depend on the name of the file
// calling this function. For example, a file named 00001_initial_migration.go
// will be executed before a migration defined in 000004_add_users_table.go.
// Register needs to provide both an up and a down function.
func Register(up, down MigrationFunc) {
	if up == nil || down == nil {
		panic(fmt.Errorf("migrations cannot be nil in register"))
	}

	_, file, _, _ := runtime.Caller(1)
	file = filepath.Base(file)
	v, err := versionFromFile(file)
	if err != nil {
		panic(err)
	}

	if v < 0 {
		panic(fmt.Errorf("version %d in file %q is not valid, it must be bigger than 0", v, file))
	}

	for _, m := range migrations {
		if m.version == v {
			panic(fmt.Errorf("migration with number %d has already been registered in file %s", v, m.file))
		}
	}

	migrations = append(migrations, migration{
		version: v,
		up:      up,
		down:    down,
		file:    file,
	})
}

func sortedMigrations() []migration {
	var m = make([]migration, len(migrations))
	copy(m, migrations)
	sort.Stable(byVersion(m))
	return m
}

// Create creates a new migration file.
func Create(path, name string) (string, error) {
	if path == "" {
		path = "migrations"
	}

	dir, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("unable to get absolute path of migrations dir: %s", path)
	}

	if fi, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0755); err != nil {
			return "", fmt.Errorf("unable to create migrations directory at %s: %s", dir, err)
		}
	} else {
		if !fi.IsDir() {
			return "", fmt.Errorf("migrations directory path %s already exists but it's not a directory", dir)
		}
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return "", fmt.Errorf("unable to get list of migrations directory files: %s", err)
	}

	var migrations []migration
	for _, m := range matches {
		if v, err := versionFromFile(m); err == nil {
			migrations = append(migrations, migration{version: v})
		}
	}

	sort.Stable(byVersion(migrations))
	var lastVersion int64
	if len(migrations) > 0 {
		lastVersion = migrations[len(migrations)-1].version
	}

	filename := fmt.Sprintf("%04d_%s.go", lastVersion+1, name)
	if err := ioutil.WriteFile(filepath.Join(dir, filename), []byte(migrationTpl), 0755); err != nil {
		return "", fmt.Errorf("unable to create migration file: %s", err)
	}

	return filename, nil
}

// ToVersion executes up or down migrations from the current version until the
// target version.
// If tx is true, all migrations will be run inside a transaction.
func ToVersion(db *sql.DB, tx bool, v int64) (oldVersion, newVersion int64, err error) {
	oldVersion, err = CurrentVersion(db)
	if err != nil {
		return
	}

	if oldVersion == v {
		return v, v, nil
	}

	var found bool
	for _, m := range migrations {
		if m.version == v {
			found = true
			break
		}
	}

	if !found {
		return 0, 0, fmt.Errorf("unable to find a migration with version %d", v)
	}

	if v > oldVersion {
		newVersion, err = upTo(db, tx, oldVersion, v)
	} else {
		newVersion, err = downTo(db, tx, oldVersion, v)
	}

	return
}

// Up runs all the pending database migrations until it's up to date.
// If tx is true, all migrations will be run inside a transaction.
func Up(db *sql.DB, tx bool) (oldVersion, newVersion int64, err error) {
	oldVersion, err = CurrentVersion(db)
	if err != nil {
		return
	}

	newVersion, err = upTo(db, tx, oldVersion, math.MaxInt64)
	return
}

func upTo(db *sql.DB, tx bool, oldVersion, target int64) (newVersion int64, err error) {
	migrations := sortedMigrations()
	var pendingMigrations []migration
	for _, m := range migrations {
		if m.version > oldVersion && m.version <= target {
			pendingMigrations = append(pendingMigrations, m)
		}
	}

	if len(pendingMigrations) == 0 {
		return 0, fmt.Errorf("no transactions to run")
	}

	fn := func(db DB) error {
		for _, m := range pendingMigrations {
			newVersion = m.version
			if err := m.up(db); err != nil {
				return fmt.Errorf("error applying migration up %d: %s", m.version, err)
			}
		}

		return SetVersion(db, newVersion)
	}

	if tx {
		return newVersion, runTx(db, fn)
	}
	return newVersion, fn(db)
}

// Down rolls back a single database migration.
// If tx is true, all migrations will be run inside a transaction.
func Down(db *sql.DB, tx bool) (oldVersion, newVersion int64, err error) {
	oldVersion, err = CurrentVersion(db)
	if err != nil {
		return 0, 0, err
	}

	newVersion, err = downTo(db, tx, oldVersion, oldVersion-1)
	return
}

func downTo(db *sql.DB, tx bool, oldVersion, target int64) (newVersion int64, err error) {
	migrations := sortedMigrations()
	var pendingMigrations []migration
	for i := len(migrations) - 1; i >= 0; i-- {
		version := migrations[i].version
		if version <= oldVersion && version > target {
			pendingMigrations = append(pendingMigrations, migrations[i])
		}
	}

	if len(pendingMigrations) == 0 {
		return 0, fmt.Errorf("no transactions to run")
	}

	fn := func(db DB) error {
		for _, m := range pendingMigrations {
			newVersion = m.version
			if err := m.down(db); err != nil {
				return fmt.Errorf("error applying migration down %d: %s", newVersion, err)
			}
		}

		return SetVersion(db, newVersion)
	}

	if tx {
		return newVersion - 1, runTx(db, fn)
	}
	return newVersion - 1, fn(db)
}

func runTx(db *sql.DB, fn func(DB) error) (err error) {
	var tx *sql.Tx
	tx, err = db.Begin()
	if err != nil {
		return fmt.Errorf("unable to start transaction: %s", err)
	}

	if err := fn(tx); err != nil {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("unable to rollback: %s", err)
		}

		return fmt.Errorf("transaction was rolled back: %s", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("unable to commit transaction: %s", err)
	}

	return nil
}

// CurrentVersion returns the current version of the database.
func CurrentVersion(db *sql.DB) (version int64, err error) {
	if err = setup(db); err != nil {
		return
	}

	query := fmt.Sprintf("SELECT version FROM %s ORDER BY updated_at DESC", tableName)
	err = db.QueryRow(query).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("error checking current version: %s", err)
	}

	return
}

// SetVersion sets the current version of the database to the given version.
func SetVersion(db DB, v int64) error {
	query := fmt.Sprintf("INSERT INTO %s (version, updated_at) VALUES (%d, %d)", tableName, v, time.Now().Unix())
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("error setting version of database to %d: %s", v, err)
	}
	return nil
}

const migrationsTableSQL = `
CREATE TABLE IF NOT EXISTS %s (
	version bigint not null,
	updated_at bigint not null
)
`

func setup(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf(migrationsTableSQL, tableName))
	if err != nil {
		return fmt.Errorf("unable to create table %s: %s", tableName, err)
	}

	return nil
}

type migration struct {
	version int64
	up      MigrationFunc
	down    MigrationFunc
	file    string
}

type byVersion []migration

func (m byVersion) Len() int           { return len(m) }
func (m byVersion) Less(i, j int) bool { return m[i].version < m[j].version }
func (m byVersion) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

func versionFromFile(file string) (int64, error) {
	if !strings.HasSuffix(file, ".go") {
		return 0, fmt.Errorf("migration file %s should have .go extension", file)
	}

	if idx := strings.IndexRune(file, '_'); idx >= 0 {
		v, err := strconv.ParseInt(file[:idx], 10, 64)
		if err == nil {
			return v, nil
		}
	}

	return 0, fmt.Errorf("migration file name must be NUMBER_NAME.go, is %s", file)
}

const migrationTpl = `package migrations

import "github.com/erizocosmico/mig"

func init() {
	mig.Register(
		func(db mig.DB) error {
			_, err := db.Exec("UP")
			return err
		},
		func (db mig.DB) error {
			_, err := db.Exec("DOWN")
			return err
		},
	)
}
`
