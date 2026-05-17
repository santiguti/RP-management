package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/auth"
	"github.com/santiguti/rp-management/backend/internal/config"
	"github.com/santiguti/rp-management/backend/internal/db"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func main() {
	if len(os.Args) < 2 {
		usage(2)
	}
	switch os.Args[1] {
	case "seed-owner":
		if err := seedOwner(os.Args[2:]); err != nil {
			log.Fatalf("seed-owner: %v", err)
		}
	case "-h", "--help", "help":
		usage(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n", os.Args[1])
		usage(2)
	}
}

func usage(code int) {
	out := os.Stderr
	if code == 0 {
		out = os.Stdout
	}
	fmt.Fprintln(out, "usage: jobs <subcommand> [flags]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "subcommands:")
	fmt.Fprintln(out, "  seed-owner --username <u> --password <p> [--full-name <name>]")
	os.Exit(code)
}

func seedOwner(args []string) error {
	fs := flag.NewFlagSet("seed-owner", flag.ExitOnError)
	username := fs.String("username", "", "username (required)")
	password := fs.String("password", "", "plaintext password (required)")
	fullName := fs.String("full-name", "", "full name (defaults to username)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" || *password == "" {
		fs.Usage()
		return fmt.Errorf("--username and --password are required")
	}
	if *fullName == "" {
		*fullName = *username
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	hash, err := auth.Hash(*password)
	if err != nil {
		return err
	}

	q := sqlc.New(pool)
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Username:        *username,
		PasswordHash:    hash,
		FullName:        *fullName,
		Role:            "owner",
		CreatedByUserID: pgtype.Int8{}, // NULL — bootstrap user has no creator
	})
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	log.Printf("created owner id=%d username=%s ucode=%x", u.ID, u.Username, u.Ucode.Bytes)
	return nil
}
