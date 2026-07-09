package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/auth"
	"github.com/santiguti/rp-management/backend/internal/config"
	"github.com/santiguti/rp-management/backend/internal/db"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	recurringdomain "github.com/santiguti/rp-management/backend/internal/domain/recurring"
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
	case "run-recurring":
		if err := runRecurring(os.Args[2:]); err != nil {
			log.Fatalf("run-recurring: %v", err)
		}
	case "cleanup-sessions":
		if err := cleanupSessions(os.Args[2:]); err != nil {
			log.Fatalf("cleanup-sessions: %v", err)
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
	fmt.Fprintln(out, "  run-recurring [--rule <ucode>] [--at YYYY-MM-DD]")
	fmt.Fprintln(out, "  cleanup-sessions")
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

	cfg, err := config.LoadForJobs()
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

func runRecurring(args []string) error {
	fs := flag.NewFlagSet("run-recurring", flag.ExitOnError)
	ruleRaw := fs.String("rule", "", "only process this recurring expense rule ucode")
	atRaw := fs.String("at", "", "pretend today is this YYYY-MM-DD date")
	if err := fs.Parse(args); err != nil {
		return err
	}

	today := time.Now().UTC()
	if strings.TrimSpace(*atRaw) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*atRaw))
		if err != nil {
			return fmt.Errorf("parse --at: %w", err)
		}
		today = parsed
	}

	cfg, err := config.LoadForJobs()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	q := sqlc.New(pool)
	rules, err := recurringRules(ctx, q, *ruleRaw, today)
	if err != nil {
		return err
	}

	var generated, skipped int
	for _, rule := range rules {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		qtx := q.WithTx(tx)
		result, err := recurringdomain.ProcessOne(ctx, qtx, rule, today)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("process rule %s: %w", rule.Name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit rule %s: %w", rule.Name, err)
		}

		if result.Generated {
			generated++
			log.Printf("created: rule=%s due=%s amount=%s", rule.Name, result.DueDate.Format("2006-01-02"), numericString(rule.Amount))
		} else {
			skipped++
			log.Printf("skipped: rule=%s due=%s last_generated=%s", rule.Name, result.DueDate.Format("2006-01-02"), dateString(rule.LastGeneratedDate))
		}
	}
	log.Printf("generated=%d skipped=%d", generated, skipped)
	return nil
}

func cleanupSessions(args []string) error {
	fs := flag.NewFlagSet("cleanup-sessions", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected argument: %s", fs.Arg(0))
	}

	cfg, err := config.LoadForJobs()
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

	q := sqlc.New(pool)
	deleted, err := q.DeleteExpiredSessions(ctx)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	log.Printf("deleted expired sessions count=%d", deleted)
	return nil
}

func recurringRules(ctx context.Context, q *sqlc.Queries, ruleRaw string, today time.Time) ([]sqlc.RecurringExpense, error) {
	ruleRaw = strings.TrimSpace(ruleRaw)
	if ruleRaw != "" {
		var ucode pgtype.UUID
		if err := ucode.Scan(ruleRaw); err != nil {
			return nil, fmt.Errorf("parse --rule: %w", err)
		}
		row, err := q.GetRecurringExpenseByUcode(ctx, ucode)
		if err != nil {
			return nil, fmt.Errorf("get rule: %w", err)
		}
		return []sqlc.RecurringExpense{row.RecurringExpense}, nil
	}

	rules, err := q.ListDueRecurringExpenses(ctx, pgtype.Date{Time: today, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list due rules: %w", err)
	}
	return rules, nil
}

func numericString(n pgtype.Numeric) string {
	raw, err := n.MarshalJSON()
	if err != nil || string(raw) == "null" {
		return ""
	}
	return strings.Trim(string(raw), `"`)
}

func dateString(d pgtype.Date) string {
	if !d.Valid {
		return "<nil>"
	}
	return d.Time.Format("2006-01-02")
}
