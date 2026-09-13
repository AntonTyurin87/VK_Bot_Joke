package usecase

import (
	"VK_BOT_Wall_Grabber/internal/pkg/domain/presenter"
	"VK_BOT_Wall_Grabber/internal/pkg/logger"
	"VK_BOT_Wall_Grabber/internal/pkg/service/repository"
)

type usecase struct {
	log *logger.Logger

	presenter presenter.Interface

	repo repository.Interface
}

// NewUsacase ...
func NewUsacase(
	log *logger.Logger,

	presenter presenter.Interface,

	repo repository.Interface,

) *usecase {
	return &usecase{
		log: log,

		presenter: presenter,

		repo: repo,
	}
}
