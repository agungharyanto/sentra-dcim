package service_test

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	apispec "github.com/agungharyanto/sentra-dcim/api"
	helpdocs "github.com/agungharyanto/sentra-dcim/docs/help"
	"github.com/agungharyanto/sentra-dcim/internal/help/service"
	uiservice "github.com/agungharyanto/sentra-dcim/internal/ui/service"
)

var locales = []string{"id", "en"}

func registryMenus() []service.Menu {
	_, menus := uiservice.Registry()
	out := []service.Menu{}
	for _, m := range menus {
		out = append(out, service.Menu{Key: m.Key, Path: m.Path, Available: m.Status == uiservice.StatusAvailable})
	}
	return out
}

// The real, embedded Help Center must always validate: this is the CI gate (CLAUDE.md rule 7).
func TestEmbeddedHelpIsValid(t *testing.T) {
	cat, err := service.Load(helpdocs.FS, locales, registryMenus(), apispec.OpenAPI)
	if err != nil {
		t.Fatalf("docs/help is invalid:\n%v", err)
	}
	idx, _ := cat.Index("id")
	enIdx, _ := cat.Index("en")
	if len(idx) == 0 || len(idx) != len(enIdx) {
		t.Fatalf("index id=%d en=%d", len(idx), len(enIdx))
	}
	var spec map[string]any
	if err := json.Unmarshal(cat.OpenAPI(), &spec); err != nil || spec["openapi"] == nil || spec["paths"] == nil {
		t.Fatalf("openapi json: %v", err)
	}
}

// --- synthetic catalogs for negative tests ---

const fm = "---\ntitle: %s\naudience: user\n%sorder: 10\n---\n\n# T\n\n## Section\n\nBody.\n"

func page(title, menu string) string {
	m := ""
	if menu != "" {
		m = "menu: " + menu + "\n"
	}
	return strings.Replace(strings.Replace(fm, "%s", title, 1), "%s", m, 1)
}

func validFS() fstest.MapFS {
	return fstest.MapFS{
		"id/dashboard/index.md": {Data: []byte(page("Dasbor", "dashboard"))},
		"en/dashboard/index.md": {Data: []byte(page("Dashboard", "dashboard"))},
		"id/help/index.md":      {Data: []byte(page("Bantuan", "help"))},
		"en/help/index.md":      {Data: []byte(page("Help", "help"))},
	}
}

var testMenus = []service.Menu{
	{Key: "dashboard", Path: "/dashboard", Available: true},
	{Key: "help", Path: "/help", Available: true},
	{Key: "alarms", Path: "/alarms"},
}

var spec = []byte("openapi: 3.1.0\npaths: {}\n")

func load(fs fstest.MapFS) error {
	_, err := service.Load(fs, locales, testMenus, spec)
	return err
}

const okSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`

func TestSyntheticValid(t *testing.T) {
	if err := load(validFS()); err != nil {
		t.Fatal(err)
	}
}

func TestImagesServed(t *testing.T) {
	f := validFS()
	f["assets/step-one.svg"] = &fstest.MapFile{Data: []byte(okSVG)}
	f["id/dashboard/index.md"] = &fstest.MapFile{Data: []byte(page("Dasbor", "dashboard") + "\n![Langkah pertama di HP](/api/v1/help/assets/step-one.svg)\n")}
	cat, err := service.Load(f, locales, testMenus, spec)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := cat.Asset("step-one.svg")
	if !ok || a.ContentType != "image/svg+xml" || string(a.Data) != okSVG {
		t.Fatalf("asset %+v %v", a, ok)
	}
	if _, ok := cat.Asset("../id/dashboard/index.md"); ok {
		t.Fatal("path traversal served")
	}
}

func TestValidationRejects(t *testing.T) {
	body := func(extra string) []byte { return []byte(page("Dasbor", "dashboard") + "\n" + extra + "\n") }
	cases := []struct {
		name, want string
		mutate     func(fstest.MapFS)
	}{
		{"broken front matter", "front matter", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: []byte("---\ntitle: [unclosed\n---\nx\n")}
		}},
		{"missing front matter", "missing front matter", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: []byte("# no front matter\n")}
		}},
		{"unknown field", "field secret not found", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: []byte("---\ntitle: X\naudience: user\nmenu: dashboard\norder: 1\nsecret: y\n---\nx\n")}
		}},
		{"unknown audience", "audience", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: []byte(strings.Replace(page("X", "dashboard"), "audience: user", "audience: developer", 1))}
		}},
		{"missing title", "title is required", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: []byte(strings.Replace(page("X", "dashboard"), "title: X", "title: \"\"", 1))}
		}},
		{"missing EN slug", "missing en counterpart", func(f fstest.MapFS) {
			f["id/extra/index.md"] = &fstest.MapFile{Data: []byte(page("Ekstra", ""))}
		}},
		{"missing ID slug", "no id counterpart", func(f fstest.MapFS) {
			f["en/extra/index.md"] = &fstest.MapFile{Data: []byte(page("Extra", ""))}
		}},
		{"unknown menu key", "not in the menu registry", func(f fstest.MapFS) {
			f["id/x/index.md"] = &fstest.MapFile{Data: []byte(page("X", "nope"))}
			f["en/x/index.md"] = &fstest.MapFile{Data: []byte(page("X", "nope"))}
		}},
		{"available menu without page", `available menu "dashboard" has no help page`, func(f fstest.MapFS) {
			delete(f, "id/dashboard/index.md")
			delete(f, "en/dashboard/index.md")
		}},
		{"menu differs across locales", "differ", func(f fstest.MapFS) {
			f["en/help/index.md"] = &fstest.MapFile{Data: []byte(page("Help", "dashboard"))}
		}},
		{"invalid slug", "invalid slug", func(f fstest.MapFS) {
			f["id/Bad_Slug/index.md"] = &fstest.MapFile{Data: []byte(page("X", ""))}
		}},
		{"raw HTML script", "raw HTML", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("<script>alert(1)</script>")}
		}},
		{"raw HTML img", "raw HTML", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body(`<img src=x onerror="alert(1)">`)}
		}},
		{"image outside the help assets", "must be /api/v1/help/assets/", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("![Diagram langkah satu](https://evil.example/x.png)")}
		}},
		{"unknown image asset", "not in docs/help/assets", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("![Diagram langkah satu](/api/v1/help/assets/nope.png)")}
		}},
		{"image without alt text", "alt text", func(f fstest.MapFS) {
			f["assets/step.svg"] = &fstest.MapFile{Data: []byte(okSVG)}
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("![](/api/v1/help/assets/step.svg)")}
		}},
		{"SVG with script", "SVG with scripts", func(f fstest.MapFS) {
			f["assets/bad.svg"] = &fstest.MapFile{Data: []byte(`<svg><script>alert(1)</script></svg>`)}
		}},
		{"SVG with event handler", "SVG with scripts", func(f fstest.MapFS) {
			f["assets/bad.svg"] = &fstest.MapFile{Data: []byte(`<svg onload="x()"></svg>`)}
		}},
		{"SVG with external reference", "SVG with scripts", func(f fstest.MapFS) {
			f["assets/bad.svg"] = &fstest.MapFile{Data: []byte(`<svg><image href="https://evil.example/t.png"/></svg>`)}
		}},
		{"badly named asset", "kebab-case", func(f fstest.MapFS) { f["assets/Step 1.PNG"] = &fstest.MapFile{Data: []byte("x")} }},
		{"fake PNG", "not a PNG", func(f fstest.MapFS) { f["assets/step.png"] = &fstest.MapFile{Data: []byte("<html>")} }},
		{"IPv4", "IPv4", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("Server 10.0.0.12 down")}
		}},
		{"IPv4 in code too", "IPv4", func(f fstest.MapFS) { f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("`curl 127.0.0.1`")} }},
		{"/opt path", "server path", func(f fstest.MapFS) { f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("cd /opt/sentra-dcim")} }},
		{"/etc path", "server path", func(f fstest.MapFS) { f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("edit /etc/ssh/x")} }},
		{".env file", ".env", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("Edit `.env` lalu restart")}
		}},
		{"OS user", "OS user", func(f fstest.MapFS) { f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("login sebagai ubuntu")} }},
		{"sudo", "shell/admin command", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("jalankan sudo make up")}
		}},
		{"broken relative link", "missing page", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("[x](../nope/index.md)")}
		}},
		{"link outside tree", "missing page", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("[x](../../../ops/runbooks/x.md)")}
		}},
		{"unknown app route", "not a known app route", func(f fstest.MapFS) { f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("[x](/admin)")} }},
		{"javascript link", "scheme not allowed", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("[x](javascript:alert(1))")}
		}},
		{"plain http link", "scheme not allowed", func(f fstest.MapFS) {
			f["id/dashboard/index.md"] = &fstest.MapFile{Data: body("[x](http://example.org)")}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validFS()
			tc.mutate(f)
			err := load(f)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidationAllowsCodeAndSafeLinks(t *testing.T) {
	extra := strings.Join([]string{
		"Placeholder `<user>` dan `<hash>` di inline code.",
		"```",
		"<script>contoh di blok kode</script>",
		"```",
		"Autolink <https://example.org> dan [eksternal](https://example.org).",
		"[Bantuan](../help/index.md) · [Dasbor](/dashboard) · [API](/help/api/reference) · [atas](#section)",
	}, "\n")
	f := validFS()
	f["id/dashboard/index.md"] = &fstest.MapFile{Data: []byte(page("Dasbor", "dashboard") + "\n" + extra + "\n")}
	if err := load(f); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidOpenAPI(t *testing.T) {
	for _, bad := range []string{"paths: [unclosed", "- just\n- a list\n", "title: no openapi or paths\n"} {
		if _, err := service.Load(validFS(), locales, testMenus, []byte(bad)); err == nil || !strings.Contains(err.Error(), "openapi") {
			t.Fatalf("%q: want openapi error, got %v", bad, err)
		}
	}
}

