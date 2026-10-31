package pg

import "github.com/google/uuid"

func IsUUID(s string) bool {
	return len(s) == 36 && uuid.Validate(s) == nil
}
