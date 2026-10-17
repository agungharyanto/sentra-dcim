// Package service implements the facility hierarchy Site > Building > Floor > Room > Rack (v0.4.1, D-082..D-085).
//
// Every call works inside the caller's site scope (authz, D-084): reads use the read permission's scope, changes the
// manage permission's scope. A row outside the read scope is reported as missing (ErrNotFound → 404); a row the
// caller may read but not change is ErrForbidden (403). Sites are created and archived by global facility managers
// only. Changes require the version the caller last saw (optimistic lock, ETag/If-Match): a different version is
// ErrStale (412). Rows are archived, never deleted; a parent with active children cannot be archived
// (ErrHasChildren, 409). Every change is audited with the changed fields before and after.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/facility/repo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/authz"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// Audit actions.
const (
	ActionSiteCreate      = "site.create"
	ActionSiteUpdate      = "site.update"
	ActionSiteArchive     = "site.archive"
	ActionBuildingCreate  = "building.create"
	ActionBuildingUpdate  = "building.update"
	ActionBuildingArchive = "building.archive"
	ActionFloorCreate     = "floor.create"
	ActionFloorUpdate     = "floor.update"
	ActionFloorArchive    = "floor.archive"
	ActionRoomCreate      = "room.create"
	ActionRoomUpdate      = "room.update"
	ActionRoomArchive     = "room.archive"
	ActionRackCreate      = "rack.create"
	ActionRackUpdate      = "rack.update"
	ActionRackRename      = "rack.rename" // an update that changes the name (D-083), other changes included
	ActionRackArchive     = "rack.archive"
)

// Errors.
var (
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("outside the caller's manage scope")
	ErrGlobalScope    = errors.New("only global facility managers may create or archive sites")
	ErrStale          = errors.New("the row was changed by someone else")
	ErrArchived       = errors.New("the row is archived")
	ErrParentArchived = errors.New("the parent is archived")
	ErrCrossSite      = errors.New("racks cannot move to another site")
)

// DuplicateError is a unique-constraint conflict on a field (409).
type DuplicateError struct{ Field string }

func (e *DuplicateError) Error() string { return "duplicate " + e.Field }

// HasChildrenError refuses to archive a parent with active children (409).
type HasChildrenError struct {
	Kind  string // buildings, floors, rooms
	Count int64
}

func (e *HasChildrenError) Error() string { return fmt.Sprintf("%d active %s", e.Count, e.Kind) }

// Paging limits.
const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Filter selects and orders a list. Parent filters apply where the level has them.
type Filter struct {
	Query           string
	Status          string
	RoomType        string
	SiteID          *uuid.UUID
	BuildingID      *uuid.UUID
	FloorID         *uuid.UUID
	RoomID          *uuid.UUID
	IncludeArchived bool
	Sort            string
	Limit           int
	Offset          int
}

// Sort keys accepted per level ("-" prefix = descending); the default order is by location and name.
var (
	SiteSorts     = []string{"code", "-code", "name", "-name", "updated_at", "-updated_at"}
	BuildingSorts = []string{"name", "-name", "updated_at", "-updated_at"}
	FloorSorts    = []string{"level", "-level", "name", "-name"}
	RoomSorts     = []string{"name", "-name", "room_type"}
	RackSorts     = []string{"name", "-name", "status", "-status", "updated_at", "-updated_at"}
)

