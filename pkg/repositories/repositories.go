package repositories

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/Redarek/go-tg-bot-rest/pkg/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoPromotions = errors.New("no_promotions")

func init() { rand.Seed(time.Now().UnixNano()) }

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{DB: db} }

func (r *Repository) CreatePromotion(ctx context.Context, name, value, imageURL string) error {
	_, err := r.DB.Exec(ctx, `INSERT INTO promotions (name, value, image_url) VALUES ($1, $2, $3)`, name, value, imageURL)
	return err
}

func (r *Repository) UpdatePromotion(ctx context.Context, id int, name, value, imageURL string) error {
	_, err := r.DB.Exec(ctx, `UPDATE promotions SET name=$1, value=$2, image_url=$3 WHERE id=$4`, name, value, imageURL, id)
	return err
}

func (r *Repository) DeletePromotion(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM promotions WHERE id=$1`, id)
	return err
}

func (r *Repository) GetPromotions(ctx context.Context) ([]models.Promotion, error) {
	rows, err := r.DB.Query(ctx, `SELECT id, name, value, image_url FROM promotions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Promotion
	for rows.Next() {
		var p models.Promotion
		if err = rows.Scan(&p.ID, &p.Name, &p.Value, &p.ImageURL); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *Repository) GetRandomPromotion(ctx context.Context) (models.Promotion, error) {
	var p models.Promotion
	err := r.DB.QueryRow(ctx,
		`SELECT id, name, value, image_url FROM promotions ORDER BY RANDOM() LIMIT 1`).
		Scan(&p.ID, &p.Name, &p.Value, &p.ImageURL)

	if errors.Is(err, pgx.ErrNoRows) {
		return models.Promotion{}, errors.New("список скидок пуст")
	}
	return p, err
}

// Атомарная попытка получить право на скидку 1 раз на user_id
func (r *Repository) TryClaim(ctx context.Context, userID int64) (bool, error) {
	ct, err := r.DB.Exec(ctx, `
		INSERT INTO user_claims (user_id) VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}

func (r *Repository) HasUserClaimed(ctx context.Context, userID int64) bool {
	var exists bool
	_ = r.DB.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_claims WHERE user_id=$1)`, userID).
		Scan(&exists)
	return exists
}

func (r *Repository) MarkUserClaimed(ctx context.Context, userID int64) error {
	_, err := r.DB.Exec(ctx,
		`INSERT INTO user_claims (user_id) VALUES ($1)`,
		userID)
	return err
}

func (r *Repository) SetAdminState(ctx context.Context, st models.AdminState) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO admin_states (user_id, state, data)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id) DO UPDATE SET state=$2, data=$3`,
		st.UserID, st.State, st.Data)
	return err
}

func (r *Repository) GetAdminState(ctx context.Context, userID int64) (models.AdminState, error) {
	var st models.AdminState
	err := r.DB.QueryRow(ctx, `SELECT user_id, state, data FROM admin_states WHERE user_id=$1`,
		userID).Scan(&st.UserID, &st.State, &st.Data)

	if errors.Is(err, pgx.ErrNoRows) {
		return models.AdminState{}, nil
	}
	return st, err
}

func (r *Repository) ClearAdminState(ctx context.Context, userID int64) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM admin_states WHERE user_id=$1`, userID)
	return err
}

func (r *Repository) UpsertBotUser(ctx context.Context, userID int64) error {
	_, err := r.DB.Exec(ctx,
		`INSERT INTO bot_users (user_id) VALUES ($1)
         ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}
