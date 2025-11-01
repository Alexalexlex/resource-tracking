package service

import (
	"example.com/fiber-hello/internal/entity"
	"example.com/fiber-hello/internal/repository"
)

type UserService interface {
	GetAllUsers() []entity.User
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) GetAllUsers() []entity.User {
	return s.repo.FindAll()
}
