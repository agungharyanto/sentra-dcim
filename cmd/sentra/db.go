package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/database"
)

// appGroupRole holds the application's privileges (migration 000006_db_roles).
const appGroupRole = "sentra_app"

var roleNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]{2,62}$`)

// runDB implements `sentra db <subcommand>` (ops container, migration owner credentials).
func runDB(args []string) error {
	if len(args) != 1 || args[0] != "sync-app-role" {
		return errors.New("usage: sentra db sync-app-role   (env DB_APP_USER, DB_APP_PASSWORD; run with the owner's DB credentials)")
	}
	return syncAppRole()
}

// syncAppRole creates or updates the application's login role (ISS-039): LOGIN, the password from DB_APP_PASSWORD
// (rotation = change .env, run again, restart the app), member of sentra_app, no superuser/createdb/createrole and
// not the owner of anything. The password is sent as a literal of a server-built statement and never printed.
func syncAppRole() error {
	user := os.Getenv("DB_APP_USER")
	pw := os.Getenv("DB_APP_PASSWORD")
	if !roleNameRE.MatchString(user) || user == appGroupRole {
		return fmt.Errorf("DB_APP_USER %q: 3–63 characters [a-z0-9_], starting with a letter, not %q", user, appGroupRole)
	}
	if len(pw) < 20 || len(pw) > 128 {
		return errors.New("DB_APP_PASSWORD must be 20–128 characters (e.g. openssl rand -hex 24)")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.DB.User == user {
		return errors.New("run with the migration owner's credentials (ops container), not as the application role")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)", user).Scan(&exists); err != nil {
		return err
	}
	verb := "ALTER"
	if !exists {
		verb = "CREATE"
	}
	var stmt string
	// format() quotes the identifier and the password literal on the server side.
	if err := tx.QueryRow(ctx, "SELECT format('%s ROLE %I WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION INHERIT PASSWORD %L', $1::text, $2::text, $3::text)",
		verb, user, pw).Scan(&stmt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, stmt); err != nil {
		// Never wrap err blindly: the statement carries the password. Server messages do not quote it.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return fmt.Errorf("%s role %s: %s (SQLSTATE %s)", verb, user, pgErr.Message, pgErr.Code)
		}
		return fmt.Errorf("%s role %s failed", verb, user)
	}
	if _, err := tx.Exec(ctx, "GRANT "+pgx.Identifier{appGroupRole}.Sanitize()+" TO "+pgx.Identifier{user}.Sanitize()); err != nil {
		return fmt.Errorf("grant %s: %w", appGroupRole, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "db: role %q %sd (login, member of %s); restart the app to use a changed password\n", user, map[bool]string{true: "update", false: "create"}[exists], appGroupRole)
	return nil
}