func TestIndexAndPage(t *testing.T) {
	f := validFS()
	f["id/dashboard/extra.md"] = &fstest.MapFile{Data: []byte(strings.Replace(page("Ekstra", ""), "order: 10", "order: 5", 1) + "[kembali](index.md)\n")}
	f["en/dashboard/extra.md"] = &fstest.MapFile{Data: []byte(strings.Replace(page("Extra", ""), "order: 10", "order: 5", 1))}
	cat, err := service.Load(f, locales, testMenus, spec)
	if err != nil {
		t.Fatal(err)
	}
	idx, ok := cat.Index("id")
	if !ok || len(idx) != 3 || idx[0].Slug != "dashboard/extra" {
		t.Fatalf("index = %+v", idx)
	}
	if len(idx[1].Headings) != 1 || idx[1].Headings[0].Text != "Section" || idx[1].Headings[0].Level != 2 {
		t.Fatalf("headings = %+v", idx[1].Headings)
	}
	extra, _ := cat.Page("id", "dashboard/extra")
	if !strings.Contains(extra.Markdown, "[kembali](/help/dashboard)") {
		t.Fatalf("relative link not rewritten to an app route: %q", extra.Markdown)
	}
	p, ok := cat.Page("en", "dashboard")
	if !ok || p.Title != "Dashboard" || p.Menu != "dashboard" || strings.Contains(p.Markdown, "audience:") {
		t.Fatalf("page = %+v", p)
	}
	for _, slug := range []string{"../id/dashboard", "dashboard/../help", "", "dashboard/index", "nope"} {
		if _, ok := cat.Page("id", slug); ok {
			t.Fatalf("slug %q must not resolve", slug)
		}
	}
	if _, ok := cat.Index("fr"); ok {
		t.Fatal("unknown locale must not resolve")
	}
}
