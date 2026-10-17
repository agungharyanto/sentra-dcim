package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/facility/repo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// RackCreate is the body of a new rack. Defaults: 42U from U1, ascending, 19", status planned.
type RackCreate struct {
	RoomID       uuid.UUID `json:"room_id"`
	Name         string    `json:"name"`
	AssetTag     *string   `json:"asset_tag"`
	RowLabel     string    `json:"row_label"`
	RowPosition  *int      `json:"row_position"`
	UHeight      *int      `json:"u_height"`
	StartingUnit *int      `json:"starting_unit"`
	DescUnits    bool      `json:"desc_units"`
	WidthIn      *int      `json:"width_in"`
	DepthMM      *int      `json:"depth_mm"`
	MaxWeightKg  *float64  `json:"max_weight_kg"`
	MaxPowerKW   *float64  `json:"max_power_kw"`
	Status       string    `json:"status"`
	Notes        string    `json:"notes"`
}

// RackPatch is a partial update of a rack; room_id moves it to another room of the same site.
type RackPatch struct {
	RoomID       Opt[uuid.UUID] `json:"room_id"`
	Name         Opt[string]    `json:"name"`
	AssetTag     Opt[string]    `json:"asset_tag"`
	RowLabel     Opt[string]    `json:"row_label"`
	RowPosition  Opt[int]       `json:"row_position"`
	UHeight      Opt[int]       `json:"u_height"`
	StartingUnit Opt[int]       `json:"starting_unit"`
	DescUnits    Opt[bool]      `json:"desc_units"`
	WidthIn      Opt[int]       `json:"width_in"`
	DepthMM      Opt[int]       `json:"depth_mm"`
	MaxWeightKg  Opt[float64]   `json:"max_weight_kg"`
	MaxPowerKW   Opt[float64]   `json:"max_power_kw"`
	Status       Opt[string]    `json:"status"`
	Notes        Opt[string]    `json:"notes"`
}

func orDefault(p *int, d int) int {
	if p == nil {
		return d
	}
	return *p
}

func pgText(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func pgInt(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true} //nolint:gosec // validated ranges
}

func pgFloat(p *float64) pgtype.Float8 {
	if p == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *p, Valid: true}
}

func rackFromList(r repo.ListRacksRow) Rack {
	return rackFrom(repo.GetRackRow{ID: r.ID, RoomID: r.RoomID, SiteID: r.SiteID, Name: r.Name, AssetTag: r.AssetTag, RowLabel: r.RowLabel,
		RowPosition: r.RowPosition, UHeight: r.UHeight, StartingUnit: r.StartingUnit, DescUnits: r.DescUnits, WidthIn: r.WidthIn,
		DepthMm: r.DepthMm, MaxWeightKg: r.MaxWeightKg, MaxPowerKw: r.MaxPowerKw, Status: r.Status, Notes: r.Notes, Version: r.Version,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ArchivedAt: r.ArchivedAt, RoomName: r.RoomName, RoomType: r.RoomType,
		FloorID: r.FloorID, FloorName: r.FloorName, BuildingID: r.BuildingID, BuildingName: r.BuildingName, SiteCode: r.SiteCode,
		SiteName: r.SiteName})
}

// ListRacks lists racks in the rack read scope.
func (s *Service) ListRacks(ctx context.Context, f Filter) ([]Rack, int64, error) {
	if err := f.normalize(RackSorts); err != nil {
		return nil, 0, err
	}
	if f.Status != "" {
		if err := oneOf("status", f.Status, RackStatuses); err != nil {
			return nil, 0, err
		}
	}
	sc := scopeOf(ctx, rbac.RackRead)
	rows, err := s.q.ListRacks(ctx, repo.ListRacksParams{AllSites: sc.All, SiteIds: sc.IDs(), IncludeArchived: f.IncludeArchived,
		SiteID: f.SiteID, RoomID: f.RoomID, Status: f.Status, Query: f.Query, Sort: f.Sort, Lim: int32(f.Limit), Off: int32(f.Offset)}) //nolint:gosec // bounded
	if err != nil {
		return nil, 0, err
	}
	out := make([]Rack, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.Total
		out = append(out, rackFromList(r))
	}
	return out, total, nil
}

// GetRack returns a rack in the rack read scope.
func (s *Service) GetRack(ctx context.Context, id uuid.UUID) (Rack, error) {
	sc := scopeOf(ctx, rbac.RackRead)
	r, err := found(s.q.GetRack(ctx, repo.GetRackParams{ID: id, AllSites: sc.All, SiteIds: sc.IDs()}))
	if err != nil {
		return Rack{}, err
	}
	return rackFrom(r), nil
}

