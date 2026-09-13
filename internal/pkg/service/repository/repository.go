package repository

import (
	"VK_BOT_Wall_Grabber/internal/pkg/domain/presenter"
	stor "VK_BOT_Wall_Grabber/internal/pkg/service/storage"
)

type repository struct {
	storage   *stor.Storage
	presenter presenter.Interface
}

func NewRepository(
	storage *stor.Storage,
	presenter presenter.Interface,
) *repository {
	return &repository{
		storage:   storage,
		presenter: presenter,
	}
}
