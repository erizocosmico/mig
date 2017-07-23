package manager

import (
	"database/sql"
	"strconv"

	cli "gopkg.in/urfave/cli.v1"

	"github.com/erizocosmico/mig"
	"github.com/sirupsen/logrus"
)

// Run executes the manager app.
func Run(dbtype string, args []string) {
	app := cli.NewApp()
	app.Name = "migrate"
	app.Version = "1.0.0"
	app.Usage = "manages migrations"
	app.Commands = []cli.Command{
		{
			Name:   "up",
			Usage:  "executes all the pending migrations",
			Flags:  defaultFlags,
			Action: up(dbtype),
		},
		{
			Name:   "rollback",
			Usage:  "rollbacks just one migration",
			Flags:  defaultFlags,
			Action: rollback(dbtype),
		},
		{
			Name:   "to-version",
			Usage:  "executes all the migrations (either up or down) until the database is at the desired version",
			Flags:  defaultFlags,
			Action: toVersion(dbtype),
		},
	}

	app.Run(args)
}

var defaultFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "url, u",
		Usage: "url of the database e.g. `postgres://user:pass@0.0.0.0:5432/database`",
	},
	cli.BoolFlag{
		Name:  "no-tx",
		Usage: "if given, all the migrations won't be run in a single transaction",
	},
}

func flags(ctx *cli.Context, dbtype string) (*sql.DB, bool) {
	dburl := ctx.String("url")
	notx := ctx.Bool("no-tx")

	db, err := sql.Open(dbtype, dburl)
	if err != nil {
		logrus.Fatalf("unable to open a database connection: %s", err)
	}

	return db, !notx
}

func up(dbtype string) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		db, tx := flags(ctx, dbtype)
		report(mig.Up(db, tx))
		return nil
	}
}

func rollback(dbtype string) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		db, tx := flags(ctx, dbtype)
		report(mig.Down(db, tx))
		return nil
	}
}

func toVersion(dbtype string) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		v, err := strconv.ParseInt(ctx.Args().First(), 10, 64)
		if err != nil {
			logrus.Fatalf("given version %s is not a valid number", ctx.Args().First())
		}

		db, tx := flags(ctx, dbtype)
		report(mig.ToVersion(db, tx, v))
		return nil
	}
}

func report(oldVersion, newVersion int64, err error) {
	if err != nil {
		logrus.Fatal(err)
	}

	if oldVersion == newVersion {
		logrus.Warnf("no migrations executed, database is at the same version: %d", oldVersion)
	} else {
		logrus.WithFields(logrus.Fields{
			"old": oldVersion,
			"new": newVersion,
		}).Info("database migrated correctly")
	}
}
