package service

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestNormalizeRackName(t *testing.T) {
	ok := map[string]string{"R-A01": "R-A01", "  rack 1 ": "rack 1", "A/B.c_d-e": "A/B.c_d-e", "9": "9",
		strings.Repeat("x", 64): strings.Repeat("x", 64)}
	for in, want := range ok {
		got, err := NormalizeRackName(in)
		if err != nil || got != want {
			t.Errorf("%q → %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "   ", "-R1", ".R", " /x", "R@1", "R#", "Rák", "R\t1", strings.Repeat("x", 65)} {
		if _, err := NormalizeRackName(in); err == nil {
			t.Errorf("%q accepted", in)
		} else {
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != "name" {
				t.Errorf("%q: %v", in, err)
			}
		}
	}
}

// The rack name rule is enforced twice (app + CHECK in migration 000011): keep the patterns identical.
func TestRackNamePatternMatchesMigration(t *testing.T) {
	b, err := os.ReadFile("../../../migrations/000011_facility.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "'"+rackNameRE.String()+"'") || !strings.Contains(string(b), "'"+siteCodeRE.String()+"'") ||
		!strings.Contains(string(b), "'"+assetTagRE.String()+"'") {
		t.Error("patterns differ between validate.go and migration 000011")
	}
}

// D-084: every query of the facility repository filters by the caller's site scope — there is no unscoped variant.
func TestEveryQueryIsScoped(t *testing.T) {
	b, err := os.ReadFile("../repo/queries.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := regexp.MustCompile(`(?m)^-- name: `).Split(string(b), -1)[1:]
	if len(parts) < 30 {
		t.Fatalf("only %d queries", len(parts))
	}
	for _, q := range parts {
		name := strings.Fields(q)[0]
		if !strings.Contains(q, "@all_sites::bool") {
			t.Errorf("query %s has no scope predicate", name)
		}
	}
}

func TestSiteCodeAndRackInput(t *testing.T) {
	if c, err := NormalizeSiteCode(" dps-1 "); err != nil || c != "DPS-1" {
		t.Errorf("site code: %q %v", c, err)
	}
	for _, bad := range []string{"", "_X", "TOO-LONG-CODE-12345", "A B"} {
		if _, err := NormalizeSiteCode(bad); err == nil {
			t.Errorf("site code %q accepted", bad)
		}
	}
	tag := "  "
	in := RackInput{Name: "R1", AssetTag: &tag, UHeight: 42, StartingUnit: 1, WidthIn: 19, Status: "planned"}
	if err := in.Validate(); err != nil || in.AssetTag != nil {
		t.Errorf("blank asset tag must become null: %v %v", err, in.AssetTag)
	}
	for field, mut := range map[string]func(*RackInput){
		"u_height": func(r *RackInput) { r.UHeight = 0 }, "width_in": func(r *RackInput) { r.WidthIn = 21 },
		"status": func(r *RackInput) { r.Status = "broken" }, "starting_unit": func(r *RackInput) { r.StartingUnit = 0 },
		"max_power_kw": func(r *RackInput) { v := -1.0; r.MaxPowerKW = &v },
		"depth_mm":     func(r *RackInput) { v := 50; r.DepthMM = &v },
	} {
		r := RackInput{Name: "R1", UHeight: 42, StartingUnit: 1, WidthIn: 19, Status: "planned"}
		mut(&r)
		var ve *ValidationError
		if err := r.Validate(); !errors.As(err, &ve) || ve.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
}

func TestOptAndPatch(t *testing.T) {
	var p RackPatch
	if err := json.Unmarshal([]byte(`{"name":" R2 ","asset_tag":null,"u_height":48}`), &p); err != nil {
		t.Fatal(err)
	}
	tag := "T1"
	cur := Rack{Name: "R1", AssetTag: &tag, UHeight: 42, StartingUnit: 1, WidthIn: 19, Status: "active"}
	in, room, err := p.apply(cur)
	if err != nil || in.Name != "R2" || in.AssetTag != nil || in.UHeight != 48 || in.Status != "active" || room != cur.RoomID {
		t.Errorf("apply: %+v %v", in, err)
	}
	if err := json.Unmarshal([]byte(`{"name":null}`), &p); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.apply(cur); err == nil {
		t.Error("null name accepted")
	}
}

func TestChanges(t *testing.T) {
	a := Site{Code: "A", Name: "One", Status: "active"}
	b := a
	if changes(a, b, siteFields) != nil {
		t.Error("no change must give nil")
	}
	b.Name = "Two"
	md := changes(a, b, siteFields)
	if md["before"].(map[string]any)["name"] != "One" || md["after"].(map[string]any)["name"] != "Two" || len(md["after"].(map[string]any)) != 1 {
		t.Errorf("changes = %v", md)
	}
}
