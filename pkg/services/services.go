package services

import (
	"context"
	"errors"
	"github.com/Redarek/go-tg-bot-rest/pkg/models"
	"github.com/Redarek/go-tg-bot-rest/pkg/repositories"
)

type Service struct {
	Repo *repositories.Repository
}

func NewService(repo *repositories.Repository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) ClaimPromotion(ctx context.Context, userID, adminID int64) (models.Promotion, error) {
	if userID != adminID {
		if s.Repo.HasUserClaimed(ctx, userID) {
			return models.Promotion{}, errors.New("⚡️<u>Попытка была одна — и Фортуна уже подарила тебе особую скидку!</u>\n" +
				"🔄Хочешь другой? Тогда заказывай нашу броню TWILIGHT HAMMER и получай в бонус фирменный стикерпак, который идёт в комплекте с экипировкой.\n" +
				"Заказать можешь тут :\n" +
				"🟣<a href=\"https://www.wildberries.ru/brands/311439225-twilight-hammer\">WILDBERRIES</a>\n" +
				"🔵<a href=\"https://vk.com/t.hammer.clan\">VKONTAKTE</a>")
		}

		err := s.Repo.MarkUserClaimed(ctx, userID)
		if err != nil {
			return models.Promotion{}, err
		}
	}

	promotion, err := s.Repo.GetRandomPromotion(ctx)
	if err != nil {
		return models.Promotion{}, err
	}

	return promotion, err
}
