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

type settingRow struct {
	Key       string `gorm:"primaryKey"`
	Value     string `gorm:"type:jsonb"`
	Public    bool
	UpdatedAt time.Time
}

func (settingRow) TableName() string { return "settings" }

func (r settingRow) model() model.Setting {
	return model.Setting{Key: r.Key, Value: json.RawMessage(r.Value), Public: r.Public, UpdatedAt: r.UpdatedAt}
}

type Settings struct {
	db *pg.DB
}

func NewSettings(db *pg.DB) *Settings {
	return &Settings{db: db}
}

func (s *Settings) ListPublic(ctx context.Context) ([]model.Setting, error) {
	return pg.Read(s.db, func(db *gorm.DB) ([]model.Setting, error) {
		var rows []settingRow
		if err := db.WithContext(ctx).Where("public").Order("key").Find(&rows).Error; err != nil {
			return nil, err
		}
		settings := make([]model.Setting, 0, len(rows))
		for _, r := range rows {
			settings = append(settings, r.model())
		}
		return settings, nil
	})
}

func (s *Settings) Get(ctx context.Context, key string) (model.Setting, error) {
	setting, err := pg.Read(s.db, func(db *gorm.DB) (model.Setting, error) {
		var row settingRow
		err := db.WithContext(ctx).First(&row, "key = ?", key).Error
		return row.model(), err
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Setting{}, model.ErrSettingNotFound
	}
	return setting, err
}
