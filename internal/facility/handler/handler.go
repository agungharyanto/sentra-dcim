// Package handler exposes the facility hierarchy (v0.4.1): /api/v1/sites, /buildings, /floors, /rooms, /racks and
// /facility/tree. Reads need facility.read (rack.read for racks), changes facility.manage (rack.manage); the service
// narrows every call to the caller's site scope (D-084).
//
// Optimistic locking (D-085): single-object responses carry ETag "<version>"; PATCH and archive require If-Match
// with that value — missing is 428 precondition_required, different is 412 version_conflict.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/facility/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// Stable error codes (translated in the UI: errors:code.<code>).
const (
	CodeValidation      = "validation_failed"
	CodeInvalidRequest  = "invalid_request"
	CodeNotFound        = "not_found"
	CodeDuplicateName   = "duplicate_name"
	CodeDuplicateCode   = "duplicate_code"
	CodeDuplicateTag    = "duplicate_asset_tag"
	CodeHasChildren     = "has_active_children"
	CodeArchived        = "archived"
	CodeParentArchived  = "parent_archived"
	CodeCrossSite       = "cross_site_move"
	CodePrecondition    = "precondition_required"
	CodeVersionConflict = "version_conflict"
	CodeGlobalScope     = "global_scope_required"
	CodeOutOfScope      = "out_of_scope"
)

// Handler serves the facility API.
type Handler struct {
	svc *service.Service
	log *slog.Logger
}

// New creates a Handler.
func New(svc *service.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes registers the facility routes behind the session.
func (h *Handler) Routes(r chi.Router, require func(rbac.Permission) func(http.Handler) http.Handler) {
	read, manage := require(rbac.FacilityRead), require(rbac.FacilityManage)
	rackRead, rackManage := require(rbac.RackRead), require(rbac.RackManage)
	r.With(read).Get("/facility/tree", h.tree)

	r.With(read).Get("/sites", h.listSites)
	r.With(manage).Post("/sites", h.createSite)
	r.With(read).Get("/sites/{id}", h.getSite)
	r.With(manage).Patch("/sites/{id}", h.updateSite)
	r.With(manage).Post("/sites/{id}/archive", h.archiveSite)

	r.With(read).Get("/buildings", h.listBuildings)
	r.With(manage).Post("/buildings", h.createBuilding)
	r.With(read).Get("/buildings/{id}", h.getBuilding)
	r.With(manage).Patch("/buildings/{id}", h.updateBuilding)
	r.With(manage).Post("/buildings/{id}/archive", h.archiveBuilding)

	r.With(read).Get("/floors", h.listFloors)
	r.With(manage).Post("/floors", h.createFloor)
	r.With(read).Get("/floors/{id}", h.getFloor)
	r.With(manage).Patch("/floors/{id}", h.updateFloor)
	r.With(manage).Post("/floors/{id}/archive", h.archiveFloor)

	r.With(read).Get("/rooms", h.listRooms)
	r.With(manage).Post("/rooms", h.createRoom)
	r.With(read).Get("/rooms/{id}", h.getRoom)
	r.With(manage).Patch("/rooms/{id}", h.updateRoom)
	r.With(manage).Post("/rooms/{id}/archive", h.archiveRoom)

	r.With(rackRead).Get("/racks", h.listRacks)
	r.With(rackManage).Post("/racks", h.createRack)
	r.With(rackRead).Get("/racks/{id}", h.getRack)
	r.With(rackManage).Patch("/racks/{id}", h.updateRack)
	r.With(rackManage).Post("/racks/{id}/archive", h.archiveRack)
}

// ---------------------------------------------------------------------------------------------------------------
// Helpers

type listResponse[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpserver.Error(w, r, http.StatusBadRequest, CodeInvalidRequest, "invalid JSON body")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "not found")
		return uuid.Nil, false
	}
	return id, true
}

// ETag is the strong entity tag of a version.
func ETag(version int) string { return `"` + strconv.Itoa(version) + `"` }

