# dbmigrator

Easy to use tool for managing database migrations. 

## Usage

This package is designed to be extensible and not to be used directly. After installing it as a depedency, the engine should be used as presented below in a buildable standalone.

```go
engine, err := dbmigrator.NewFromJsonConfig("./config.json") 
if err != nil {
  // ...
}

if err := engine.Process(); err != nil {
  // ...
}
```

This way, it can be part of a backend application or anyone can write a CLI wrapper around it.

## Operation

During the run, the programs reads the linked `sql` file. It selects which commands to run and then executes them. The selection is based upon `versioning`. The file structure should follow this pattern:

```sql
#v1

-- Single-line comments are valid and parsed correctly.
CREATE TABLE foo (
  id  INTEGER NOT NULL,

  PRIMARY KEY (id)
);

/*
And of course, multi-line comments are also supported,
so use if freely! :) 
*/
#v1.0.1
ALTER TABLE foo ADD COLUMN bar VARCHAR (10) DEFAULT '';

#v1.2
ALTER TABLE foo DROP COLUMN bar;
```

Each `block` should have a version tag – which is a `semver` – and the following statements will be associated with it. After the program starts, it checks for the `migrations` table, if there is none, then it tries to create it.

During the config phase, you can dinamically set this `migrations` table name, if there is none, the default will be used, which is `__migrations__`.

After parsing the given `sql` file, and quering the latest stored state of the database, the programs sorts the statements, which have higher version than the stored latest. Then those statements are executed.

It is possible to determine, whether the statements should be executed inside a transaction or not. In case of executing it via a transaction, each statement should run successfully or a rollback would take place. (NOTE: mysql do no support `DDL` statements in transactions, meaning each successful statement triggers an auto-commit.) Without using transactions, the execution would not stop at the first error.

### Direction

Directions are used to determine which statements would be used in a version section to upgrade or downgrade the database schema.


```sql
#v1.1

#[UP]
CREATE TABLE foo (
	id INTEGER NOT NULL,

	PRIMARY KEY (id)
);

#[DOWN]
DROP TABLE foo;
```

By default the engine is in up direction, but it be changed explicitly by calling:

```go
if err := e.ProcessWithDirection(dbmigrator.DirectionDown); err != nil {
	// ...
}
```

Keep in mind, that up direction runs every command that has higher version than the current version stored in the `migrations` table. However, down direction only runs the those commands – of course inside the #[DOWN] block – that has the same versioning as the stored.

## Config

Out of the box, only `JSON` and `environmental` configs are supported – `NewFromEnv`, `NewFromJsonConfig` factories –, however by explicitly calling `New` you can workaroud this, by providing the appropriate details.

```go
// New creates a new instance based upon the given config.
func New(c *Config, opts ...EngineOptFunc) (Engine, error)
```

```go
type Config struct {
	Host                string    // Target database host.
	Port                int       // Target database port.
	Database            string    // Target database name.
	Username            string    // Target database username.
	Password            string    // Target database password.
	DriverName          string    // The used driver eg. "mysql".
	MigrationsTableName string    // The name of the table, that holds the migrations history.
	MigrationsFilePath  string    // Relative path of the sql file.
	WithTransaction     bool      // Should use transactions, or not.
}
```

The corresponding pairs of `environmental variables`:
- HOST
- PORT
- DATABASE
- USERNAME
- PASSWORD
- DRIVER_NAME
- MIGRATIONS_TABLE_NAME
- MIGRATIONS_FILE_PATH
- WITH_TRANSACTION

Keep in mind, that this package do not include any drivers (such as "mysql"), so you have to import it within your own tool!
