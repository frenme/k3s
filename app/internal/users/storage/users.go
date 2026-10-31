package storage

import (
	"context"
	"errors"
	"time"

	"github.com/egor/k3s-base/internal/lib/cache"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/users/model"
	"gorm.io/gorm"
)

const userCacheTTL = time.Minute

type userRow struct {
	ID        string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email     string
	Name      string
	CreatedAt time.Time `gorm:"autoCreateTime:false;default:now()"`
}

func (userRow) TableName() string { return "users" }

func (r userRow) model() model.User {
	return model.User{ID: r.ID, Email: r.Email, Name: r.Name, CreatedAt: r.CreatedAt}
}

type Users struct {
	db    *pg.DB
	cache *cache.Cache
}

func NewUsers(db *pg.DB, c *cache.Cache) *Users {
	return &Users{db: db, cache: c}
}

func (s *Users) List(ctx context.Context, limit int, after *pagination.Cursor) ([]model.User, error) {
	return pg.Read(ctx, s.db, func(db *gorm.DB) ([]model.User, error) {
		var rows []userRow
		if err := pg.Keyset(db.WithContext(ctx), after, limit).Find(&rows).Error; err != nil {
			return nil, err
		}
		users := make([]model.User, 0, len(rows))
		for _, r := range rows {
			users = append(users, r.model())
		}
		return users, nil
	})
}

func (s *Users) Get(ctx context.Context, id string) (model.User, error) {
	if !pg.IsUUID(id) {
		return model.User{}, model.ErrUserNotFound
	}
	return cache.Fetch(ctx, s.cache, id, userCacheTTL, func() (model.User, error) {
		u, err := pg.Read(ctx, s.db, func(db *gorm.DB) (model.User, error) {
			var row userRow
			err := db.WithContext(ctx).First(&row, "id = ?", id).Error
			return row.model(), err
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, model.ErrUserNotFound
		}
		return u, err
	})
}

func (s *Users) Create(ctx context.Context, email, name string) (model.User, error) {
	row := userRow{Email: email, Name: name}
	err := s.db.Conn(ctx).Create(&row).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return model.User{}, model.ErrEmailTaken
	}
	return row.model(), err
}