func (f *Filter) normalize(sorts []string) error {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		return invalid("offset", "must not be negative")
	}
	if f.Sort != "" && !containsStr(sorts, f.Sort) {
		return invalid("sort", "must be one of: "+strings.Join(sorts, ", "))
	}
	f.Query = escapeLike(strings.TrimSpace(f.Query))
	if len(f.Query) > 100 {
		return invalid("q", "too long")
	}
	return nil
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// escapeLike escapes LIKE wildcards: the queries use ESCAPE '\'.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// DB is what the service needs (pgxpool.Pool).
type DB interface {
	repo.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service owns the facility hierarchy.
type Service struct {
	q     *repo.Queries
	audit *audit.Recorder
	log   *slog.Logger
}

// New creates a Service.
func New(db DB, rec *audit.Recorder, log *slog.Logger) *Service {
	return &Service{q: repo.New(db), audit: rec, log: log}
}

func scopeOf(ctx context.Context, perm rbac.Permission) authz.Scope {
	return authz.FromContext(ctx, perm)
}

func found[T any](v T, err error) (T, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

// mapDBError turns constraint violations into service errors.
func mapDBError(err error) error {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) {
		return err
	}
	switch pe.Code {
	case "23505": // unique_violation
		switch {
		case strings.HasSuffix(pe.ConstraintName, "_asset_tag_uq"):
			return &DuplicateError{Field: "asset_tag"}
		case strings.HasSuffix(pe.ConstraintName, "_code_uq"):
			return &DuplicateError{Field: "code"}
		default:
			return &DuplicateError{Field: "name"}
		}
	case "23514": // check_violation: the service validates first; this is a last line of defence
		return invalid(pe.ConstraintName, "invalid value")
	case "23503": // foreign_key_violation: parent missing or in another site
		return ErrCrossSite
	}
	return err
}

// guard checks a change of an existing row: manage scope, not archived, expected version.
func guard(manage authz.Scope, site uuid.UUID, archived bool, have, want int) error {
	switch {
	case !manage.Allows(site):
		return ErrForbidden
	case archived:
		return ErrArchived
	case have != want:
		return ErrStale
	}
	return nil
}

func (s *Service) record(ctx context.Context, action, targetType string, id uuid.UUID, meta audit.Meta, md map[string]any) {
	_ = s.audit.Record(ctx, audit.Event{Action: action, Result: audit.ResultSuccess, TargetType: targetType,
		TargetID: id.String(), Meta: meta, Metadata: md})
}

// recordRefusal audits refused changes that tell something about concurrent work or misuse: a stale version, a
// parent with children, a site change by a site-scoped manager. Validation errors are not audited.
func (s *Service) recordRefusal(ctx context.Context, action, targetType string, id uuid.UUID, meta audit.Meta, err error) {
	reason := ""
	var hc *HasChildrenError
	switch {
	case errors.Is(err, ErrStale):
		reason = "stale_version"
	case errors.As(err, &hc):
		reason = "has_active_children"
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrGlobalScope):
		reason = "scope"
	case errors.Is(err, ErrCrossSite):
		reason = "cross_site"
	default:
		return
	}
	result := audit.ResultFailure
	if reason == "scope" {
		result = audit.ResultDenied
	}
	target := ""
	if id != uuid.Nil {
		target = id.String()
	}
	_ = s.audit.Record(ctx, audit.Event{Action: action, Result: result, TargetType: targetType, TargetID: target,
		Meta: meta, Metadata: map[string]any{"reason": reason}})
}

// ---------------------------------------------------------------------------------------------------------------
// Sites

// SiteInput is a create or (merged) update of a site.
type SiteInput struct {
	Code    string
	Name    string
	Address string
	Status  string
}

func (in *SiteInput) validate() error {
	var err error
	if in.Code, err = NormalizeSiteCode(in.Code); err != nil {
		return err
	}
	if in.Name, err = normalizeName("name", in.Name); err != nil {
		return err
	}
	if in.Address, err = normalizeText("address", in.Address, 500); err != nil {
		return err
	}
	if in.Status == "" {
		in.Status = "active"
	}
	return oneOf("status", in.Status, SiteStatuses)
}

// ListSites lists the sites in the read scope.
func (s *Service) ListSites(ctx context.Context, f Filter) ([]Site, int64, error) {
	if err := f.normalize(SiteSorts); err != nil {
		return nil, 0, err
	}
	if f.Status != "" {
		if err := oneOf("status", f.Status, SiteStatuses); err != nil {
			return nil, 0, err
		}
	}
	sc := scopeOf(ctx, rbac.FacilityRead)
	rows, err := s.q.ListSites(ctx, repo.ListSitesParams{AllSites: sc.All, SiteIds: sc.IDs(), IncludeArchived: f.IncludeArchived,
		Status: f.Status, Query: f.Query, Sort: f.Sort, Lim: int32(f.Limit), Off: int32(f.Offset)}) //nolint:gosec // bounded by normalize
	if err != nil {
		return nil, 0, err
	}
	out := make([]Site, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.Total
		out = append(out, siteFrom(repo.Site{ID: r.ID, Code: r.Code, Name: r.Name, Address: r.Address, Status: r.Status,
			Version: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ArchivedAt: r.ArchivedAt}))
	}
	return out, total, nil
}

// GetSite returns a site in the read scope.
func (s *Service) GetSite(ctx context.Context, id uuid.UUID) (Site, error) {
	sc := scopeOf(ctx, rbac.FacilityRead)
	r, err := found(s.q.GetSite(ctx, repo.GetSiteParams{ID: id, AllSites: sc.All, SiteIds: sc.IDs()}))
	if err != nil {
		return Site{}, err
	}
	return siteFrom(r), nil
}

