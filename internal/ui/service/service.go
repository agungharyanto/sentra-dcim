// Package service builds the UI bootstrap document: regional defaults and the menu registry.
package service

import (
	"slices"
	"time"

	"github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo"
)

// AppName is the product name shown by the UI shell.
const AppName = "Sentra DCIM"

// Menu status values. Labels are never sent: the UI translates shell:sidebar.<key> itself.
const (
	StatusAvailable    = "available"
	StatusComingSoon   = "coming_soon"
	StatusNotConnected = "not_connected"
)

// Menu is one entry of the navigation registry.
type Menu struct {
	Key    string `json:"key"`
	Path   string `json:"path"`
	Icon   string `json:"icon"` // lucide icon name
	Status string `json:"status"`
	Order  int    `json:"order"`
}

// Bootstrap is the document the UI loads on start (GET /api/v1/ui/bootstrap).
type Bootstrap struct {
	AppName          string   `json:"app_name"`
	Version          string   `json:"version"`
	DefaultLocale    string   `json:"default_locale"`
	SupportedLocales []string `json:"supported_locales"`
	Timezone         string   `json:"timezone"`
	Menus            []Menu   `json:"menus"`
}

// registry is the single source of the navigation menu, following docs/ROADMAP.md. A module flips its entry to
// StatusAvailable in the version that ships it.
var registry = []Menu{
	{Key: "dashboard", Path: "/", Icon: "layout-dashboard", Status: StatusAvailable, Order: 10},
	{Key: "facility", Path: "/facility", Icon: "building-2", Status: StatusComingSoon, Order: 20},
	{Key: "assets", Path: "/assets", Icon: "server", Status: StatusComingSoon, Order: 30},
	{Key: "tenants", Path: "/tenants", Icon: "users", Status: StatusComingSoon, Order: 40},
	{Key: "alarms", Path: "/alarms", Icon: "siren", Status: StatusComingSoon, Order: 50},
	{Key: "power", Path: "/power", Icon: "zap", Status: StatusComingSoon, Order: 60},
	{Key: "environment", Path: "/environment", Icon: "thermometer", Status: StatusComingSoon, Order: 70},
	{Key: "access", Path: "/access", Icon: "door-closed", Status: StatusComingSoon, Order: 80},
	{Key: "visitors", Path: "/visitors", Icon: "id-card", Status: StatusComingSoon, Order: 90},
	{Key: "maintenance", Path: "/maintenance", Icon: "wrench", Status: StatusComingSoon, Order: 100},
	{Key: "analytics", Path: "/analytics", Icon: "chart-line", Status: StatusComingSoon, Order: 110},
	{Key: "help", Path: "/help", Icon: "circle-help", Status: StatusComingSoon, Order: 900},
	{Key: "settings", Path: "/settings", Icon: "settings", Status: StatusComingSoon, Order: 910},
}

// Service serves the UI bootstrap document.
type Service struct {
	locale  string
	locales []string
	tz      *time.Location
}

// New creates a Service. locale must be one of locales (validated by config).
func New(locale string, locales []string, tz *time.Location) *Service {
	return &Service{locale: locale, locales: slices.Clone(locales), tz: tz}
}

// Bootstrap returns the bootstrap document with menus sorted by Order.
func (s *Service) Bootstrap() Bootstrap {
	menus := slices.Clone(registry)
	slices.SortStableFunc(menus, func(a, b Menu) int { return a.Order - b.Order })
	return Bootstrap{
		AppName:          AppName,
		Version:          buildinfo.Version,
		DefaultLocale:    s.locale,
		SupportedLocales: slices.Clone(s.locales),
		Timezone:         s.tz.String(),
		Menus:            menus,
	}
}
