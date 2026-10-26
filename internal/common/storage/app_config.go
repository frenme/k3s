package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/egor/k3s-base/internal/common/model"
	"github.com/egor/k3s-base/internal/lib/pg"
	"gorm.io/gorm"
)

type configRow struct {
	Key       string `gorm:"primaryKey"`
	Value     string `gorm:"type:jsonb"`
	Public    bool
	UpdatedAt time.Time
}

func (configRow) TableName() string { return "config" }

func (r configRow) model() model.Config {
	return model.Config{Key: r.Key, Value: json.RawMessage(r.Value), Public: r.Public, UpdatedAt: r.UpdatedAt}
}

type Config struct {
	db *pg.DB
}

func NewConfig(db *pg.DB) *Config {
	return &Config{db: db}
}

func (s *Config) ListPublic(ctx context.Context) ([]model.Config, error) {
	return pg.Read(s.db, func(db *gorm.DB) ([]model.Config, error) {
		var rows []configRow
		if err := db.WithContext(ctx).Where("public").Order("key").Find(&rows).Error; err != nil {
			return nil, err
		}
		config := make([]model.Config, 0, len(rows))
		for _, r := range rows {
			config = append(config, r.model())
		}
		return config, nil
	})
}

func (s *Config) Get(ctx context.Context, key string) (model.Config, error) {
	config, err := pg.Read(s.db, func(db *gorm.DB) (model.Config, error) {
		var row configRow
		err := db.WithContext(ctx).First(&row, "key = ?", key).Error
		return row.model(), err
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Config{}, model.ErrConfigNotFound
	}
	return config, err
}
