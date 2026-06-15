package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	appmigrations "github.com/insiderEnesGozuela/go-app/migrations"
)

// Migrate applies all up migrations embedded in the migrations package against
// the given connection string. It is idempotent: golang-migrate records the
// current version in a schema_migrations table, so re-running on an up-to-date
// database is a no-op (migrate.ErrNoChange, which we treat as success).
//
// Why run migrations from the app at startup rather than as a separate step?
// For a single service this keeps "the schema the code expects" and "the code"
// deployed atomically — you can never run new code against an old schema. At
// larger scale you'd extract this into a dedicated `cmd/migrate` job; the
// roadmap leaves that for later. The mechanism is identical either way.
//
// We go through database/sql (pgx's stdlib driver) instead of pgx's native
// pool because golang-migrate's postgres driver is built on database/sql. This
// is the one place we touch database/sql; everything else uses the pgx pool.
func Migrate(dsn string) (err error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql db for migrate: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close migrate db: %w", cerr)
		}
	}()

	driver, err := migratepg.WithInstance(db, &migratepg.Config{})
	if err != nil {
		return fmt.Errorf("build migrate driver: %w", err)
	}

	src, err := iofs.New(appmigrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if upErr := m.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", upErr)
	}
	return nil
}