// ifMatch reads the version from If-Match ("3" or W/"3"); a missing or wildcard header is 428.
func ifMatch(w http.ResponseWriter, r *http.Request) (int, bool) {
	v := strings.TrimSpace(r.Header.Get("If-Match"))
	if v == "" || v == "*" {
		httpserver.Error(w, r, http.StatusPreconditionRequired, CodePrecondition, "If-Match with the current ETag is required")
		return 0, false
	}
	v = strings.TrimPrefix(v, "W/")
	n, err := strconv.Atoi(strings.Trim(v, `"`))
	if err != nil || n < 1 || !strings.HasPrefix(v, `"`) || !strings.HasSuffix(v, `"`) {
		httpserver.Error(w, r, http.StatusPreconditionFailed, CodeVersionConflict, "If-Match does not match the current version")
		return 0, false
	}
	return n, true
}

func one(w http.ResponseWriter, code, version int, v any) {
	w.Header().Set("ETag", ETag(version))
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, code, v)
}

func optUUID(w http.ResponseWriter, r *http.Request, name string) (*uuid.UUID, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, true
	}
	id, err := uuid.Parse(s)
	if err != nil {
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, name, "must be a UUID")
		return nil, false
	}
	return &id, true
}

// filter parses the common list parameters: q, status, room_type, site_id, building_id, floor_id, room_id,
// include_archived, sort, limit, offset.
func filter(w http.ResponseWriter, r *http.Request) (service.Filter, bool) {
	q := r.URL.Query()
	f := service.Filter{Query: q.Get("q"), Status: q.Get("status"), RoomType: q.Get("room_type"), Sort: q.Get("sort")}
	var ok bool
	for _, p := range []struct {
		name string
		dst  **uuid.UUID
	}{{"site_id", &f.SiteID}, {"building_id", &f.BuildingID}, {"floor_id", &f.FloorID}, {"room_id", &f.RoomID}} {
		if *p.dst, ok = optUUID(w, r, p.name); !ok {
			return f, false
		}
	}
	for _, p := range []struct {
		name string
		dst  *int
	}{{"limit", &f.Limit}, {"offset", &f.Offset}} {
		if s := q.Get(p.name); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil {
				httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, p.name, "must be an integer")
				return f, false
			}
			*p.dst = n
		}
	}
	switch q.Get("include_archived") {
	case "", "false":
	case "true":
		f.IncludeArchived = true
	default:
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, "include_archived", "must be true or false")
		return f, false
	}
	return f, true
}

