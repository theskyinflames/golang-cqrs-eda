package postgres

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	"github.com/acme/shop/internal/ordering/app"
	"github.com/acme/shop/internal/ordering/domain"
)

// Orders implements domain.OrderRepository and app.OrderReader.
type Orders struct{ DB *sql.DB }

func (r Orders) Save(ctx context.Context, o *domain.Order) error {
	_, err := conn(ctx, r.DB).ExecContext(ctx,
		`INSERT INTO orders (id, total) VALUES ($1, $2)`, o.ID().String(), o.Total())
	return err
}

func (r Orders) OrderByID(ctx context.Context, id uuid.UUID) (app.OrderView, error) {
	v := app.OrderView{ID: id}
	err := conn(ctx, r.DB).QueryRowContext(ctx,
		`SELECT total FROM orders WHERE id = $1`, id.String()).Scan(&v.Total)
	if errors.Is(err, sql.ErrNoRows) {
		return v, domain.ErrNotFound // adapters must not know about sql
	}
	return v, err
}
