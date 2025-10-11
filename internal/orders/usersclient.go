package orders

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/egor/k3s-base/internal/platform/httpx"
)

var ErrUserNotFound = errors.New("user not found")

type UserChecker interface {
	Exists(ctx context.Context, id string) (bool, error)
}

