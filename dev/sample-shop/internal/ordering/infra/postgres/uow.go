package postgres

import (
	"context"
	"database/sql"
	"errors"
)

type txKey struct{}

// UnitOfWork implements cqrs.UnitOfWork with a database/sql transaction.
type UnitOfWork struct{ DB *sql.DB }

func (u UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	tx, err := u.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// conn returns the transaction in ctx, or db when there is none.
func conn(ctx context.Context, db *sql.DB) querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}
