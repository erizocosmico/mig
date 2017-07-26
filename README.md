# ![mig](https://cdn.rawgit.com/erizocosmico/mig/d0481b14/mig.svg)

[![Build Status](https://travis-ci.org/erizocosmico/mig.svg?branch=master)](https://travis-ci.org/erizocosmico/mig) [![GoDoc](https://godoc.org/github.com/erizocosmico/mig?status.svg)](https://godoc.org/github.com/erizocosmico/mig) [![codecov](https://codecov.io/gh/erizocosmico/mig/branch/master/graph/badge.svg)](https://codecov.io/gh/erizocosmico/mig) [![Go Report Card](https://goreportcard.com/badge/github.com/erizocosmico/mig)](https://goreportcard.com/report/github.com/erizocosmico/mig)

Dead simple Go migration tool and library that keeps your migrations inside a binary (or your application's own binary) for ease of use.

## Install

```
go get -v github.com/erizocosmico/mig/...
```

## Get started

First thing we should do is the following:

* Go to the root of your project
* Run `mkdir migrations`
* Run `mig scaffold --db postgres` (you can change postgres for any of the supported database drivers)

By now we'll have something like this:

```
| myproject/
    |- migrations/
    |- cmd/
        |- migrate/
            |- main.go
```

That `migrate` command is the binary we'll use to manage our migrations. Note that we didn't have to configure anything in the scaffold command other than the database because we used the default `./migrations` as the package for our migrations.

Now we can start writing our migrations.

```
mig new initial_schema
mig new add_sessions_table
mig new add_profile_picture_column
```

That command will add new migration files inside the `migrations` directory.

You can edit them and place your migrations. It's Go code, so you can do whatever thing you want in there.

Now, to execute you can run the generated command or build it and use it as a binary.

```
go ./cmd/migrate/main.go --help
```

or (assuming your `~/$GOPATH/bin` is in your PATH)

```
go install ./cmd/migrate/...
migrate --help
```

There are 3 commands available in the migration manager:

* `up` runs all the migrations.
* `rollback` executes the down for the current version, leaving the database in the previous state e.g. if database is in version 3, this would get it to version 2.
* `to-version` get the database to a specific version.

```
migrate up --url postgres://postgres:@0.0.0.0:5432/testing?sslmode=disable
```

You can pass the URL as an environment variable as well:

```
DBURL=postgres://postgres:@0.0.0.0:5432/testing?sslmode=disable migrate to-version 5
```

## Using the API programmatically

Lucky for you, the API can be used programmatically as well. Do you want to import your migrations? Easy, import them, it's just Go code.

```
package main

import (
        _ "my/package/migrations"
)

// do something
```

Check the [documentation](https://godoc.org/github.com/erizocosmico/mig) to see the list of available functionality, which is the same that is available using the generated command.

Why could this be useful? In case you want your binary to autoupdate itself accordingly. The downside of this is that all migrations code would be inside your main binary. That's why the `mig` tool scaffolds a separate command just for migration management.

## Supported drivers

* [MySQL](https://github.com/go-sql-driver/mysql)
* [PostgreSQL](https://github.com/lib/pq)
* [MSSQL](https://github.com/denisenkom/go-mssqldb)
* [SQLite3](https://github.com/mattn/go-sqlite3)

## Acknowledgements

[go-pg/migrations](https://github.com/go-pg/migrations) for the inspiration. This library is basically `migrations` but it creates the migrations without needing to query the database or create the manager command yourself for the migrations. It also supports more databases than just PostgreSQL.

## LICENSE

MIT, see [LICENSE](/LICENSE).
