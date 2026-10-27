package policy

var generated = Table{
	{Method: "GET", Path: "/api/v1/admin/orders", Access: AccessAdmin},
	{Method: "GET", Path: "/api/v1/admin/scheduler/jobs", Access: AccessAdmin},
	{Method: "POST", Path: "/api/v1/admin/scheduler/jobs/*/run", Access: AccessAdmin},
	{Method: "GET", Path: "/api/v1/admin/users", Access: AccessAdmin},
	{Method: "GET", Path: "/api/v1/common/app-config", Access: AccessPublic, CacheTTL: sec(10)},
	{Method: "GET", Path: "/api/v1/common/health", Access: AccessPublic},
	{Method: "GET", Path: "/api/v1/common/health/details", Access: AccessAdmin},
	{Method: "GET", Path: "/api/v1/orders", Access: AccessUser},
	{Method: "POST", Path: "/api/v1/orders", Access: AccessUser, Timeout: sec(5), MaxBody: 65536},
	{Method: "GET", Path: "/api/v1/orders/*", Access: AccessUser},
	{Method: "GET", Path: "/api/v1/users", Access: AccessAdmin},
	{Method: "POST", Path: "/api/v1/users", Access: AccessPublic},
	{Method: "GET", Path: "/api/v1/users/*", Access: AccessUser},
	{Method: "GET", Path: "/api/v1/users/*/roles", Access: AccessAdmin},
	{Method: "DELETE", Path: "/api/v1/users/*/roles/*", Access: AccessAdmin},
	{Method: "PUT", Path: "/api/v1/users/*/roles/*", Access: AccessAdmin},
}
