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

func SetTableName(name string) {
	tableName = name
}

type DB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

type MigrationFunc func(DB) error

func Register(up, down MigrationFunc) {
	_, file, _, _ := runtime.Caller(1)
	v, err := versionFromFile(file)
	if err != nil {
		panic(err)
	}

	migrations = append(migrations, migration{
		version: v,
		up:      up,
		down:    down,
	})
}

func sortedMigrations() []migration {
	var m = make([]migration, len(migrations))
	copy(m, migrations)
	sort.Stable(byVersion(m))
	return m
}

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
		}
	}

	if !found {
		return 0, 0, fmt.Errorf("unable to find a migration with version %d", v)
	}

	if v < oldVersion {
		newVersion, err = upTo(db, tx, oldVersion, v)
	} else {
		newVersion, err = downTo(db, tx, oldVersion, v)
	}

	return
}

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

	fn := func(db DB) error {
		for i := len(pendingMigrations) - 1; i >= 0; i++ {
			newVersion = pendingMigrations[i].version
			if err := pendingMigrations[i].down(db); err != nil {
				return fmt.Errorf("error applying migration down %d: %s", newVersion, err)
			}
		}

		return SetVersion(db, newVersion)
	}

	if tx {
		return 0, runTx(db, fn)
	}
	return 0, fn(db)
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

func CurrentVersion(db *sql.DB) (version int64, err error) {
	if err = setup(db); err != nil {
		return
	}

	query := fmt.Sprintf("SELECT version FROM %s ORDER BY created_at DESC", tableName)
	if err = db.QueryRow(query).Scan(&version); err != nil {
		return 0, fmt.Errorf("error checking current version: %s", err)
	}
	return
}

func SetVersion(db DB, v int64) error {
	query := fmt.Sprintf("INSERT INTO %s (version, created_at) VALUES (%d, %d)", tableName, v, time.Now().Unix())
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("error setting version of database to %d: %s", v, err)
	}
	return nil
}

func setup(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version bigint not null,
		created_at bigint not null unique
	)`, tableName))
	if err != nil {
		return fmt.Errorf("unable to create table %s: %s", tableName, err)
	}

	return nil
}

type migration struct {
	version int64
	up      MigrationFunc
	down    MigrationFunc
}

type byVersion []migration

func (m byVersion) Len() int           { return len(m) }
func (m byVersion) Less(i, j int) bool { return m[i].version < m[j].version }
func (m byVersion) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

func versionFromFile(filename string) (int64, error) {
	file := filepath.Base(filename)
	if !strings.HasSuffix(file, ".go") {
		return 0, fmt.Errorf("migration file %s should have .go extension", file)
	}

	if idx := strings.IndexRune(file, '_'); idx >= 0 {
		v, err := strconv.ParseInt(file[:idx], 10, 64)
		if err == nil {
			return v, nil
		}
	}

	return 0, fmt.Errorf("migration file name must be NUMBER_NAME.go, is %s", filename)
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
