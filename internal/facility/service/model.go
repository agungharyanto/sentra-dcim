package service

import (
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agungharyanto/sentra-dcim/internal/facility/repo"
)

// Site statuses, room types and rack statuses (CHECK constraints of migration 000011).
var (
	SiteStatuses = []string{"planned", "active", "inactive"}
	RoomTypes    = []string{"whitespace", "mmr", "power", "cooling", "noc", "storage", "staging", "other"}
	RackStatuses = []string{"planned", "active", "reserved", "decommissioning", "retired"}
	RackWidths   = []int{19, 23}
)

// Site is the API view of a site.
type Site struct {
	ID         uuid.UUID  `json:"id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Address    string     `json:"address"`
	Status     string     `json:"status"`
	Version    int        `json:"version"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at"`
}

// Building is the API view of a building.
type Building struct {
	ID         uuid.UUID  `json:"id"`
	SiteID     uuid.UUID  `json:"site_id"`
	SiteCode   string     `json:"site_code"`
	SiteName   string     `json:"site_name"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Notes      string     `json:"notes"`
	Version    int        `json:"version"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at"`
}

// Floor is the API view of a floor.
type Floor struct {
	ID           uuid.UUID  `json:"id"`
	BuildingID   uuid.UUID  `json:"building_id"`
	BuildingName string     `json:"building_name"`
	SiteID       uuid.UUID  `json:"site_id"`
	SiteCode     string     `json:"site_code"`
	Name         string     `json:"name"`
	Level        int        `json:"level"`
	Notes        string     `json:"notes"`
	Version      int        `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ArchivedAt   *time.Time `json:"archived_at"`
}

// Room is the API view of a room.
type Room struct {
	ID           uuid.UUID  `json:"id"`
	FloorID      uuid.UUID  `json:"floor_id"`
	FloorName    string     `json:"floor_name"`
	BuildingID   uuid.UUID  `json:"building_id"`
	BuildingName string     `json:"building_name"`
	SiteID       uuid.UUID  `json:"site_id"`
	SiteCode     string     `json:"site_code"`
	Name         string     `json:"name"`
	RoomType     string     `json:"room_type"`
	Notes        string     `json:"notes"`
	RackCount    int        `json:"rack_count"`
	Version      int        `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ArchivedAt   *time.Time `json:"archived_at"`
}

// Rack is the API view of a rack with the names of its location.
type Rack struct {
	ID           uuid.UUID  `json:"id"`
	RoomID       uuid.UUID  `json:"room_id"`
	RoomName     string     `json:"room_name"`
	RoomType     string     `json:"room_type"`
	FloorID      uuid.UUID  `json:"floor_id"`
	FloorName    string     `json:"floor_name"`
	BuildingID   uuid.UUID  `json:"building_id"`
	BuildingName string     `json:"building_name"`
	SiteID       uuid.UUID  `json:"site_id"`
	SiteCode     string     `json:"site_code"`
	SiteName     string     `json:"site_name"`
	Name         string     `json:"name"`
	AssetTag     *string    `json:"asset_tag"`
	RowLabel     string     `json:"row_label"`
	RowPosition  *int       `json:"row_position"`
	UHeight      int        `json:"u_height"`
	StartingUnit int        `json:"starting_unit"`
	DescUnits    bool       `json:"desc_units"`
	WidthIn      int        `json:"width_in"`
	DepthMM      *int       `json:"depth_mm"`
	MaxWeightKg  *float64   `json:"max_weight_kg"`
	MaxPowerKW   *float64   `json:"max_power_kw"`
	Status       string     `json:"status"`
	Notes        string     `json:"notes"`
	Version      int        `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ArchivedAt   *time.Time `json:"archived_at"`
}

// Opt is an optional JSON field of a PATCH body: Set when the key is present, Null when its value is null.
type Opt[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// UnmarshalJSON implements json.Unmarshaler (only called when the key is present).
func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	v := t.String
	return &v
}

func intPtr(i pgtype.Int4) *int {
	if !i.Valid {
		return nil
	}
	v := int(i.Int32)
	return &v
}

func numPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := math.Round(f.Float64*100) / 100
	return &v
}

func siteFrom(s repo.Site) Site {
	return Site{ID: s.ID, Code: s.Code, Name: s.Name, Address: s.Address, Status: s.Status, Version: int(s.Version),
		CreatedAt: s.CreatedAt.Time.UTC(), UpdatedAt: s.UpdatedAt.Time.UTC(), ArchivedAt: tsPtr(s.ArchivedAt)}
}

func rackFrom(k repo.GetRackRow) Rack {
	return Rack{ID: k.ID, RoomID: k.RoomID, RoomName: k.RoomName, RoomType: k.RoomType, FloorID: k.FloorID,
		FloorName: k.FloorName, BuildingID: k.BuildingID, BuildingName: k.BuildingName, SiteID: k.SiteID,
		SiteCode: k.SiteCode, SiteName: k.SiteName, Name: k.Name, AssetTag: textPtr(k.AssetTag), RowLabel: k.RowLabel,
		RowPosition: intPtr(k.RowPosition), UHeight: int(k.UHeight), StartingUnit: int(k.StartingUnit),
		DescUnits: k.DescUnits, WidthIn: int(k.WidthIn), DepthMM: intPtr(k.DepthMm), MaxWeightKg: numPtr(k.MaxWeightKg),
		MaxPowerKW: numPtr(k.MaxPowerKw), Status: k.Status, Notes: k.Notes, Version: int(k.Version),
		CreatedAt: k.CreatedAt.Time.UTC(), UpdatedAt: k.UpdatedAt.Time.UTC(), ArchivedAt: tsPtr(k.ArchivedAt)}
}
