package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/facility/repo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// parentInScope loads a parent for a create: missing or outside the read scope is a validation error on the parent
// field (the caller picked it), outside the manage scope is ErrForbidden, archived is ErrParentArchived.
func parentCheck(field string, err error, manageOK, archived bool) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return invalid(field, "not found")
	case err != nil:
		return err
	case !manageOK:
		return ErrForbidden
	case archived:
		return ErrParentArchived
	}
	return nil
}

// ---------------------------------------------------------------------------------------------------------------
// Buildings

func buildingFrom(r repo.GetBuildingRow) Building {
	return Building{ID: r.ID, SiteID: r.SiteID, SiteCode: r.SiteCode, SiteName: r.SiteName, Code: r.Code, Name: r.Name,
		Notes: r.Notes, Version: int(r.Version), CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC(),
		ArchivedAt: tsPtr(r.ArchivedAt)}
}

// BuildingInput is a create or (merged) update of a building.
type BuildingInput struct {
	SiteID uuid.UUID
	Code   string
	Name   string
	Notes  string
}

func (in *BuildingInput) validate() error {
	var err error
	if in.Code, err = normalizeBuildingCode(in.Code); err != nil {
		return err
	}
	if in.Name, err = normalizeName("name", in.Name); err != nil {
		return err
	}
	in.Notes, err = normalizeText("notes", in.Notes, 2000)
	return err
}

// ListBuildings lists buildings in the read scope.
func (s *Service) ListBuildings(ctx context.Context, f Filter) ([]Building, int64, error) {
	if err := f.normalize(BuildingSorts); err != nil {
		return nil, 0, err
	}
	sc := scopeOf(ctx, rbac.FacilityRead)
	rows, err := s.q.ListBuildings(ctx, repo.ListBuildingsParams{AllSites: sc.All, SiteIds: sc.IDs(), IncludeArchived: f.IncludeArchived,
		SiteID: f.SiteID, Query: f.Query, Sort: f.Sort, Lim: int32(f.Limit), Off: int32(f.Offset)}) //nolint:gosec // bounded
	if err != nil {
		return nil, 0, err
	}
	out := make([]Building, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.Total
		out = append(out, buildingFrom(repo.GetBuildingRow{ID: r.ID, SiteID: r.SiteID, Code: r.Code, Name: r.Name, Notes: r.Notes,
			Version: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ArchivedAt: r.ArchivedAt, SiteCode: r.SiteCode, SiteName: r.SiteName}))
	}
	return out, total, nil
}

// GetBuilding returns a building in the read scope.
func (s *Service) GetBuilding(ctx context.Context, id uuid.UUID) (Building, error) {
	sc := scopeOf(ctx, rbac.FacilityRead)
	r, err := found(s.q.GetBuilding(ctx, repo.GetBuildingParams{ID: id, AllSites: sc.All, SiteIds: sc.IDs()}))
	if err != nil {
		return Building{}, err
	}
	return buildingFrom(r), nil
}

