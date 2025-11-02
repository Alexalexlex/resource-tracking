package repository

import (
	"example.com/fiber-hello/internal/entity"
)

type DataRepository interface {
	FindAll() []entity.Data
}

type dataRepo struct{}

func NewDataRepository() DataRepository {
	return &dataRepo{}
}

func (r *dataRepo) FindAll() []entity.Data {
	return []entity.Data{
		{ID: 1, Name: "Alice", Email: "alice@mail.com"},
		{ID: 2, Name: "Bob", Email: "bob@mail.com"},
	}
}