func list[T any](w http.ResponseWriter, r *http.Request, h *Handler, what string, fn func(service.Filter) ([]T, int64, error)) {
	f, ok := filter(w, r)
	if !ok {
		return
	}
	items, total, err := fn(f)
	if err != nil {
		h.fail(w, r, what, err)
		return
	}
	limit := f.Limit
	if limit <= 0 {
		limit = service.DefaultLimit
	} else if limit > service.MaxLimit {
		limit = service.MaxLimit
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, listResponse[T]{Items: items, Total: total, Limit: limit, Offset: f.Offset})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, what string, err error) {
	var ve *service.ValidationError
	var de *service.DuplicateError
	var hc *service.HasChildrenError
	switch {
	case errors.As(err, &ve):
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, ve.Field, ve.Reason)
	case errors.As(err, &de):
		code := CodeDuplicateName
		switch de.Field {
		case "code":
			code = CodeDuplicateCode
		case "asset_tag":
			code = CodeDuplicateTag
		}
		httpserver.FieldError(w, r, http.StatusConflict, code, de.Field, "already in use")
	case errors.As(err, &hc):
		httpserver.Error(w, r, http.StatusConflict, CodeHasChildren, "archive the "+hc.Kind+" first ("+strconv.FormatInt(hc.Count, 10)+" active)")
	case errors.Is(err, service.ErrNotFound):
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "not found")
	case errors.Is(err, service.ErrStale):
		httpserver.Error(w, r, http.StatusPreconditionFailed, CodeVersionConflict, "changed by someone else: reload")
	case errors.Is(err, service.ErrArchived):
		httpserver.Error(w, r, http.StatusConflict, CodeArchived, "archived")
	case errors.Is(err, service.ErrParentArchived):
		httpserver.Error(w, r, http.StatusUnprocessableEntity, CodeParentArchived, "the parent is archived")
	case errors.Is(err, service.ErrCrossSite):
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeCrossSite, "room_id", "racks cannot move to another site")
	case errors.Is(err, service.ErrGlobalScope):
		httpserver.Error(w, r, http.StatusForbidden, CodeGlobalScope, "only global facility managers may do this")
	case errors.Is(err, service.ErrForbidden):
		httpserver.Error(w, r, http.StatusForbidden, CodeOutOfScope, "you may not change this site")
	default:
		h.log.ErrorContext(r.Context(), what, "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *Handler) tree(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.Tree(r.Context())
	if err != nil {
		h.fail(w, r, "facility tree", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, map[string]any{"sites": t})
}

// crud wires the get/update/archive handlers of one level.
type crud[T any, P any] struct {
	h       *Handler
	what    string
	get     func(r *http.Request, id uuid.UUID) (T, error)
	update  func(r *http.Request, id uuid.UUID, version int, p P) (T, error)
	archive func(r *http.Request, id uuid.UUID, version int) (T, error)
	version func(T) int
}

func (c crud[T, P]) getH(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, err := c.get(r, id)
	if err != nil {
		c.h.fail(w, r, "get "+c.what, err)
		return
	}
	one(w, http.StatusOK, c.version(v), v)
}

func (c crud[T, P]) updateH(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	version, ok := ifMatch(w, r)
	if !ok {
		return
	}
	var p P
	if !decode(w, r, &p) {
		return
	}
	v, err := c.update(r, id, version, p)
	if err != nil {
		c.h.fail(w, r, "update "+c.what, err)
		return
	}
	one(w, http.StatusOK, c.version(v), v)
}

func (c crud[T, P]) archiveH(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	version, ok := ifMatch(w, r)
	if !ok {
		return
	}
	v, err := c.archive(r, id, version)
	if err != nil {
		c.h.fail(w, r, "archive "+c.what, err)
		return
	}
	one(w, http.StatusOK, c.version(v), v)
}

func meta(r *http.Request) audit.Meta { return audit.MetaFromRequest(r) }

// ---------------------------------------------------------------------------------------------------------------
// Sites

func (h *Handler) sites() crud[service.Site, service.SitePatch] {
	return crud[service.Site, service.SitePatch]{h: h, what: "site",
		get: func(r *http.Request, id uuid.UUID) (service.Site, error) { return h.svc.GetSite(r.Context(), id) },
		update: func(r *http.Request, id uuid.UUID, v int, p service.SitePatch) (service.Site, error) {
			return h.svc.UpdateSite(r.Context(), id, v, p, meta(r))
		},
		archive: func(r *http.Request, id uuid.UUID, v int) (service.Site, error) {
			return h.svc.ArchiveSite(r.Context(), id, v, meta(r))
		},
		version: func(s service.Site) int { return s.Version }}
}

func (h *Handler) listSites(w http.ResponseWriter, r *http.Request) {
	list(w, r, h, "list sites", func(f service.Filter) ([]service.Site, int64, error) { return h.svc.ListSites(r.Context(), f) })
}
func (h *Handler) getSite(w http.ResponseWriter, r *http.Request)     { h.sites().getH(w, r) }
func (h *Handler) updateSite(w http.ResponseWriter, r *http.Request)  { h.sites().updateH(w, r) }
func (h *Handler) archiveSite(w http.ResponseWriter, r *http.Request) { h.sites().archiveH(w, r) }

func (h *Handler) createSite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Address string `json:"address"`
		Status  string `json:"status"`
	}
	if !decode(w, r, &in) {
		return
	}
	s, err := h.svc.CreateSite(r.Context(), service.SiteInput{Code: in.Code, Name: in.Name, Address: in.Address, Status: in.Status}, meta(r))
	if err != nil {
		h.fail(w, r, "create site", err)
		return
	}
	one(w, http.StatusCreated, s.Version, s)
}