// roomForRack loads the target room of a create or move inside the rack manage scope.
func (s *Service) roomForRack(ctx context.Context, id uuid.UUID) (Room, error) {
	rm, err := s.GetRoom(ctx, id)
	return rm, parentCheck("room_id", err, err == nil && scopeOf(ctx, rbac.RackManage).Allows(rm.SiteID), rm.ArchivedAt != nil)
}

// CreateRack creates a rack in a room the caller may manage racks in.
func (s *Service) CreateRack(ctx context.Context, c RackCreate, meta audit.Meta) (Rack, error) {
	in := RackInput{Name: c.Name, AssetTag: c.AssetTag, RowLabel: c.RowLabel, RowPosition: c.RowPosition,
		UHeight: orDefault(c.UHeight, 42), StartingUnit: orDefault(c.StartingUnit, 1), DescUnits: c.DescUnits,
		WidthIn: orDefault(c.WidthIn, 19), DepthMM: c.DepthMM, MaxWeightKg: c.MaxWeightKg, MaxPowerKW: c.MaxPowerKW,
		Status: c.Status, Notes: c.Notes}
	if in.Status == "" {
		in.Status = "planned"
	}
	if err := in.Validate(); err != nil {
		return Rack{}, err
	}
	rm, err := s.roomForRack(ctx, c.RoomID)
	if err != nil {
		s.recordRefusal(ctx, ActionRackCreate, "rack", uuid.Nil, meta, err)
		return Rack{}, err
	}
	sc := scopeOf(ctx, rbac.RackManage)
	id, err := s.q.CreateRack(ctx, repo.CreateRackParams{RoomID: rm.ID, SiteID: rm.SiteID, Name: in.Name, AssetTag: pgText(in.AssetTag),
		RowLabel: in.RowLabel, RowPosition: pgInt(in.RowPosition), UHeight: int32(in.UHeight), StartingUnit: int32(in.StartingUnit), //nolint:gosec // validated
		DescUnits: in.DescUnits, WidthIn: int32(in.WidthIn), DepthMm: pgInt(in.DepthMM), MaxWeightKg: pgFloat(in.MaxWeightKg), //nolint:gosec // validated
		MaxPowerKw: pgFloat(in.MaxPowerKW), Status: in.Status, Notes: in.Notes, AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return Rack{}, mapDBError(err)
	}
	k, err := s.GetRack(ctx, id)
	if err != nil {
		return Rack{}, err
	}
	s.record(ctx, ActionRackCreate, "rack", id, meta, map[string]any{"site_id": k.SiteID, "after": snapshot(k, rackFields)})
	return k, nil
}

// apply merges p onto the current values; it reports whether the rack moves (target room in out).
func (p RackPatch) apply(cur Rack) (RackInput, uuid.UUID, error) {
	in := RackInput{Name: cur.Name, AssetTag: cur.AssetTag, RowLabel: cur.RowLabel, RowPosition: cur.RowPosition,
		UHeight: cur.UHeight, StartingUnit: cur.StartingUnit, DescUnits: cur.DescUnits, WidthIn: cur.WidthIn,
		DepthMM: cur.DepthMM, MaxWeightKg: cur.MaxWeightKg, MaxPowerKW: cur.MaxPowerKW, Status: cur.Status, Notes: cur.Notes}
	room := cur.RoomID
	required := func(field string, set, null bool) error {
		if set && null {
			return invalid(field, "required")
		}
		return nil
	}
	for f, o := range map[string]struct{ set, null bool }{"room_id": {p.RoomID.Set, p.RoomID.Null}, "name": {p.Name.Set, p.Name.Null},
		"row_label": {p.RowLabel.Set, p.RowLabel.Null}, "u_height": {p.UHeight.Set, p.UHeight.Null},
		"starting_unit": {p.StartingUnit.Set, p.StartingUnit.Null}, "desc_units": {p.DescUnits.Set, p.DescUnits.Null},
		"width_in": {p.WidthIn.Set, p.WidthIn.Null}, "status": {p.Status.Set, p.Status.Null}, "notes": {p.Notes.Set, p.Notes.Null}} {
		if err := required(f, o.set, o.null); err != nil {
			return in, room, err
		}
	}
	if p.RoomID.Set {
		room = p.RoomID.Value
	}
	if p.Name.Set {
		in.Name = p.Name.Value
	}
	if p.AssetTag.Set {
		in.AssetTag = nil
		if !p.AssetTag.Null {
			v := p.AssetTag.Value
			in.AssetTag = &v
		}
	}
	if p.RowLabel.Set {
		in.RowLabel = p.RowLabel.Value
	}
	if p.RowPosition.Set {
		in.RowPosition = nil
		if !p.RowPosition.Null {
			v := p.RowPosition.Value
			in.RowPosition = &v
		}
	}
	if p.UHeight.Set {
		in.UHeight = p.UHeight.Value
	}
	if p.StartingUnit.Set {
		in.StartingUnit = p.StartingUnit.Value
	}
	if p.DescUnits.Set {
		in.DescUnits = p.DescUnits.Value
	}
	if p.WidthIn.Set {
		in.WidthIn = p.WidthIn.Value
	}
	if p.DepthMM.Set {
		in.DepthMM = nil
		if !p.DepthMM.Null {
			v := p.DepthMM.Value
			in.DepthMM = &v
		}
	}
	if p.MaxWeightKg.Set {
		in.MaxWeightKg = nil
		if !p.MaxWeightKg.Null {
			v := p.MaxWeightKg.Value
			in.MaxWeightKg = &v
		}
	}
	if p.MaxPowerKW.Set {
		in.MaxPowerKW = nil
		if !p.MaxPowerKW.Null {
			v := p.MaxPowerKW.Value
			in.MaxPowerKW = &v
		}
	}
	if p.Status.Set {
		in.Status = p.Status.Value
	}
	if p.Notes.Set {
		in.Notes = p.Notes.Value
	}
	return in, room, in.Validate()
}

// UpdateRack changes, renames or moves (within its site) a rack the caller may manage.
func (s *Service) UpdateRack(ctx context.Context, id uuid.UUID, version int, p RackPatch, meta audit.Meta) (Rack, error) {
	cur, err := s.GetRack(ctx, id)
	if err != nil {
		return Rack{}, err
	}
	sc := scopeOf(ctx, rbac.RackManage)
	if err := guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version); err != nil {
		s.recordRefusal(ctx, ActionRackUpdate, "rack", id, meta, err)
		return Rack{}, err
	}
	in, roomID, err := p.apply(cur)
	if err != nil {
		return Rack{}, err
	}
	if roomID != cur.RoomID {
		rm, err := s.roomForRack(ctx, roomID)
		if err == nil && rm.SiteID != cur.SiteID {
			err = ErrCrossSite
		}
		if err != nil {
			s.recordRefusal(ctx, ActionRackUpdate, "rack", id, meta, err)
			return Rack{}, err
		}
	}
	_, err = s.q.UpdateRack(ctx, repo.UpdateRackParams{ID: id, Version: int32(version), RoomID: roomID, Name: in.Name, //nolint:gosec // small
		AssetTag: pgText(in.AssetTag), RowLabel: in.RowLabel, RowPosition: pgInt(in.RowPosition), UHeight: int32(in.UHeight), //nolint:gosec // validated
		StartingUnit: int32(in.StartingUnit), DescUnits: in.DescUnits, WidthIn: int32(in.WidthIn), DepthMm: pgInt(in.DepthMM), //nolint:gosec // validated
		MaxWeightKg: pgFloat(in.MaxWeightKg), MaxPowerKw: pgFloat(in.MaxPowerKW), Status: in.Status, Notes: in.Notes,
		AllSites: sc.All, SiteIds: sc.IDs()})
	if errors.Is(err, pgx.ErrNoRows) {
		s.recordRefusal(ctx, ActionRackUpdate, "rack", id, meta, ErrStale)
		return Rack{}, ErrStale
	}
	if err != nil {
		return Rack{}, mapDBError(err)
	}
	next, err := s.GetRack(ctx, id)
	if err != nil {
		return Rack{}, err
	}
	if md := changes(cur, next, rackFields); md != nil {
		md["site_id"] = cur.SiteID
		action := ActionRackUpdate
		if next.Name != cur.Name {
			action = ActionRackRename
		}
		s.record(ctx, action, "rack", id, meta, md)
	}
	return next, nil
}

// ArchiveRack archives a rack (racks have no children in v0.4.1; devices arrive in v0.4.3).
func (s *Service) ArchiveRack(ctx context.Context, id uuid.UUID, version int, meta audit.Meta) (Rack, error) {
	cur, err := s.GetRack(ctx, id)
	if err != nil {
		return Rack{}, err
	}
	sc := scopeOf(ctx, rbac.RackManage)
	err = guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version)
	if err == nil {
		_, err = s.q.ArchiveRack(ctx, repo.ArchiveRackParams{ID: id, Version: int32(version), AllSites: sc.All, SiteIds: sc.IDs()}) //nolint:gosec // small
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrStale
		}
	}
	if err != nil {
		s.recordRefusal(ctx, ActionRackArchive, "rack", id, meta, err)
		return Rack{}, err
	}
	next, err := s.GetRack(ctx, id)
	if err != nil {
		return Rack{}, err
	}
	s.record(ctx, ActionRackArchive, "rack", id, meta, map[string]any{"site_id": cur.SiteID, "before": snapshot(cur, rackFields)})
	return next, nil
}
