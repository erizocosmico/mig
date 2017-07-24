package main

import (
	"fmt"
	"go/build"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/erizocosmico/mig"
	"github.com/sirupsen/logrus"

	cli "gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "mig"
	app.Version = "1.0.0"
	app.Usage = "manages migrations for the mig library"
	app.Commands = commands
	app.Action = cli.ShowAppHelp

	app.Run(os.Args)
}

var commands = []cli.Command{
	{
		Name:      "new",
		Usage:     "creates a new migration",
		ArgsUsage: "[name of the migration file]",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "folder, f",
				Value: "migrations",
				Usage: "migrations folder path",
			},
		},
		Action: create,
	},
	{
		Name:  "scaffold",
		Usage: "generates a command to manage migrations using mig",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "database, db",
				Usage: "database system to use, one of (postgres, mysql, mssql, sqlite3)",
			},
			cli.StringFlag{
				Name:  "cmdfile, f",
				Value: "./cmd/migrate/main.go",
				Usage: "path where the command file will be written",
			},
			cli.StringFlag{
				Name:  "package, p",
				Value: "",
				Usage: "name of the package where your migrations are. If it is not provided, the folder `migrations` at the root of the current project will be used",
			},
		},
		Action: scaffold,
	},
}

var filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func create(ctx *cli.Context) error {
	filename := ctx.Args().First()
	if !filenameRegex.MatchString(filename) {
		logrus.Fatalf("invalid file name: %s", filename)
	}

	file, err := mig.Create(ctx.String("folder"), filename)
	if err != nil {
		logrus.Error(err.Error())
	} else {
		logrus.Infof("created migration file: %s", file)
	}

	return nil
}

func scaffold(ctx *cli.Context) error {
	var (
		pkg  = ctx.String("package")
		db   = ctx.String("database")
		file = ctx.String("cmdfile")
	)

	if pkg == "" {
		logrus.Warn("--package flag was not given, trying to find migrations in ./migrations")
		var err error
		pkg, err = defaultPkg()
		if err != nil {
			logrus.Fatal(err)
		}
	}

	driver, ok := dbDrivers[db]
	if !ok {
		logrus.Fatalf("unknown database type %s", db)
	}

	if _, err := os.Stat(file); err != nil && !os.IsNotExist(err) {
		logrus.Fatalf("unknown error checking `cmdfile`: %s", err)
	} else if err == nil {
		logrus.Fatalf("provided `cmdfile` %q already exists", file)
	}

	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		logrus.Fatalf("unable to create directories: %s", err)
	}

	f, err := os.Create(file)
	if err != nil {
		logrus.Fatalf("error creating cmd file %q: %s", file, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			logrus.Errorf("unable to close file %q: %s", file, err)
		}
	}()

	content, err := renderCmdFileTpl(db, driver, pkg)
	if err != nil {
		logrus.Fatalf("error rendering template file: %s", err)
	}

	_, err = f.Write(content)
	if err != nil {
		logrus.Fatalf("unable to write file at %q: %s", file, err)
	}

	logrus.Infof("successfully created command file at %q", file)

	return nil
}

var dbDrivers = map[string]string{
	"mysql":    "github.com/go-sql-driver/mysql",
	"postgres": "github.com/lib/pq",
	"sqlite3":  "github.com/mattn/go-sqlite3",
	"mssql":    "github.com/denisenkom/go-mssqldb",
}

func defaultPkg() (pkg string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error generating scaffold: unable to get working directory: %s", err)
	}

	for _, d := range build.Default.SrcDirs() {
		if strings.HasPrefix(wd, d) {
			dir := strings.TrimPrefix(filepath.ToSlash(strings.Replace(wd, d, "", -1)), "/")
			pkg = filepath.Join(dir, "migrations")

			if fi, err := os.Stat(filepath.Join(wd, "migrations")); os.IsNotExist(err) {
				return "", fmt.Errorf("unable to find a valid migrations directory at %s", pkg)
			} else if err != nil {
				return "", err
			} else if !fi.IsDir() {
				return "", fmt.Errorf("%s exists but is not a directory", filepath.Join(wd, "migrations"))
			}

			break
		}
	}

	if pkg == "" {
		return "", fmt.Errorf("you need to provide the --package flag with the path to your migrations directory or create a `migrations` directory in the current directory")
	}

	return pkg, nil
}

const cmdfileTpl = `package main

import (
	"os"

	_ "%s"
	_ "%s"
	"github.com/erizocosmico/mig/manager"
	"github.com/sirupsen/logrus"
)

func main() {
	manager.Run("%s", os.Args)
}
`

func renderCmdFileTpl(db, driver, pkg string) ([]byte, error) {
	file := fmt.Sprintf(
		cmdfileTpl,
		driver, pkg, db,
	)

	return format.Source([]byte(file))
}
