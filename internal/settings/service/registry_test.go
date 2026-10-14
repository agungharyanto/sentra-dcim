package service

import (
	"encoding/json"
	"testing"
)

func reg(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry(Defaults{Language: "id", Timezone: "Asia/Jakarta"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRegistryDefaultsAndGroups(t *testing.T) {
	r := reg(t)
	want := map[string]any{KeyLanguage: "id", KeyTimezone: "Asia/Jakarta", KeyTimeFormat: "24h", KeyDateFormat: "DD/MM/YYYY"}
	for k, v := range want {
		d, ok := r.Def(k)
		if !ok || d.Default != v || !d.UserOverridable {
			t.Errorf("%s: %+v", k, d)
		}
	}
	if len(r.Group("regional")) != 4 || r.Group("nope") != nil || len(r.UserOverridable()) != 4 {
		t.Fatal("groups")
	}
	if _, err := NewRegistry(Defaults{Language: "fr", Timezone: "UTC"}); err == nil {
		t.Fatal("unsupported default language accepted")
	}
	if _, err := NewRegistry(Defaults{Language: "id", Timezone: "Mars/Base"}); err == nil {
		t.Fatal("unknown default zone accepted")
	}
}

func TestDecodeValidation(t *testing.T) {
	r := reg(t)
	cases := []struct {
		key, raw string
		ok       bool
	}{
		{KeyTimezone, `"Asia/Makassar"`, true},
		{KeyTimezone, `"UTC"`, true},
		{KeyTimezone, `"America/Argentina/Buenos_Aires"`, true},
		{KeyTimezone, `"Mars/Base"`, false},
		{KeyTimezone, `"Local"`, false},
		{KeyTimezone, `""`, false},
		{KeyTimezone, `"../../etc/passwd"`, false},
		{KeyTimezone, `"Asia/ Jakarta"`, false},
		{KeyTimezone, `7`, false},
		{KeyLanguage, `"en"`, true},
		{KeyLanguage, `"fr"`, false},
		{KeyTimeFormat, `"12h"`, true},
		{KeyTimeFormat, `"12"`, false},
		{KeyDateFormat, `"DD MMM YYYY"`, true},
		{KeyDateFormat, `"MM/DD/YYYY"`, false},
		{KeyDateFormat, `null`, false},
	}
	for _, c := range cases {
		d, _ := r.Def(c.key)
		_, err := d.decode(json.RawMessage(c.raw))
		if (err == nil) != c.ok {
			t.Errorf("%s %s: err=%v, want ok=%t", c.key, c.raw, err, c.ok)
		}
	}
}

func TestDecodeIntBool(t *testing.T) {
	lo, hi := int64(5), int64(10)
	d := Def{Key: "x.n", Type: TypeInt, Min: &lo, Max: &hi}
	for raw, ok := range map[string]bool{`5`: true, `10`: true, `4`: false, `11`: false, `"7"`: false, `7.5`: false} {
		if _, err := d.decode(json.RawMessage(raw)); (err == nil) != ok {
			t.Errorf("int %s: %v", raw, err)
		}
	}
	b := Def{Key: "x.b", Type: TypeBool}
	if _, err := b.decode(json.RawMessage(`true`)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.decode(json.RawMessage(`"true"`)); err == nil {
		t.Fatal("string accepted as bool")
	}
}
