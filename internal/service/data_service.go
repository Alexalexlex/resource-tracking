package service

import (
	"example.com/fiber-hello/internal/entity"
	"example.com/fiber-hello/internal/repository"
)

type DataService interface {
	SendData() []entity.Data
}

type dataService struct {
	repo repository.DataRepository
}

func NewDataService(repo repository.DataRepository) DataService {
	return &dataService{repo: repo}
}

func (s *dataService) SendData() []entity.Data {
	return s.repo.SendData()
}
