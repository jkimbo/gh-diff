package db

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
)

const SCHEMA = `
	CREATE TABLE IF NOT EXISTS diffs (
		id TEXT PRIMARY KEY,
		branch TEXT,
		pr_number TEXT,
		stacked_on TEXT
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_diffs_id ON diffs (id);
`

// "models"

// Diff .
type Diff struct {
	ID        string `db:"id"`
	Branch    string `db:"branch"`
	PRNumber  string `db:"pr_number"`
	StackedOn string `db:"stacked_on"`
}

// DB .
type DB interface {
	GetDiff(ctx context.Context, diffID string) (*Diff, error)
	CreateDiff(ctx context.Context, diff *Diff) error
	GetChildDiff(ctx context.Context, diffID string) (*Diff, error)
}

// SQLDB .
type SQLDB struct {
	DB               *sqlx.DB
	StatementBuilder squirrel.StatementBuilderType
}

// NewDB .
func NewDB(ctx context.Context, filepath string) (*SQLDB, error) {
	db, err := sqlx.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	return &SQLDB{
		DB:               db,
		StatementBuilder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}, nil
}

// GetDiff .
func (db *SQLDB) GetDiff(ctx context.Context, diffID string) (*Diff, error) {
	query, args, err := db.StatementBuilder.Select("*").From("diffs").
		Where("id = ?", diffID).ToSql()
	if err != nil {
		return nil, err
	}
	var diff Diff
	if err := db.DB.Get(&diff, query, args...); err != nil {
		return nil, err
	}
	return &diff, nil
}

// CreateDiff .
func (db *SQLDB) CreateDiff(ctx context.Context, diff *Diff) error {
	statement := db.StatementBuilder.Insert("diffs").
		Columns(
			"id",
			"branch",
			"pr_number",
			"stacked_on",
		).
		Values(
			diff.ID,
			diff.Branch,
			diff.PRNumber,
			diff.StackedOn,
		)

	query, args, err := statement.ToSql()
	if err != nil {
		return err
	}

	_, err = db.DB.ExecContext(ctx, query, args...)
	return err
}

// GetChildDiff .
func (db *SQLDB) GetChildDiff(ctx context.Context, diffID string) (*Diff, error) {
	query, args, err := db.StatementBuilder.Select("*").From("diffs").
		Where("stacked_on = ?", diffID).ToSql()
	if err != nil {
		return nil, err
	}
	var diff Diff
	if err := db.DB.Get(&diff, query, args...); err != nil {
		return nil, err
	}
	return &diff, nil
}

// Init setups up the database schema
func (db *SQLDB) Init(ctx context.Context) error {
	// execute a query on the server
	_, err := db.DB.Exec(SCHEMA)
	return err
}
