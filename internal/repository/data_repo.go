package repository

import (
	"encoding/json"

	"example.com/fiber-hello/internal/entity"
)

type DataRepository interface {
	SendData() []entity.Data
}

type dataRepo struct{}

func NewDataRepository() DataRepository {
	return &dataRepo{}
}
func (r *dataRepo) SendData() []entity.Data {
	return []entity.Data{
		{ID: 1, Path: "/taxi", Source: "taxi", Meta: json.RawMessage(`{
		"summary":"driver here",
		"Name":"Alex"
		}`)},
		{ID: 2, Path: "/delivery", Source: "delivery", Meta: json.RawMessage(`{
		"summary":"customer feedback",
		"Name":"Alex"
		}`)},
	}
}
