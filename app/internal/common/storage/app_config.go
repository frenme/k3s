package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/egor/k3s-base/internal/common/model"
	"github.com/egor/k3s-base/internal/lib/cache"
	"github.com/egor/k3s-base/internal/lib/pg"
	"gorm.io/gorm"
)

type appConfigRow struct {
	Key       string `gorm:"primaryKey"`
	Value     string `gorm:"type:jsonb"`
	Public    bool
	UpdatedAt time.Time
}

func (appConfigRow) TableName() string { return "app_config" }

func (r appConfigRow) model() model.AppConfig {
	return model.AppConfig{Key: r.Key, Value: json.RawMessage(r.Value), Public: r.Public, UpdatedAt: r.UpdatedAt}
}

type AppConfig struct {
	db    *pg.DB
	cache *cache.Cache
	ttl   time.Duration
}

func NewAppConfig(db *pg.DB, c *cache.Cache, ttl time.Duration) *AppConfig {
	return &AppConfig{db: db, cache: c, ttl: ttl}
}

func (s *AppConfig) ListPublic(ctx context.Context) ([]model.AppConfig, error) {
	return cache.Fetch(ctx, s.cache, "public", s.ttl, func() ([]model.AppConfig, error) {
		return s.listPublic(ctx)
	})
}

func (s *AppConfig) Get(ctx context.Context, key string) (model.AppConfig, error) {
	return cache.Fetch(ctx, s.cache, "key:"+key, s.ttl, func() (model.AppConfig, error) {
		return s.get(ctx, key)
	})
}

func (s *AppConfig) listPublic(ctx context.Context) ([]model.AppConfig, error) {
	return pg.Read(ctx, s.db, func(db *gorm.DB) ([]model.AppConfig, error) {
		var rows []appConfigRow
		if err := db.WithContext(ctx).Where("public").Order("key").Find(&rows).Error; err != nil {
			return nil, err
		}
		items := make([]model.AppConfig, 0, len(rows))
		for _, r := range rows {
			items = append(items, r.model())
		}
		return items, nil
	})
}

func (s *AppConfig) get(ctx context.Context, key string) (model.AppConfig, error) {
	entry, err := pg.Read(ctx, s.db, func(db *gorm.DB) (model.AppConfig, error) {
		var row appConfigRow
		err := db.WithContext(ctx).First(&row, "key = ?", key).Error
		return row.model(), err
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AppConfig{}, model.ErrAppConfigNotFound
	}
	return entry, err
}
