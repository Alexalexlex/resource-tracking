package repository

import (
	"context"
	"log"
	"time"

	"example.com/fiber-hello/internal/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DataRepository interface {
	SendData(ctx context.Context, data entity.Data) entity.Data
}

type dataRepo struct {
	pool *pgxpool.Pool
}

func NewDataRepository(pool *pgxpool.Pool) DataRepository {
	return &dataRepo{pool: pool}
}

func (r *dataRepo) SendData(ctx context.Context, data entity.Data) entity.Data {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := r.pool.QueryRow(ctx, `INSERT INTO data (path, source, meta)
		 VALUES ($1, $2, $3)
		 RETURNING id`, data.Path, data.Source, data.Meta).Scan(&data.ID)

	if err != nil {
		log.Printf("SendData query error: %v", err)
		return entity.Data{}
	}
	return data
}
