package service

import (
	"encoding/json"
	"reflect"
)

// Audited fields per level: what an operator changes (not derived names, counters or timestamps).
var (
	siteFields     = []string{"code", "name", "address", "status"}
	buildingFields = []string{"site_id", "code", "name", "notes"}
	floorFields    = []string{"building_id", "site_id", "name", "level", "notes"}
	roomFields     = []string{"floor_id", "site_id", "name", "room_type", "notes"}
	rackFields     = []string{"room_id", "site_id", "name", "asset_tag", "row_label", "row_position", "u_height",
		"starting_unit", "desc_units", "width_in", "depth_mm", "max_weight_kg", "max_power_kw", "status", "notes"}
)

func asMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
}

// snapshot returns the audited fields of v (JSON names).
func snapshot(v any, fields []string) map[string]any {
	m := asMap(v)
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		out[f] = m[f]
	}
	return out
}

// changes returns {"before": {...}, "after": {...}} with the audited fields that differ, or nil when nothing did.
func changes(before, after any, fields []string) map[string]any {
	b, a := asMap(before), asMap(after)
	bo, ao := map[string]any{}, map[string]any{}
	for _, f := range fields {
		if !reflect.DeepEqual(b[f], a[f]) {
			bo[f], ao[f] = b[f], a[f]
		}
	}
	if len(ao) == 0 {
		return nil
	}
	return map[string]any{"before": bo, "after": ao}
}