// ---------------------------------------------------------------------------------------------------------------
// Buildings

func (h *Handler) buildings() crud[service.Building, service.BuildingPatch] {
	return crud[service.Building, service.BuildingPatch]{h: h, what: "building",
		get: func(r *http.Request, id uuid.UUID) (service.Building, error) {
			return h.svc.GetBuilding(r.Context(), id)
		},
		update: func(r *http.Request, id uuid.UUID, v int, p service.BuildingPatch) (service.Building, error) {
			return h.svc.UpdateBuilding(r.Context(), id, v, p, meta(r))
		},
		archive: func(r *http.Request, id uuid.UUID, v int) (service.Building, error) {
			return h.svc.ArchiveBuilding(r.Context(), id, v, meta(r))
		},
		version: func(b service.Building) int { return b.Version }}
}

func (h *Handler) listBuildings(w http.ResponseWriter, r *http.Request) {
	list(w, r, h, "list buildings", func(f service.Filter) ([]service.Building, int64, error) {
		return h.svc.ListBuildings(r.Context(), f)
	})
}
func (h *Handler) getBuilding(w http.ResponseWriter, r *http.Request)    { h.buildings().getH(w, r) }
func (h *Handler) updateBuilding(w http.ResponseWriter, r *http.Request) { h.buildings().updateH(w, r) }
func (h *Handler) archiveBuilding(w http.ResponseWriter, r *http.Request) {
	h.buildings().archiveH(w, r)
}

func (h *Handler) createBuilding(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SiteID uuid.UUID `json:"site_id"`
		Code   string    `json:"code"`
		Name   string    `json:"name"`
		Notes  string    `json:"notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	b, err := h.svc.CreateBuilding(r.Context(), service.BuildingInput{SiteID: in.SiteID, Code: in.Code, Name: in.Name, Notes: in.Notes}, meta(r))
	if err != nil {
		h.fail(w, r, "create building", err)
		return
	}
	one(w, http.StatusCreated, b.Version, b)
}

// ---------------------------------------------------------------------------------------------------------------
// Floors

func (h *Handler) floors() crud[service.Floor, service.FloorPatch] {
	return crud[service.Floor, service.FloorPatch]{h: h, what: "floor",
		get: func(r *http.Request, id uuid.UUID) (service.Floor, error) { return h.svc.GetFloor(r.Context(), id) },
		update: func(r *http.Request, id uuid.UUID, v int, p service.FloorPatch) (service.Floor, error) {
			return h.svc.UpdateFloor(r.Context(), id, v, p, meta(r))
		},
		archive: func(r *http.Request, id uuid.UUID, v int) (service.Floor, error) {
			return h.svc.ArchiveFloor(r.Context(), id, v, meta(r))
		},
		version: func(f service.Floor) int { return f.Version }}
}

func (h *Handler) listFloors(w http.ResponseWriter, r *http.Request) {
	list(w, r, h, "list floors", func(f service.Filter) ([]service.Floor, int64, error) { return h.svc.ListFloors(r.Context(), f) })
}
func (h *Handler) getFloor(w http.ResponseWriter, r *http.Request)     { h.floors().getH(w, r) }
func (h *Handler) updateFloor(w http.ResponseWriter, r *http.Request)  { h.floors().updateH(w, r) }
func (h *Handler) archiveFloor(w http.ResponseWriter, r *http.Request) { h.floors().archiveH(w, r) }

func (h *Handler) createFloor(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BuildingID uuid.UUID `json:"building_id"`
		Name       string    `json:"name"`
		Level      int       `json:"level"`
		Notes      string    `json:"notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	f, err := h.svc.CreateFloor(r.Context(), service.FloorInput{BuildingID: in.BuildingID, Name: in.Name, Level: in.Level, Notes: in.Notes}, meta(r))
	if err != nil {
		h.fail(w, r, "create floor", err)
		return
	}
	one(w, http.StatusCreated, f.Version, f)
}

