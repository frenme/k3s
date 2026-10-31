package policy

import "github.com/egor/k3s-base/internal/lib/httpx"

var generated = Table{
	{Method: "GET", Path: "/api/v1/admin/orders", Access: httpx.AccessAdmin},
	{Method: "GET", Path: "/api/v1/admin/scheduler/jobs", Access: httpx.AccessAdmin},
	{Method: "POST", Path: "/api/v1/admin/scheduler/jobs/*/run", Access: httpx.AccessAdmin},
	{Method: "GET", Path: "/api/v1/admin/users", Access: httpx.AccessAdmin},
	{Method: "GET", Path: "/api/v1/common/app-config", Access: httpx.AccessPublic, CacheTTL: sec(10)},
	{Method: "GET", Path: "/api/v1/common/health", Access: httpx.AccessPublic},
	{Method: "GET", Path: "/api/v1/common/health/details", Access: httpx.AccessAdmin},
	{Method: "GET", Path: "/api/v1/orders", Access: httpx.AccessUser},
	{Method: "POST", Path: "/api/v1/orders", Access: httpx.AccessUser, Timeout: sec(5), MaxBody: 65536},
	{Method: "GET", Path: "/api/v1/orders/*", Access: httpx.AccessUser},
	{Method: "GET", Path: "/api/v1/users", Access: httpx.AccessAdmin},
	{Method: "POST", Path: "/api/v1/users", Access: httpx.AccessPublic},
	{Method: "GET", Path: "/api/v1/users/*", Access: httpx.AccessUser},
	{Method: "GET", Path: "/api/v1/users/*/roles", Access: httpx.AccessAdmin},
	{Method: "DELETE", Path: "/api/v1/users/*/roles/*", Access: httpx.AccessAdmin},
	{Method: "PUT", Path: "/api/v1/users/*/roles/*", Access: httpx.AccessAdmin},
}
