package pg

import (
	"github.com/egor/k3s-base/internal/lib/pagination"
	"gorm.io/gorm"
)

func Keyset(q *gorm.DB, after *pagination.Cursor, limit int) *gorm.DB {
	if after != nil {
		q = q.Where("(created_at, id) < (?, ?)", after.CreatedAt, after.ID)
	}
	return q.Order("created_at DESC, id DESC").Limit(limit + 1)
}
