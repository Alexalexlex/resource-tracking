package service

import (
	"context"

	"example.com/fiber-hello/internal/entity"
	"example.com/fiber-hello/internal/repository"
)

type DataService interface {
	SendData(ctx context.Context, data entity.Data) entity.Data
}

type dataService struct {
	repo repository.DataRepository
}

func NewDataService(repo repository.DataRepository) DataService {
	return &dataService{repo: repo}
}

func (s *dataService) SendData(ctx context.Context, data entity.Data) entity.Data {

	id := s.repo.SendData(ctx, data)

	return id
}
