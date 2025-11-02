package repository

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"example.com/fiber-hello/internal/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DataRepository interface {
	SendData() []entity.Data
}

type dataRepo struct {
	pool *pgxpool.Pool
}

func NewDataRepository(pool *pgxpool.Pool) DataRepository {
	return &dataRepo{pool: pool}
}

func (r *dataRepo) SendData() []entity.Data {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT id, path, source, meta
		FROM data
		ORDER BY id;
	`)
	if err != nil {
		log.Printf("SendData query error: %v", err)
		return []entity.Data{}
	}
	defer rows.Close()

	out := make([]entity.Data, 0, 16)
	for rows.Next() {
		var d entity.Data
		var metaBytes []byte
		if err := rows.Scan(&d.ID, &d.Path, &d.Source, &metaBytes); err != nil {
			log.Printf("SendData scan error: %v", err)
			continue
		}
		d.Meta = json.RawMessage(metaBytes)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		log.Printf("SendData rows error: %v", err)
	}
	return out
}
