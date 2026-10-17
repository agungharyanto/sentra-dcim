package service

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidationError names an invalid field (422 validation_failed).
type ValidationError struct{ Field, Reason string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

func invalid(field, reason string) error { return &ValidationError{Field: field, Reason: reason} }

// Same patterns as the CHECK constraints of migration 000011 (the database refuses what slips through).
var (
	siteCodeRE     = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,15}$`)
	buildingCodeRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,15}$`)
	rackNameRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._/-]{0,63}$`)
	assetTagRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$`)
)

// NormalizeRackName trims surrounding spaces and validates a custom rack name (D-083): 1–64 characters, letters
// A–Z/a–z, digits, space and . _ - /, starting with a letter or digit. Uniqueness (per site, case-insensitive,
// non-archived racks) is checked by the database.
func NormalizeRackName(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", invalid("name", "enter a rack name")
	case utf8.RuneCountInString(s) > 64:
		return "", invalid("name", "at most 64 characters")
	case !rackNameRE.MatchString(s):
		return "", invalid("name", "use letters, digits, space and . _ - / and start with a letter or digit")
	}
	return s, nil
}

// normalizeName trims and validates a display name of a site, building, floor or room (1–100 characters, no
// control characters).
func normalizeName(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", invalid(field, "required")
	}
	if utf8.RuneCountInString(s) > 100 {
		return "", invalid(field, "at most 100 characters")
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 || !utf8.ValidString(s) {
		return "", invalid(field, "control characters are not allowed")
	}
	return s, nil
}

// NormalizeSiteCode upper-cases and validates a site code (1–16 characters A–Z, 0–9, _ and -).
func NormalizeSiteCode(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !siteCodeRE.MatchString(s) {
		return "", invalid("code", "1–16 characters A–Z, 0–9, _ and -, starting with a letter or digit")
	}
	return s, nil
}

func normalizeBuildingCode(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s != "" && !buildingCodeRE.MatchString(s) {
		return "", invalid("code", "up to 16 characters A–Z, 0–9, _ and -")
	}
	return s, nil
}

func normalizeText(field, s string, max int) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		return "", invalid(field, "too long")
	}
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return "", invalid(field, "invalid characters")
	}
	return s, nil
}

func oneOf(field, v string, allowed []string) error {
	if !slices.Contains(allowed, v) {
		return invalid(field, "must be one of: "+strings.Join(allowed, ", "))
	}
	return nil
}

func between(field string, v, lo, hi int) error {
	if v < lo || v > hi {
		return invalid(field, "out of range")
	}
	return nil
}

// RackInput is a complete, normalised rack (create, or current values merged with a PATCH).
type RackInput struct {
	Name         string
	AssetTag     *string
	RowLabel     string
	RowPosition  *int
	UHeight      int
	StartingUnit int
	DescUnits    bool
	WidthIn      int
	DepthMM      *int
	MaxWeightKg  *float64
	MaxPowerKW   *float64
	Status       string
	Notes        string
}

// Validate normalises in place and checks every field against the rules of migration 000011.
func (in *RackInput) Validate() error {
	var err error
	if in.Name, err = NormalizeRackName(in.Name); err != nil {
		return err
	}
	if in.AssetTag != nil {
		t := strings.TrimSpace(*in.AssetTag)
		if t == "" {
			in.AssetTag = nil
		} else if !assetTagRE.MatchString(t) {
			return invalid("asset_tag", "1–64 characters A–Z, 0–9, . _ - /")
		} else {
			in.AssetTag = &t
		}
	}
	if in.RowLabel, err = normalizeText("row_label", in.RowLabel, 16); err != nil {
		return err
	}
	if in.RowPosition != nil {
		if err := between("row_position", *in.RowPosition, 1, 999); err != nil {
			return err
		}
	}
	if err := between("u_height", in.UHeight, 1, 60); err != nil {
		return err
	}
	if err := between("starting_unit", in.StartingUnit, 1, 100); err != nil {
		return err
	}
	if !slices.Contains(RackWidths, in.WidthIn) {
		return invalid("width_in", "must be 19 or 23")
	}
	if in.DepthMM != nil {
		if err := between("depth_mm", *in.DepthMM, 100, 2000); err != nil {
			return err
		}
	}
	if in.MaxWeightKg != nil && (*in.MaxWeightKg <= 0 || *in.MaxWeightKg > 999999.99) {
		return invalid("max_weight_kg", "out of range")
	}
	if in.MaxPowerKW != nil && (*in.MaxPowerKW <= 0 || *in.MaxPowerKW > 9999.99) {
		return invalid("max_power_kw", "out of range")
	}
	if err := oneOf("status", in.Status, RackStatuses); err != nil {
		return err
	}
	in.Notes, err = normalizeText("notes", in.Notes, 2000)
	return err
}
