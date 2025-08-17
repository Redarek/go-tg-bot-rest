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
			return models.Promotion{}, errors.New("⚡️<u>Попытка была одна — и Фортуна уже подарила тебе особую скидку!</u>\n\n" +
				"Забронируй столик на нашем сайте и воспользуйся скидкой в ресторане:\n" +
				"🔹<a href=\"https://ketino.ru\">НАШ САЙТ</a>\n" +
				"🔸<a href=\"https://instagram.com/ketino_rest\">INSTA</a>\n" +
				"🔹<a href=\"https://vk.com/ketinorest\">VKONTAKTE</a>\n" +
				"🔸<a href=\"https://t.me/ketinorest\">TELEGRAM</a>\n")
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
