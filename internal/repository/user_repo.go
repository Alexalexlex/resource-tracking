package repository

import (
	"example.com/fiber-hello/internal/entity"
)

type UserRepository interface {
	FindAll() []entity.User
}

type userRepo struct{}

func NewUserRepository() UserRepository {
	return &userRepo{}
}

func (r *userRepo) FindAll() []entity.User {
	return []entity.User{
		{ID: 1, Name: "Alice", Email: "alice@mail.com"},
		{ID: 2, Name: "Bob", Email: "bob@mail.com"},
	}
}
