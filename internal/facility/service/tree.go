package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/agungharyanto/sentra-dcim/internal/facility/repo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// TreeRack is a rack leaf of the facility tree.
type TreeRack struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
}

// TreeRoom is a room with its racks and their count per status.
type TreeRoom struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	RoomType  string         `json:"room_type"`
	RackCount int            `json:"rack_count"`
	ByStatus  map[string]int `json:"racks_by_status"`
	Racks     []TreeRack     `json:"racks"`
}

// TreeFloor is a floor with its rooms.
type TreeFloor struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Level     int        `json:"level"`
	RackCount int        `json:"rack_count"`
	Rooms     []TreeRoom `json:"rooms"`
}

// TreeBuilding is a building with its floors.
type TreeBuilding struct {
	ID        uuid.UUID   `json:"id"`
	Code      string      `json:"code"`
	Name      string      `json:"name"`
	RackCount int         `json:"rack_count"`
	Floors    []TreeFloor `json:"floors"`
}

// TreeSite is a site with its buildings.
type TreeSite struct {
	ID        uuid.UUID      `json:"id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	RackCount int            `json:"rack_count"`
	Buildings []TreeBuilding `json:"buildings"`
}

// Tree returns the non-archived hierarchy in the caller's facility read scope. Racks are listed only when the caller
// may read racks of that site (rack.read); counts follow the same rule.
func (s *Service) Tree(ctx context.Context) ([]TreeSite, error) {
	sc := scopeOf(ctx, rbac.FacilityRead)
	rsc := scopeOf(ctx, rbac.RackRead)
	sites, err := s.q.TreeSites(ctx, repo.TreeSitesParams{AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return nil, err
	}
	buildings, err := s.q.TreeBuildings(ctx, repo.TreeBuildingsParams{AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return nil, err
	}
	floors, err := s.q.TreeFloors(ctx, repo.TreeFloorsParams{AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return nil, err
	}
	rooms, err := s.q.TreeRooms(ctx, repo.TreeRoomsParams{AllSites: sc.All, SiteIds: sc.IDs()})
	if err != nil {
		return nil, err
	}
	racks, err := s.q.TreeRacks(ctx, repo.TreeRacksParams{AllSites: rsc.All, SiteIds: rsc.IDs()})
	if err != nil {
		return nil, err
	}

	racksOf := map[uuid.UUID][]TreeRack{}
	for _, k := range racks {
		racksOf[k.RoomID] = append(racksOf[k.RoomID], TreeRack{ID: k.ID, Name: k.Name, Status: k.Status})
	}
	roomsOf := map[uuid.UUID][]TreeRoom{}
	for _, r := range rooms {
		rr := racksOf[r.ID]
		if rr == nil {
			rr = []TreeRack{}
		}
		by := map[string]int{}
		for _, k := range rr {
			by[k.Status]++
		}
		roomsOf[r.FloorID] = append(roomsOf[r.FloorID], TreeRoom{ID: r.ID, Name: r.Name, RoomType: r.RoomType, RackCount: len(rr),
			ByStatus: by, Racks: rr})
	}
	floorsOf := map[uuid.UUID][]TreeFloor{}
	for _, f := range floors {
		rs := roomsOf[f.ID]
		if rs == nil {
			rs = []TreeRoom{}
		}
		n := 0
		for _, r := range rs {
			n += r.RackCount
		}
		floorsOf[f.BuildingID] = append(floorsOf[f.BuildingID], TreeFloor{ID: f.ID, Name: f.Name, Level: int(f.Level), RackCount: n, Rooms: rs})
	}
	buildingsOf := map[uuid.UUID][]TreeBuilding{}
	for _, b := range buildings {
		fs := floorsOf[b.ID]
		if fs == nil {
			fs = []TreeFloor{}
		}
		n := 0
		for _, f := range fs {
			n += f.RackCount
		}
		buildingsOf[b.SiteID] = append(buildingsOf[b.SiteID], TreeBuilding{ID: b.ID, Code: b.Code, Name: b.Name, RackCount: n, Floors: fs})
	}
	out := make([]TreeSite, 0, len(sites))
	for _, st := range sites {
		bs := buildingsOf[st.ID]
		if bs == nil {
			bs = []TreeBuilding{}
		}
		n := 0
		for _, b := range bs {
			n += b.RackCount
		}
		out = append(out, TreeSite{ID: st.ID, Code: st.Code, Name: st.Name, Status: st.Status, RackCount: n, Buildings: bs})
	}
	return out, nil
}
