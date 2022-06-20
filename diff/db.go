package diff

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // so that sqlx works with sqlite
)

const schema = `
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
type dbdiff struct {
	ID        string `db:"id"`
	Branch    string `db:"branch"`
	PRNumber  string `db:"pr_number"`
	StackedOn string `db:"stacked_on"`
}

// DB .
type DB interface {
	getDiff(ctx context.Context, diffID string) (*dbdiff, error)
	createDiff(ctx context.Context, diff *dbdiff) error
	getChildDiff(ctx context.Context, diffID string) (*dbdiff, error)
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

func (db *SQLDB) getDiff(ctx context.Context, diffID string) (*dbdiff, error) {
	query, args, err := db.StatementBuilder.Select("*").From("diffs").
		Where("id = ?", diffID).ToSql()
	if err != nil {
		return nil, err
	}
	var diff dbdiff
	if err := db.DB.Get(&diff, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &diff, nil
}

func (db *SQLDB) createDiff(ctx context.Context, diff *dbdiff) error {
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

func (db *SQLDB) updatePrNumber(ctx context.Context, diffID, prNumber string) error {
	statement := db.StatementBuilder.Update("diffs").
		Set("pr_number", prNumber).
		Where("id = ?", diffID)

	query, args, err := statement.ToSql()
	if err != nil {
		return err
	}

	_, err = db.DB.ExecContext(ctx, query, args...)
	return err
}

func (db *SQLDB) getChildDiff(ctx context.Context, diffID string) (*dbdiff, error) {
	query, args, err := db.StatementBuilder.Select("*").From("diffs").
		Where("stacked_on = ?", diffID).ToSql()
	if err != nil {
		return nil, err
	}
	var diff dbdiff
	if err := db.DB.Get(&diff, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &diff, nil
}

func (db *SQLDB) removeDiff(ctx context.Context, diffID string) error {
	query, args, err := db.StatementBuilder.Delete("diffs").
		Where("id = ?", diffID).ToSql()
	if err != nil {
		return err
	}
	_, err = db.DB.ExecContext(ctx, query, args...)
	return err
}

// Init setups up the database schema
func (db *SQLDB) Init(ctx context.Context) error {
	// execute a query on the server
	_, err := db.DB.Exec(schema)
	return err
}
