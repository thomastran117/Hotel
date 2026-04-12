package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"hotel/application/environment"
)

//go:embed schema.sql
var schemaSQL string

// ResetSchema drops the configured database, recreates it, and applies
// schema.sql from scratch. Triggered by DB_RESET_SCHEMA=true at startup.
func ResetSchema(cfg *environment.Config) error {
	// Step 1 — connect without a database to drop and recreate it.
	dsnNoDB := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/?charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort,
	)
	root, err := sql.Open("mysql", dsnNoDB)
	if err != nil {
		return fmt.Errorf("migrate: open root connection: %w", err)
	}
	defer root.Close()

	if _, err := root.Exec("DROP DATABASE IF EXISTS `" + cfg.DBName + "`"); err != nil {
		return fmt.Errorf("migrate: drop database: %w", err)
	}
	if _, err := root.Exec("CREATE DATABASE `" + cfg.DBName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return fmt.Errorf("migrate: create database: %w", err)
	}
	root.Close()

	// Step 2 — connect to the new database and execute every statement in schema.sql.
	dsnWithDB := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)
	schema, err := sql.Open("mysql", dsnWithDB)
	if err != nil {
		return fmt.Errorf("migrate: open schema connection: %w", err)
	}
	defer schema.Close()

	for _, stmt := range parseSQLStatements(schemaSQL) {
		if _, err := schema.Exec(stmt); err != nil {
			preview := stmt
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			return fmt.Errorf("migrate: exec %q: %w", preview, err)
		}
	}

	return nil
}

// parseSQLStatements splits a SQL file into individual executable statements.
// It honours MySQL's DELIMITER directive, which is used for triggers and
// stored procedures where the body itself contains semicolons.
func parseSQLStatements(src string) []string {
	var stmts []string
	delimiter := ";"
	var buf strings.Builder

	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)

		// DELIMITER directive — switch the active terminator, don't accumulate.
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER") {
			parts := strings.Fields(trimmed)
			if len(parts) == 2 {
				delimiter = parts[1]
			}
			continue
		}

		buf.WriteString(line)
		buf.WriteString("\n")

		if strings.HasSuffix(trimmed, delimiter) {
			stmt := strings.TrimSuffix(strings.TrimSpace(buf.String()), delimiter)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" && !sqlOnlyComments(stmt) {
				stmts = append(stmts, stmt)
			}
			buf.Reset()
		}
	}

	return stmts
}

// sqlOnlyComments reports whether every non-blank line in s is a SQL comment.
func sqlOnlyComments(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t != "" && !strings.HasPrefix(t, "--") {
			return false
		}
	}
	return true
}