// CreateBuilding creates a building in a site the caller may manage.
func (s *Service) CreateBuilding(ctx context.Context, in BuildingInput, meta audit.Meta) (Building, error) {
	if err := in.validate(); err != nil {
		return Building{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	site, err := s.GetSite(ctx, in.SiteID)
	if err := parentCheck("site_id", err, err == nil && sc.Allows(site.ID), site.ArchivedAt != nil); err != nil {
		s.recordRefusal(ctx, ActionBuildingCreate, "building", uuid.Nil, meta, err)
		return Building{}, err
	}
	id, err := s.q.CreateBuilding(ctx, repo.CreateBuildingParams{SiteID: in.SiteID, Code: in.Code, Name: in.Name, Notes: in.Notes,
		AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return Building{}, mapDBError(err)
	}
	b, err := s.GetBuilding(ctx, id)
	if err != nil {
		return Building{}, err
	}
	s.record(ctx, ActionBuildingCreate, "building", id, meta, map[string]any{"site_id": b.SiteID, "after": snapshot(b, buildingFields)})
	return b, nil
}

// BuildingPatch is a partial update of a building (it stays in its site).
type BuildingPatch struct {
	Code  Opt[string] `json:"code"`
	Name  Opt[string] `json:"name"`
	Notes Opt[string] `json:"notes"`
}

// UpdateBuilding changes a building the caller may manage.
func (s *Service) UpdateBuilding(ctx context.Context, id uuid.UUID, version int, p BuildingPatch, meta audit.Meta) (Building, error) {
	cur, err := s.GetBuilding(ctx, id)
	if err != nil {
		return Building{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	if err := guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version); err != nil {
		s.recordRefusal(ctx, ActionBuildingUpdate, "building", id, meta, err)
		return Building{}, err
	}
	in := BuildingInput{SiteID: cur.SiteID, Code: cur.Code, Name: cur.Name, Notes: cur.Notes}
	if p.Code.Set {
		in.Code = p.Code.Value
	}
	if p.Name.Set {
		in.Name = p.Name.Value
	}
	if p.Notes.Set {
		in.Notes = p.Notes.Value
	}
	if err := in.validate(); err != nil {
		return Building{}, err
	}
	_, err = s.q.UpdateBuilding(ctx, repo.UpdateBuildingParams{ID: id, Version: int32(version), Code: in.Code, Name: in.Name, //nolint:gosec // small
		Notes: in.Notes, AllSites: sc.All, SiteIds: sc.IDs()})
	if errors.Is(err, pgx.ErrNoRows) {
		s.recordRefusal(ctx, ActionBuildingUpdate, "building", id, meta, ErrStale)
		return Building{}, ErrStale
	}
	if err != nil {
		return Building{}, mapDBError(err)
	}
	next, err := s.GetBuilding(ctx, id)
	if err != nil {
		return Building{}, err
	}
	if md := changes(cur, next, buildingFields); md != nil {
		md["site_id"] = cur.SiteID
		s.record(ctx, ActionBuildingUpdate, "building", id, meta, md)
	}
	return next, nil
}

// ArchiveBuilding archives a building without active floors.
func (s *Service) ArchiveBuilding(ctx context.Context, id uuid.UUID, version int, meta audit.Meta) (Building, error) {
	cur, err := s.GetBuilding(ctx, id)
	if err != nil {
		return Building{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	err = guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version)
	if err == nil {
		var n int64
		if n, err = s.q.CountActiveFloors(ctx, repo.CountActiveFloorsParams{BuildingID: id, AllSites: sc.All, SiteIds: sc.IDs()}); err == nil && n > 0 {
			err = &HasChildrenError{Kind: "floors", Count: n}
		}
	}
	if err == nil {
		_, err = s.q.ArchiveBuilding(ctx, repo.ArchiveBuildingParams{ID: id, Version: int32(version), AllSites: sc.All, SiteIds: sc.IDs()}) //nolint:gosec // small
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrStale
		}
	}
	if err != nil {
		s.recordRefusal(ctx, ActionBuildingArchive, "building", id, meta, err)
		return Building{}, err
	}
	next, err := s.GetBuilding(ctx, id)
	if err != nil {
		return Building{}, err
	}
	s.record(ctx, ActionBuildingArchive, "building", id, meta, map[string]any{"site_id": cur.SiteID, "before": snapshot(cur, buildingFields)})
	return next, nil
}

// ---------------------------------------------------------------------------------------------------------------
// Floors

func floorFrom(r repo.GetFloorRow) Floor {
	return Floor{ID: r.ID, BuildingID: r.BuildingID, BuildingName: r.BuildingName, SiteID: r.SiteID, SiteCode: r.SiteCode,
		Name: r.Name, Level: int(r.Level), Notes: r.Notes, Version: int(r.Version), CreatedAt: r.CreatedAt.Time.UTC(),
		UpdatedAt: r.UpdatedAt.Time.UTC(), ArchivedAt: tsPtr(r.ArchivedAt)}
}

// FloorInput is a create or (merged) update of a floor.
type FloorInput struct {
	BuildingID uuid.UUID
	Name       string
	Level      int
	Notes      string
}

func (in *FloorInput) validate() error {
	var err error
	if in.Name, err = normalizeName("name", in.Name); err != nil {
		return err
	}
	if err := between("level", in.Level, -20, 200); err != nil {
		return err
	}
	in.Notes, err = normalizeText("notes", in.Notes, 2000)
	return err
}

// ListFloors lists floors in the read scope.
func (s *Service) ListFloors(ctx context.Context, f Filter) ([]Floor, int64, error) {
	if err := f.normalize(FloorSorts); err != nil {
		return nil, 0, err
	}
	sc := scopeOf(ctx, rbac.FacilityRead)
	rows, err := s.q.ListFloors(ctx, repo.ListFloorsParams{AllSites: sc.All, SiteIds: sc.IDs(), IncludeArchived: f.IncludeArchived,
		SiteID: f.SiteID, BuildingID: f.BuildingID, Query: f.Query, Sort: f.Sort, Lim: int32(f.Limit), Off: int32(f.Offset)}) //nolint:gosec // bounded
	if err != nil {
		return nil, 0, err
	}
	out := make([]Floor, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.Total
		out = append(out, floorFrom(repo.GetFloorRow{ID: r.ID, BuildingID: r.BuildingID, SiteID: r.SiteID, Name: r.Name, Level: r.Level,
			Notes: r.Notes, Version: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ArchivedAt: r.ArchivedAt,
			BuildingName: r.BuildingName, SiteCode: r.SiteCode}))
	}
	return out, total, nil
}

// GetFloor returns a floor in the read scope.
func (s *Service) GetFloor(ctx context.Context, id uuid.UUID) (Floor, error) {
	sc := scopeOf(ctx, rbac.FacilityRead)
	r, err := found(s.q.GetFloor(ctx, repo.GetFloorParams{ID: id, AllSites: sc.All, SiteIds: sc.IDs()}))
	if err != nil {
		return Floor{}, err
	}
	return floorFrom(r), nil
}

// CreateFloor creates a floor in a building the caller may manage.
func (s *Service) CreateFloor(ctx context.Context, in FloorInput, meta audit.Meta) (Floor, error) {
	if err := in.validate(); err != nil {
		return Floor{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	b, err := s.GetBuilding(ctx, in.BuildingID)
	if err := parentCheck("building_id", err, err == nil && sc.Allows(b.SiteID), b.ArchivedAt != nil); err != nil {
		s.recordRefusal(ctx, ActionFloorCreate, "floor", uuid.Nil, meta, err)
		return Floor{}, err
	}
	id, err := s.q.CreateFloor(ctx, repo.CreateFloorParams{BuildingID: b.ID, SiteID: b.SiteID, Name: in.Name, Level: int32(in.Level), //nolint:gosec // validated
		Notes: in.Notes, AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return Floor{}, mapDBError(err)
	}
	fl, err := s.GetFloor(ctx, id)
	if err != nil {
		return Floor{}, err
	}
	s.record(ctx, ActionFloorCreate, "floor", id, meta, map[string]any{"site_id": fl.SiteID, "after": snapshot(fl, floorFields)})
	return fl, nil
}

// FloorPatch is a partial update of a floor (it stays in its building).
type FloorPatch struct {
	Name  Opt[string] `json:"name"`
	Level Opt[int]    `json:"level"`
	Notes Opt[string] `json:"notes"`
}

// UpdateFloor changes a floor the caller may manage.
func (s *Service) UpdateFloor(ctx context.Context, id uuid.UUID, version int, p FloorPatch, meta audit.Meta) (Floor, error) {
	cur, err := s.GetFloor(ctx, id)
	if err != nil {
		return Floor{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	if err := guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version); err != nil {
		s.recordRefusal(ctx, ActionFloorUpdate, "floor", id, meta, err)
		return Floor{}, err
	}
	in := FloorInput{BuildingID: cur.BuildingID, Name: cur.Name, Level: cur.Level, Notes: cur.Notes}
	if p.Name.Set {
		in.Name = p.Name.Value
	}
	if p.Level.Set {
		if p.Level.Null {
			return Floor{}, invalid("level", "required")
		}
		in.Level = p.Level.Value
	}
	if p.Notes.Set {
		in.Notes = p.Notes.Value
	}
	if err := in.validate(); err != nil {
		return Floor{}, err
	}
	_, err = s.q.UpdateFloor(ctx, repo.UpdateFloorParams{ID: id, Version: int32(version), Name: in.Name, Level: int32(in.Level), //nolint:gosec // validated
		Notes: in.Notes, AllSites: sc.All, SiteIds: sc.IDs()})
	if errors.Is(err, pgx.ErrNoRows) {
		s.recordRefusal(ctx, ActionFloorUpdate, "floor", id, meta, ErrStale)
		return Floor{}, ErrStale
	}
	if err != nil {
		return Floor{}, mapDBError(err)
	}
	next, err := s.GetFloor(ctx, id)
	if err != nil {
		return Floor{}, err
	}
	if md := changes(cur, next, floorFields); md != nil {
		md["site_id"] = cur.SiteID
		s.record(ctx, ActionFloorUpdate, "floor", id, meta, md)
	}
	return next, nil
}

// ArchiveFloor archives a floor without active rooms.
func (s *Service) ArchiveFloor(ctx context.Context, id uuid.UUID, version int, meta audit.Meta) (Floor, error) {
	cur, err := s.GetFloor(ctx, id)
	if err != nil {
		return Floor{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	err = guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version)
	if err == nil {
		var n int64
		if n, err = s.q.CountActiveRooms(ctx, repo.CountActiveRoomsParams{FloorID: id, AllSites: sc.All, SiteIds: sc.IDs()}); err == nil && n > 0 {
			err = &HasChildrenError{Kind: "rooms", Count: n}
		}
	}
	if err == nil {
		_, err = s.q.ArchiveFloor(ctx, repo.ArchiveFloorParams{ID: id, Version: int32(version), AllSites: sc.All, SiteIds: sc.IDs()}) //nolint:gosec // small
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrStale
		}
	}
	if err != nil {
		s.recordRefusal(ctx, ActionFloorArchive, "floor", id, meta, err)
		return Floor{}, err
	}
	next, err := s.GetFloor(ctx, id)
	if err != nil {
		return Floor{}, err
	}
	s.record(ctx, ActionFloorArchive, "floor", id, meta, map[string]any{"site_id": cur.SiteID, "before": snapshot(cur, floorFields)})
	return next, nil
}

// ---------------------------------------------------------------------------------------------------------------
// Rooms

func roomFrom(r repo.GetRoomRow) Room {
	return Room{ID: r.ID, FloorID: r.FloorID, FloorName: r.FloorName, BuildingID: r.BuildingID, BuildingName: r.BuildingName,
		SiteID: r.SiteID, SiteCode: r.SiteCode, Name: r.Name, RoomType: r.RoomType, Notes: r.Notes, RackCount: int(r.RackCount),
		Version: int(r.Version), CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC(), ArchivedAt: tsPtr(r.ArchivedAt)}
}

// RoomInput is a create or (merged) update of a room.
type RoomInput struct {
	FloorID  uuid.UUID
	Name     string
	RoomType string
	Notes    string
}

func (in *RoomInput) validate() error {
	var err error
	if in.Name, err = normalizeName("name", in.Name); err != nil {
		return err
	}
	if in.RoomType == "" {
		in.RoomType = "whitespace"
	}
	if err := oneOf("room_type", in.RoomType, RoomTypes); err != nil {
		return err
	}
	in.Notes, err = normalizeText("notes", in.Notes, 2000)
	return err
}

// ListRooms lists rooms in the read scope.
func (s *Service) ListRooms(ctx context.Context, f Filter) ([]Room, int64, error) {
	if err := f.normalize(RoomSorts); err != nil {
		return nil, 0, err
	}
	if f.RoomType != "" {
		if err := oneOf("room_type", f.RoomType, RoomTypes); err != nil {
			return nil, 0, err
		}
	}
	sc := scopeOf(ctx, rbac.FacilityRead)
	rows, err := s.q.ListRooms(ctx, repo.ListRoomsParams{AllSites: sc.All, SiteIds: sc.IDs(), IncludeArchived: f.IncludeArchived,
		SiteID: f.SiteID, FloorID: f.FloorID, RoomType: f.RoomType, Query: f.Query, Sort: f.Sort, Lim: int32(f.Limit), Off: int32(f.Offset)}) //nolint:gosec // bounded
	if err != nil {
		return nil, 0, err
	}
	out := make([]Room, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.Total
		out = append(out, roomFrom(repo.GetRoomRow{ID: r.ID, FloorID: r.FloorID, SiteID: r.SiteID, Name: r.Name, RoomType: r.RoomType,
			Notes: r.Notes, Version: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ArchivedAt: r.ArchivedAt,
			FloorName: r.FloorName, BuildingID: r.BuildingID, BuildingName: r.BuildingName, SiteCode: r.SiteCode, RackCount: r.RackCount}))
	}
	return out, total, nil
}

// GetRoom returns a room in the read scope.
func (s *Service) GetRoom(ctx context.Context, id uuid.UUID) (Room, error) {
	sc := scopeOf(ctx, rbac.FacilityRead)
	r, err := found(s.q.GetRoom(ctx, repo.GetRoomParams{ID: id, AllSites: sc.All, SiteIds: sc.IDs()}))
	if err != nil {
		return Room{}, err
	}
	return roomFrom(r), nil
}

// CreateRoom creates a room on a floor the caller may manage.
func (s *Service) CreateRoom(ctx context.Context, in RoomInput, meta audit.Meta) (Room, error) {
	if err := in.validate(); err != nil {
		return Room{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	fl, err := s.GetFloor(ctx, in.FloorID)
	if err := parentCheck("floor_id", err, err == nil && sc.Allows(fl.SiteID), fl.ArchivedAt != nil); err != nil {
		s.recordRefusal(ctx, ActionRoomCreate, "room", uuid.Nil, meta, err)
		return Room{}, err
	}
	id, err := s.q.CreateRoom(ctx, repo.CreateRoomParams{FloorID: fl.ID, SiteID: fl.SiteID, Name: in.Name, RoomType: in.RoomType,
		Notes: in.Notes, AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return Room{}, mapDBError(err)
	}
	rm, err := s.GetRoom(ctx, id)
	if err != nil {
		return Room{}, err
	}
	s.record(ctx, ActionRoomCreate, "room", id, meta, map[string]any{"site_id": rm.SiteID, "after": snapshot(rm, roomFields)})
	return rm, nil
}

// RoomPatch is a partial update of a room (it stays on its floor).
type RoomPatch struct {
	Name     Opt[string] `json:"name"`
	RoomType Opt[string] `json:"room_type"`
	Notes    Opt[string] `json:"notes"`
}

// UpdateRoom changes a room the caller may manage.
func (s *Service) UpdateRoom(ctx context.Context, id uuid.UUID, version int, p RoomPatch, meta audit.Meta) (Room, error) {
	cur, err := s.GetRoom(ctx, id)
	if err != nil {
		return Room{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	if err := guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version); err != nil {
		s.recordRefusal(ctx, ActionRoomUpdate, "room", id, meta, err)
		return Room{}, err
	}
	in := RoomInput{FloorID: cur.FloorID, Name: cur.Name, RoomType: cur.RoomType, Notes: cur.Notes}
	if p.Name.Set {
		in.Name = p.Name.Value
	}
	if p.RoomType.Set {
		in.RoomType = p.RoomType.Value
		if in.RoomType == "" {
			return Room{}, invalid("room_type", "required")
		}
	}
	if p.Notes.Set {
		in.Notes = p.Notes.Value
	}
	if err := in.validate(); err != nil {
		return Room{}, err
	}
	_, err = s.q.UpdateRoom(ctx, repo.UpdateRoomParams{ID: id, Version: int32(version), Name: in.Name, RoomType: in.RoomType, //nolint:gosec // small
		Notes: in.Notes, AllSites: sc.All, SiteIds: sc.IDs()})
	if errors.Is(err, pgx.ErrNoRows) {
		s.recordRefusal(ctx, ActionRoomUpdate, "room", id, meta, ErrStale)
		return Room{}, ErrStale
	}
	if err != nil {
		return Room{}, mapDBError(err)
	}
	next, err := s.GetRoom(ctx, id)
	if err != nil {
		return Room{}, err
	}
	if md := changes(cur, next, roomFields); md != nil {
		md["site_id"] = cur.SiteID
		s.record(ctx, ActionRoomUpdate, "room", id, meta, md)
	}
	return next, nil
}

// ArchiveRoom archives a room without active racks.
func (s *Service) ArchiveRoom(ctx context.Context, id uuid.UUID, version int, meta audit.Meta) (Room, error) {
	cur, err := s.GetRoom(ctx, id)
	if err != nil {
		return Room{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	err = guard(sc, cur.SiteID, cur.ArchivedAt != nil, cur.Version, version)
	if err == nil && cur.RackCount > 0 {
		err = &HasChildrenError{Kind: "racks", Count: int64(cur.RackCount)}
	}
	if err == nil {
		_, err = s.q.ArchiveRoom(ctx, repo.ArchiveRoomParams{ID: id, Version: int32(version), AllSites: sc.All, SiteIds: sc.IDs()}) //nolint:gosec // small
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrStale
		}
	}
	if err != nil {
		s.recordRefusal(ctx, ActionRoomArchive, "room", id, meta, err)
		return Room{}, err
	}
	next, err := s.GetRoom(ctx, id)
	if err != nil {
		return Room{}, err
	}
	s.record(ctx, ActionRoomArchive, "room", id, meta, map[string]any{"site_id": cur.SiteID, "before": snapshot(cur, roomFields)})
	return next, nil
}
