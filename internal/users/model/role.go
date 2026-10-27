package model

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

func ParseRole(s string) (Role, bool) {
	switch r := Role(s); r {
	case RoleUser, RoleAdmin:
		return r, true
	}
	return "", false
}
