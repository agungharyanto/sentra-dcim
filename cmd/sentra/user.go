package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	auditrepo "github.com/agungharyanto/sentra-dcim/internal/audit/repo"
	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/auth/password"
	authrepo "github.com/agungharyanto/sentra-dcim/internal/auth/repo"
	authservice "github.com/agungharyanto/sentra-dcim/internal/auth/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/database"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	usersservice "github.com/agungharyanto/sentra-dcim/internal/users/service"
)

const userUsage = "usage: sentra user create-admin --username <name> [--display-name <name>]\n" +
	"       sentra user reset-mfa --username <name>"

// runUser implements `sentra user <subcommand>`.
func runUser(args []string) error {
	if len(args) == 0 {
		return errors.New(userUsage)
	}
	switch args[0] {
	case "create-admin":
		return runCreateAdmin(args)
	case "reset-mfa":
		return runResetMFA(args)
	}
	return errors.New(userUsage)
}

// runResetMFA is the emergency path when a user (also the last Admin) lost the authenticator and the recovery
// codes: MFA and recovery codes are removed, all sessions revoked; the next sign-in enrols again (D-064).
func runResetMFA(args []string) error {
	fs := flag.NewFlagSet("user reset-mfa", flag.ContinueOnError)
	username := fs.String("username", "", "login name of the account")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("--username is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logging.New(os.Stderr, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()
	svc := usersservice.New(pool, auditservice.New(auditrepo.New(pool), log), log, cfg.Auth)
	id, err := svc.UserIDByUsername(ctx, *username)
	if err != nil {
		return fmt.Errorf("user %q: %w", *username, err)
	}
	if err := svc.ResetMFA(ctx, id, auditservice.Meta{}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "MFA of %q reset; all sessions revoked. The user sets up MFA again at the next sign-in.\n", *username)
	return nil
}

func runCreateAdmin(args []string) error {
	fs := flag.NewFlagSet("user create-admin", flag.ContinueOnError)
	username := fs.String("username", "", "login name (3–64: letters, digits, . _ -; NIP allowed)")
	display := fs.String("display-name", "", "display name (default: username)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("--username is required")
	}

	pw, err := readNewPassword(os.Stdin, os.Stderr)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logging.New(os.Stderr, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	svc, err := authservice.New(authrepo.New(pool), nil, auditservice.New(auditrepo.New(pool), log), log, cfg.Auth)
	if err != nil {
		return err
	}
	u, err := svc.CreateAdmin(ctx, *username, *display, pw)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "created user %q (id %s); at first login: set up MFA, then change the password\n", u.Username, u.ID)
	return nil
}

// readNewPassword asks twice without echo on a terminal; otherwise it reads exactly one line from stdin (for
// `docker compose exec -T`). The password is never printed or logged.
func readNewPassword(in *os.File, out io.Writer) (string, error) {
	fd := int(in.Fd()) //nolint:gosec // file descriptors fit in int
	if term.IsTerminal(fd) {
		_, _ = fmt.Fprintf(out, "New password (%d–%d characters): ", password.MinLength, password.MaxLength)
		a, err := term.ReadPassword(fd)
		_, _ = fmt.Fprintln(out)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprint(out, "Repeat password: ")
		b, err := term.ReadPassword(fd)
		_, _ = fmt.Fprintln(out)
		if err != nil {
			return "", err
		}
		if string(a) != string(b) {
			return "", errors.New("passwords do not match")
		}
		return checked(string(a))
	}
	line, err := bufio.NewReader(io.LimitReader(in, 4*password.MaxLength+2)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return checked(strings.TrimRight(line, "\r\n"))
}

func checked(pw string) (string, error) {
	if err := password.CheckPolicy(pw); err != nil {
		return "", fmt.Errorf("password must be %d–%d characters", password.MinLength, password.MaxLength)
	}
	return pw, nil
}
