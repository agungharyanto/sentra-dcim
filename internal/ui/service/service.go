// Package service builds the UI bootstrap document: regional defaults and the menu registry.
package service

import (
	"context"
	"slices"
	"time"

	"github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// AppName is the product name shown by the UI shell.
const AppName = "Sentra DCIM"

// Menu status values. Labels are never sent: the UI translates shell:sidebar.<key> itself.
const (
	StatusAvailable    = "available"
	StatusComingSoon   = "coming_soon"
	StatusNotConnected = "not_connected"
)

// Menu groups (sidebar sections). Labels are translated by the UI from shell:groups.<key>.
const (
	GroupMonitoring     = "monitoring"
	GroupInfrastructure = "infrastructure"
	GroupOperations     = "operations"
	GroupSystem         = "system"
)

// MenuGroup is one sidebar section; groups are shown in Order.
type MenuGroup struct {
	Key   string `json:"key"`
	Order int    `json:"order"`
}

// Menu is one entry of the navigation registry.
type Menu struct {
	Key    string `json:"key"`
	Path   string `json:"path"`
	Icon   string `json:"icon"` // lucide icon name
	Status string `json:"status"`
	Group  string `json:"group"`
	Order  int    `json:"order"` // position within its group
	// Permission needed to see the menu ("" = every signed-in user). The UI hides menus the user lacks; the API
	// enforces the same permission on the module's routes.
	Permission string `json:"permission"`
}

// Bootstrap is the document the UI loads on start (GET /api/v1/ui/bootstrap).
type Bootstrap struct {
	AppName          string      `json:"app_name"`
	Version          string      `json:"version"`
	DefaultLocale    string      `json:"default_locale"`
	SupportedLocales []string    `json:"supported_locales"`
	Timezone         string      `json:"timezone"`
	MenuGroups       []MenuGroup `json:"menu_groups"`
	Menus            []Menu      `json:"menus"`
}

// groups and registry are the single source of the navigation menu, following docs/ROADMAP.md. A module flips its
// entry to StatusAvailable in the version that ships it (and must then have a page in the UI's moduleRegistry —
// enforced by web/src/app/routes.test.ts via web/src/test/fixtures/ui-registry.json).
var groups = []MenuGroup{
	{Key: GroupMonitoring, Order: 10},
	{Key: GroupInfrastructure, Order: 20},
	{Key: GroupOperations, Order: 30},
	{Key: GroupSystem, Order: 90},
}

var registry = []Menu{
	{Key: "dashboard", Path: "/dashboard", Icon: "layout-dashboard", Status: StatusAvailable, Group: GroupMonitoring, Order: 10},
	{Key: "alarms", Path: "/alarms", Icon: "siren", Status: StatusComingSoon, Group: GroupMonitoring, Order: 20},
	{Key: "power", Path: "/power", Icon: "zap", Status: StatusComingSoon, Group: GroupMonitoring, Order: 30},
	{Key: "environment", Path: "/environment", Icon: "thermometer", Status: StatusComingSoon, Group: GroupMonitoring, Order: 40},
	{Key: "facility", Path: "/facility", Icon: "building-2", Status: StatusAvailable, Group: GroupInfrastructure, Order: 10,
		Permission: string(rbac.FacilityRead)},
	{Key: "assets", Path: "/assets", Icon: "server", Status: StatusComingSoon, Group: GroupInfrastructure, Order: 20},
	{Key: "tenants", Path: "/tenants", Icon: "users", Status: StatusComingSoon, Group: GroupInfrastructure, Order: 30},
	{Key: "access", Path: "/access", Icon: "door-closed", Status: StatusComingSoon, Group: GroupOperations, Order: 10},
	{Key: "visitors", Path: "/visitors", Icon: "id-card", Status: StatusComingSoon, Group: GroupOperations, Order: 20},
	{Key: "maintenance", Path: "/maintenance", Icon: "wrench", Status: StatusComingSoon, Group: GroupOperations, Order: 30},
	{Key: "analytics", Path: "/analytics", Icon: "chart-line", Status: StatusComingSoon, Group: GroupOperations, Order: 40},
	{Key: "help", Path: "/help", Icon: "circle-help", Status: StatusAvailable, Group: GroupSystem, Order: 10},
	{Key: "users", Path: "/users", Icon: "user-cog", Status: StatusAvailable, Group: GroupSystem, Order: 15,
		Permission: string(rbac.UsersRead)},
	{Key: "audit", Path: "/audit", Icon: "scroll-text", Status: StatusAvailable, Group: GroupSystem, Order: 17,
		Permission: string(rbac.AuditRead)},
	{Key: "settings", Path: "/settings", Icon: "settings", Status: StatusAvailable, Group: GroupSystem, Order: 20,
		Permission: string(rbac.SettingsManage)},
}

// Service serves the UI bootstrap document.
type Service struct {
	locale  string
	locales []string
	tz      *time.Location
	system  func(ctx context.Context) (string, string, bool)
}

// New creates a Service. locale must be one of locales (validated by config).
func New(locale string, locales []string, tz *time.Location) *Service {
	return &Service{locale: locale, locales: slices.Clone(locales), tz: tz}
}

// SetSystem makes the bootstrap document follow the system language and time zone of Settings – Regional (v0.3.5)
// instead of DEFAULT_LOCALE / DISPLAY_TIMEZONE (which stay the defaults). f returns ok=false on errors.
func (s *Service) SetSystem(f func(ctx context.Context) (locale, timezone string, ok bool)) {
	s.system = f
}

// Bootstrap returns the bootstrap document: groups sorted by Order, menus sorted by group order then menu Order.
// DefaultLocale and Timezone are the system values; a signed-in user's own values come from /auth/me (regional).
func (s *Service) Bootstrap(ctx context.Context) Bootstrap {
	gs, menus := Registry()
	locale, tz := s.locale, s.tz.String()
	if s.system != nil {
		if l, z, ok := s.system(ctx); ok {
			locale, tz = l, z
		}
	}
	return Bootstrap{
		AppName:          AppName,
		Version:          buildinfo.Version,
		DefaultLocale:    locale,
		SupportedLocales: slices.Clone(s.locales),
		Timezone:         tz,
		MenuGroups:       gs,
		Menus:            menus,
	}
}

// Registry returns sorted copies of the menu groups and menus (independent of server configuration).
func Registry() ([]MenuGroup, []Menu) {
	gs := slices.Clone(groups)
	slices.SortStableFunc(gs, func(a, b MenuGroup) int { return a.Order - b.Order })
	rank := make(map[string]int, len(gs))
	for i, g := range gs {
		rank[g.Key] = i
	}
	menus := slices.Clone(registry)
	slices.SortStableFunc(menus, func(a, b Menu) int {
		if d := rank[a.Group] - rank[b.Group]; d != 0 {
			return d
		}
		return a.Order - b.Order
	})
	return gs, menus
}
