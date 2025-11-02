package service

import (
	"example.com/fiber-hello/internal/entity"
	"example.com/fiber-hello/internal/repository"
)

type MetaService interface {
	GetAllUsers() []entity.Meta
}

type metaService struct {
	repo repository.MetaRepository
}

func NewMetaService(repo repository.MetaRepository) MetaService {
	return &metaService{repo: repo}
}

func (s *metaService) GetAllUsers() []entity.Meta {
	return s.repo.FindAll()
}
