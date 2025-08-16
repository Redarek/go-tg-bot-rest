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

func init() { rand.Seed(time.Now().UnixNano()) }

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{DB: db} }

func (r *Repository) CreatePromotion(ctx context.Context, name, value string) error {
	_, err := r.DB.Exec(ctx,
		`INSERT INTO promotions (name, value) VALUES ($1, $2)`, name, value)
	return err
}

func (r *Repository) UpdatePromotion(ctx context.Context, id int, name, value string) error {
	_, err := r.DB.Exec(ctx,
		`UPDATE promotions SET name=$1, value=$2 WHERE id=$3`, name, value, id)
	return err
}

func (r *Repository) DeletePromotion(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM promotions WHERE id=$1`, id)
	return err
}

func (r *Repository) GetPromotions(ctx context.Context) ([]models.Promotion, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, value FROM promotions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Promotion
	for rows.Next() {
		var p models.Promotion
		if err := rows.Scan(&p.ID, &p.Name, &p.Value); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *Repository) GetRandomPromotion(ctx context.Context) (models.Promotion, error) {
	var p models.Promotion
	err := r.DB.QueryRow(ctx,
		`SELECT id, name, value FROM promotions ORDER BY RANDOM() LIMIT 1`).
		Scan(&p.ID, &p.Name, &p.Value)

	if errors.Is(err, pgx.ErrNoRows) {
		return models.Promotion{}, errors.New("список скидок пуст")
	}
	return p, err
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
		ON CONFLICT (user_id)
		DO UPDATE SET state=$2, data=$3`,
		st.UserID, st.State, st.Data)
	return err
}

func (r *Repository) GetAdminState(ctx context.Context, userID int64) (models.AdminState, error) {
	var st models.AdminState
	err := r.DB.QueryRow(ctx,
		`SELECT user_id, state, data FROM admin_states WHERE user_id=$1`,
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
