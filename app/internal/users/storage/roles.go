package storage

import (
	"context"
	"errors"

	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/users/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userRoleRow struct {
	UserID string `gorm:"type:uuid;primaryKey"`
	Role   string `gorm:"primaryKey"`
}

func (userRoleRow) TableName() string { return "user_roles" }

type Roles struct {
	db *pg.DB
}

func NewRoles(db *pg.DB) *Roles {
	return &Roles{db: db}
}

func (s *Roles) Assigned(ctx context.Context, userID string) ([]model.Role, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return nil, err
	}
	var rows []userRoleRow
	if err := s.db.Conn(ctx).Order("role").Find(&rows, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	roles := make([]model.Role, 0, len(rows))
	for _, r := range rows {
		roles = append(roles, model.Role(r.Role))
	}
	return roles, nil
}

func (s *Roles) Grant(ctx context.Context, userID string, role model.Role) error {
	if err := s.requireUser(ctx, userID); err != nil {
		return err
	}
	row := userRoleRow{UserID: userID, Role: string(role)}
	return s.db.Conn(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (s *Roles) Revoke(ctx context.Context, userID string, role model.Role) error {
	if err := s.requireUser(ctx, userID); err != nil {
		return err
	}
	return s.db.Conn(ctx).Delete(&userRoleRow{}, "user_id = ? AND role = ?", userID, string(role)).Error
}

func (s *Roles) requireUser(ctx context.Context, userID string) error {
	if !pg.IsUUID(userID) {
		return model.ErrUserNotFound
	}
	var row userRow
	err := s.db.Conn(ctx).Select("id").First(&row, "id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrUserNotFound
	}
	return err
}
