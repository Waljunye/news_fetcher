package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type ctxKey string

const txKey ctxKey = "tx"

type TransactionManager struct {
	db *sqlx.DB
}

func NewTransactionManager(db *sqlx.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

func (tm *TransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	txCtx := context.WithValue(ctx, txKey, tx)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func GetTxFromContext(ctx context.Context) *sqlx.Tx {
	tx, _ := ctx.Value(txKey).(*sqlx.Tx)
	return tx
}

func GetExecutor(ctx context.Context, db *sqlx.DB) sqlx.ExtContext {
	if tx := GetTxFromContext(ctx); tx != nil {
		return tx
	}
	return db
}