// CreateSite creates a site (global facility managers only).
func (s *Service) CreateSite(ctx context.Context, in SiteInput, meta audit.Meta) (Site, error) {
	sc := scopeOf(ctx, rbac.FacilityManage)
	if !sc.All {
		s.recordRefusal(ctx, ActionSiteCreate, "site", uuid.Nil, meta, ErrGlobalScope)
		return Site{}, ErrGlobalScope
	}
	if err := in.validate(); err != nil {
		return Site{}, err
	}
	r, err := s.q.CreateSite(ctx, repo.CreateSiteParams{Code: in.Code, Name: in.Name, Address: in.Address, Status: in.Status, AllSites: sc.All})
	if err != nil {
		return Site{}, mapDBError(err)
	}
	site := siteFrom(r)
	s.record(ctx, ActionSiteCreate, "site", site.ID, meta, map[string]any{"site_id": site.ID, "after": snapshot(site, siteFields)})
	return site, nil
}

// SitePatch is a partial update of a site; the code is fixed once created.
type SitePatch struct {
	Name    Opt[string] `json:"name"`
	Address Opt[string] `json:"address"`
	Status  Opt[string] `json:"status"`
}

// UpdateSite changes a site the caller may manage.
func (s *Service) UpdateSite(ctx context.Context, id uuid.UUID, version int, p SitePatch, meta audit.Meta) (Site, error) {
	cur, err := s.GetSite(ctx, id)
	if err != nil {
		return Site{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	if err := guard(sc, cur.ID, cur.ArchivedAt != nil, cur.Version, version); err != nil {
		s.recordRefusal(ctx, ActionSiteUpdate, "site", id, meta, err)
		return Site{}, err
	}
	in := SiteInput{Code: cur.Code, Name: cur.Name, Address: cur.Address, Status: cur.Status}
	if p.Name.Set {
		in.Name = p.Name.Value
	}
	if p.Address.Set {
		in.Address = p.Address.Value
	}
	if p.Status.Set {
		in.Status = p.Status.Value
	}
	if err := in.validate(); err != nil {
		return Site{}, err
	}
	r, err := s.q.UpdateSite(ctx, repo.UpdateSiteParams{ID: id, Version: int32(version), Name: in.Name, Address: in.Address, //nolint:gosec // small
		Status: in.Status, AllSites: sc.All, SiteIds: sc.IDs()})
	if errors.Is(err, pgx.ErrNoRows) {
		s.recordRefusal(ctx, ActionSiteUpdate, "site", id, meta, ErrStale)
		return Site{}, ErrStale
	}
	if err != nil {
		return Site{}, mapDBError(err)
	}
	next := siteFrom(r)
	if md := changes(cur, next, siteFields); md != nil {
		md["site_id"] = id
		s.record(ctx, ActionSiteUpdate, "site", id, meta, md)
	}
	return next, nil
}

// ArchiveSite archives a site without active buildings (global facility managers only).
func (s *Service) ArchiveSite(ctx context.Context, id uuid.UUID, version int, meta audit.Meta) (Site, error) {
	cur, err := s.GetSite(ctx, id)
	if err != nil {
		return Site{}, err
	}
	sc := scopeOf(ctx, rbac.FacilityManage)
	if !sc.All {
		s.recordRefusal(ctx, ActionSiteArchive, "site", id, meta, ErrGlobalScope)
		return Site{}, ErrGlobalScope
	}
	if err := guard(sc, cur.ID, cur.ArchivedAt != nil, cur.Version, version); err != nil {
		s.recordRefusal(ctx, ActionSiteArchive, "site", id, meta, err)
		return Site{}, err
	}
	n, err := s.q.CountActiveBuildings(ctx, repo.CountActiveBuildingsParams{SiteID: id, AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return Site{}, err
	}
	if n > 0 {
		err := &HasChildrenError{Kind: "buildings", Count: n}
		s.recordRefusal(ctx, ActionSiteArchive, "site", id, meta, err)
		return Site{}, err
	}
	r, err := s.q.ArchiveSite(ctx, repo.ArchiveSiteParams{ID: id, Version: int32(version), AllSites: sc.All, SiteIds: sc.IDs()}) //nolint:gosec // small
	if errors.Is(err, pgx.ErrNoRows) {
		return Site{}, ErrStale
	}
	if err != nil {
		return Site{}, err
	}
	next := siteFrom(r)
	s.record(ctx, ActionSiteArchive, "site", id, meta, map[string]any{"site_id": id, "before": snapshot(cur, siteFields)})
	return next, nil
}