// ---------------------------------------------------------------------------------------------------------------
// Rooms

func (h *Handler) rooms() crud[service.Room, service.RoomPatch] {
	return crud[service.Room, service.RoomPatch]{h: h, what: "room",
		get: func(r *http.Request, id uuid.UUID) (service.Room, error) { return h.svc.GetRoom(r.Context(), id) },
		update: func(r *http.Request, id uuid.UUID, v int, p service.RoomPatch) (service.Room, error) {
			return h.svc.UpdateRoom(r.Context(), id, v, p, meta(r))
		},
		archive: func(r *http.Request, id uuid.UUID, v int) (service.Room, error) {
			return h.svc.ArchiveRoom(r.Context(), id, v, meta(r))
		},
		version: func(rm service.Room) int { return rm.Version }}
}

func (h *Handler) listRooms(w http.ResponseWriter, r *http.Request) {
	list(w, r, h, "list rooms", func(f service.Filter) ([]service.Room, int64, error) { return h.svc.ListRooms(r.Context(), f) })
}
func (h *Handler) getRoom(w http.ResponseWriter, r *http.Request)     { h.rooms().getH(w, r) }
func (h *Handler) updateRoom(w http.ResponseWriter, r *http.Request)  { h.rooms().updateH(w, r) }
func (h *Handler) archiveRoom(w http.ResponseWriter, r *http.Request) { h.rooms().archiveH(w, r) }

func (h *Handler) createRoom(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FloorID  uuid.UUID `json:"floor_id"`
		Name     string    `json:"name"`
		RoomType string    `json:"room_type"`
		Notes    string    `json:"notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	rm, err := h.svc.CreateRoom(r.Context(), service.RoomInput{FloorID: in.FloorID, Name: in.Name, RoomType: in.RoomType, Notes: in.Notes}, meta(r))
	if err != nil {
		h.fail(w, r, "create room", err)
		return
	}
	one(w, http.StatusCreated, rm.Version, rm)
}

// ---------------------------------------------------------------------------------------------------------------
// Racks

func (h *Handler) racks() crud[service.Rack, service.RackPatch] {
	return crud[service.Rack, service.RackPatch]{h: h, what: "rack",
		get: func(r *http.Request, id uuid.UUID) (service.Rack, error) { return h.svc.GetRack(r.Context(), id) },
		update: func(r *http.Request, id uuid.UUID, v int, p service.RackPatch) (service.Rack, error) {
			return h.svc.UpdateRack(r.Context(), id, v, p, meta(r))
		},
		archive: func(r *http.Request, id uuid.UUID, v int) (service.Rack, error) {
			return h.svc.ArchiveRack(r.Context(), id, v, meta(r))
		},
		version: func(k service.Rack) int { return k.Version }}
}

func (h *Handler) listRacks(w http.ResponseWriter, r *http.Request) {
	list(w, r, h, "list racks", func(f service.Filter) ([]service.Rack, int64, error) { return h.svc.ListRacks(r.Context(), f) })
}
func (h *Handler) getRack(w http.ResponseWriter, r *http.Request)     { h.racks().getH(w, r) }
func (h *Handler) updateRack(w http.ResponseWriter, r *http.Request)  { h.racks().updateH(w, r) }
func (h *Handler) archiveRack(w http.ResponseWriter, r *http.Request) { h.racks().archiveH(w, r) }

func (h *Handler) createRack(w http.ResponseWriter, r *http.Request) {
	var in service.RackCreate
	if !decode(w, r, &in) {
		return
	}
	k, err := h.svc.CreateRack(r.Context(), in, meta(r))
	if err != nil {
		h.fail(w, r, "create rack", err)
		return
	}
	one(w, http.StatusCreated, k.Version, k)
}
