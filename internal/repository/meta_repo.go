package repository

import (
	"example.com/fiber-hello/internal/entity"
)

type MetaRepository interface {
	FindAll() []entity.Meta
}

type metaRepo struct{}

func NewMetaRepository() MetaRepository {
	return &metaRepo{}
}

func (r *metaRepo) FindAll() []entity.Meta {
	return []entity.Meta{
		{ID: 1, Name: "Alice", Email: "alice@mail.com"},
		{ID: 2, Name: "Bob", Email: "bob@mail.com"},
	}
}
