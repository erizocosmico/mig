package main

import (
	"os"
	"regexp"
	"strconv"

	"github.com/erizocosmico/mig"
	"github.com/sirupsen/logrus"

	cli "gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "mig"
	app.Usage = "manages migrations for Go"
	app.Commands = commands

	app.Run(os.Args)
}

var dbFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "url, u",
		Usage: "url to connect to the database",
	},
	cli.BoolFlag{
		Name:  "no-transaction, notx",
		Usage: "don't run all the migrations inside a transaction",
	},
}

var commands = []cli.Command{
	{
		Name:   "up",
		Usage:  "migrates the database all the way up running all pending migrations",
		Flags:  dbFlags,
		Action: up,
	},
	{
		Name:   "down",
		Flags:  dbFlags,
		Usage:  "rollbacks the database to the previous version running the rollback of the current database version",
		Action: down,
	},
	{
		Name:      "create",
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
		Name:   "to-version",
		Usage:  "upgrades or rolls back a database to the given version",
		Flags:  dbFlags,
		Action: toVersion,
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

func up(ctx *cli.Context) error {
	return nil
}

func down(ctx *cli.Context) error {
	return nil
}

func toVersion(ctx *cli.Context) error {
	version, err := strconv.ParseInt(ctx.Args().First(), 10, 64)
	if err != nil {
		logrus.Fatalf("invalid version given: %s", ctx.Args().First())
	}

	oldVersion, newVersion, err := mig.ToVersion(db, tx, v)
	if err != nil {
		logrus.Error(err)
		return nil
	}

	if oldVersion == newVersion {
		logrus.Infof("no migrations to apply, current version is: %d", oldVersion)
	} else {
		logrus.Infof("migrations run successfully, current version is: %d", newVersion)
	}

	return nil
}
