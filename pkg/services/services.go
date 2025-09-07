package services

import (
	"context"
	"errors"
	"github.com/Redarek/go-tg-bot-rest/pkg/models"
	"github.com/Redarek/go-tg-bot-rest/pkg/repositories"
)

var ErrAlreadyClaimed = errors.New("already_claimed")

type Service struct {
	Repo *repositories.Repository
}

func NewService(repo *repositories.Repository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) ClaimPromotion(ctx context.Context, userID, adminID int64) (models.Promotion, error) {
	// Админ может дергать бесконечно
	if userID != adminID {
		ok, err := s.Repo.TryClaim(ctx, userID)
		if err != nil {
			return models.Promotion{}, err
		}
		if !ok {
			return models.Promotion{}, ErrAlreadyClaimed
		}
	}
	return s.Repo.GetRandomPromotion(ctx)
}
