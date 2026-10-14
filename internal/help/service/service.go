// Package service loads, validates and serves the Help Center: Markdown pages embedded from docs/help/{id,en}
// and the OpenAPI specification. Validation failures are reported together; `go test` is the blocking gate, and
// at runtime an invalid catalog only disables the help module (see cmd/sentra).
package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Audiences allowed in front matter. From v0.3 pages are filtered by the caller's role (ISS-032).
const (
	AudienceUser  = "user"
	AudienceAdmin = "admin"
)

// APIReferenceRoute is the UI route of the read-only API viewer; help pages may link to it.
const APIReferenceRoute = "/help/api/reference"

// Heading is an H2/H3 of a page, used for client-side search.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

// Meta is the page metadata (front matter + slug + headings), without the body.
type Meta struct {
	Slug     string    `json:"slug"`
	Title    string    `json:"title"`
	Audience string    `json:"audience"`
	Menu     string    `json:"menu,omitempty"`
	Order    int       `json:"order"`
	Headings []Heading `json:"headings"`
}

// Page is a full help page: metadata plus Markdown body (front matter removed).
type Page struct {
	Meta
	Locale   string `json:"locale"`
	Markdown string `json:"markdown"`
	dir      string // directory of the source file, base for relative links
}

// Menu is the part of the UI menu registry the validator needs.
type Menu struct {
	Key, Path string
	Available bool
}

// Catalog is a validated, immutable set of help pages per locale plus the OpenAPI document as JSON.
type Catalog struct {
	pages   map[string]map[string]Page // locale → slug → page
	openAPI json.RawMessage
	assets  map[string]Asset // file name → image
}

// Asset is an embedded help image.
type Asset struct {
	ContentType string
	Data        []byte
}

// AssetPrefix is the URL path of help images in Markdown: ![alt](/api/v1/help/assets/<file>).
const AssetPrefix = "/api/v1/help/assets/"

var (
	assetNameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*\.(png|svg)$`)
	imageRE     = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	// SVG help images are our own drawings: nothing active or external.
	svgUnsafeRE = regexp.MustCompile(`(?i)<script|<foreignobject|\son[a-z]+\s*=|javascript:|(?:href|src)\s*=\s*"(?:https?:)?//`)
)

// Asset returns an embedded help image by file name.
func (c *Catalog) Asset(name string) (Asset, bool) {
	a, ok := c.assets[name]
	return a, ok
}

type frontMatter struct {
	Title    string `yaml:"title"`
	Audience string `yaml:"audience"`
	Menu     string `yaml:"menu"`
	Order    *int   `yaml:"order"`
}

var (
	slugRE    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*(?:/[a-z0-9]+(?:-[a-z0-9]+)*)*$`)
	fenceRE   = regexp.MustCompile("(?ms)^\\s*(```|~~~).*?^\\s*(```|~~~)\\s*$")
	inlineRE  = regexp.MustCompile("`[^`\n]*`")
	htmlRE    = regexp.MustCompile(`<[A-Za-z!/?][^>]*>`)
	autoRE    = regexp.MustCompile(`<(?:https?://|mailto:)[^>\s]+>`)
	linkRE    = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	headingRE = regexp.MustCompile(`(?m)^(#{2,3})\s+(.+?)\s*#*\s*$`)
)

// Infrastructure details never belong in user/admin help (they live in docs/ops/runbooks).
var infraPatterns = []struct {
	re   *regexp.Regexp
	what string
}{
	{regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), "IPv4 address"},
	{regexp.MustCompile(`/(?:opt|etc|home|root|var)/`), "server path"},
	{regexp.MustCompile(`(?:^|[^\w])\.env\b`), ".env file"},
	{regexp.MustCompile(`(?i)\b(?:ubuntu|root)\b`), "OS user name"},
	{regexp.MustCompile(`(?i)\b(?:sudo|ssh)\b`), "shell/admin command"},
}

// Load reads every <locale>/**/*.md from fsys, validates the whole set against the menu registry and returns the
// catalog. All problems are returned together (errors.Join); a non-nil error means the catalog must not be served.
func Load(fsys fs.FS, locales []string, menus []Menu, openAPIYAML []byte) (*Catalog, error) {
	var errs []error
	c := &Catalog{pages: map[string]map[string]Page{}}
	assets, aerrs := loadAssets(fsys)
	errs = append(errs, aerrs...)
	c.assets = assets

	for _, loc := range locales {
		pages, perrs := loadLocale(fsys, loc)
		errs = append(errs, perrs...)
		c.pages[loc] = pages
	}

	routes := map[string]bool{APIReferenceRoute: true}
	menuKeys := map[string]bool{}
	for _, m := range menus {
		routes[m.Path] = true
		menuKeys[m.Key] = true
	}

	ref := locales[0]
	for _, loc := range locales {
		for slug, p := range c.pages[loc] {
			where := loc + "/" + slug
			if p.Menu != "" && !menuKeys[p.Menu] {
				errs = append(errs, fmt.Errorf("%s: menu %q is not in the menu registry", where, p.Menu))
			}
			errs = append(errs, checkLinks(where, p.Markdown, p.dir, c.pages[loc], routes)...)
			errs = append(errs, checkImages(where, p.Markdown, c.assets)...)
			if loc == ref {
				continue
			}
			rp, ok := c.pages[ref][slug]
			if !ok {
				errs = append(errs, fmt.Errorf("%s: no %s counterpart (slugs must be identical across locales)", where, ref))
				continue
			}
			if rp.Audience != p.Audience || rp.Menu != p.Menu || rp.Order != p.Order {
				errs = append(errs, fmt.Errorf("%s: audience/menu/order differ from %s/%s", where, ref, slug))
			}
		}
		if loc != ref {
			for slug := range c.pages[ref] {
				if _, ok := c.pages[loc][slug]; !ok {
					errs = append(errs, fmt.Errorf("%s/%s: missing %s counterpart (slugs must be identical across locales)", ref, slug, loc))
				}
			}
		}
	}

	// Every available menu needs a help page (CLAUDE.md rule 7) in every locale.
	for _, m := range menus {
		if !m.Available {
			continue
		}
		for _, loc := range locales {
			found := false
			for _, p := range c.pages[loc] {
				found = found || p.Menu == m.Key
			}
			if !found {
				errs = append(errs, fmt.Errorf("%s: available menu %q has no help page (front matter menu: %s)", loc, m.Key, m.Key))
			}
		}
	}

	spec, err := yamlToJSON(openAPIYAML)
	if err != nil {
		errs = append(errs, fmt.Errorf("openapi: %w", err))
	}
	c.openAPI = spec

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	// Serve relative .md links as app routes (/help/<slug>): only the backend knows each page's source directory.
	for _, pages := range c.pages {
		for slug, p := range pages {
			p.Markdown = rewriteLinks(p.Markdown, p.dir)
			pages[slug] = p
		}
	}
	return c, nil
}

// rewriteLinks turns validated relative .md links into /help/<slug>[#anchor].
func rewriteLinks(body, dir string) string {
	return linkRE.ReplaceAllStringFunc(body, func(m string) string {
		sub := linkRE.FindStringSubmatch(m)
		target := sub[1]
		if strings.Contains(target, ":") || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "#") {
			return m
		}
		file, anchor, _ := strings.Cut(target, "#")
		slug := SlugFromLink(dir, file)
		if slug == "" {
			return m
		}
		repl := "/help/" + slug
		if anchor != "" {
			repl += "#" + anchor
		}
		return strings.Replace(m, "("+target, "("+repl, 1)
	})
}

func loadLocale(fsys fs.FS, loc string) (map[string]Page, []error) {
	pages := map[string]Page{}
	var errs []error
	err := fs.WalkDir(fsys, loc, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(p, loc+"/"), ".md")
		slug := strings.TrimSuffix(rel, "/index")
		where := loc + "/" + rel + ".md"
		if !slugRE.MatchString(slug) || slug == "index" {
			errs = append(errs, fmt.Errorf("%s: invalid slug %q (use <slug>/index.md, lowercase a-z 0-9 -)", where, slug))
			return nil
		}
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		page, perrs := parsePage(where, loc, slug, path.Dir(rel), raw)
		errs = append(errs, perrs...)
		if len(perrs) == 0 {
			pages[slug] = page
		}
		return nil
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", loc, err))
	}
	if len(pages) == 0 && len(errs) == 0 {
		errs = append(errs, fmt.Errorf("%s: no help pages", loc))
	}
	return pages, errs
}

func parsePage(where, loc, slug, dir string, raw []byte) (Page, []error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Page{}, []error{fmt.Errorf("%s: missing front matter (--- block at the top)", where)}
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return Page{}, []error{fmt.Errorf("%s: unterminated front matter", where)}
	}
	head, body := text[4:4+end], strings.TrimLeft(text[4+end+5:], "\n")

	var fm frontMatter
	dec := yaml.NewDecoder(strings.NewReader(head))
	dec.KnownFields(true)
	var errs []error
	if err := dec.Decode(&fm); err != nil {
		return Page{}, []error{fmt.Errorf("%s: front matter: %w", where, err)}
	}
	if strings.TrimSpace(fm.Title) == "" {
		errs = append(errs, fmt.Errorf("%s: front matter title is required", where))
	}
	if fm.Audience != AudienceUser && fm.Audience != AudienceAdmin {
		errs = append(errs, fmt.Errorf("%s: audience %q must be %q or %q", where, fm.Audience, AudienceUser, AudienceAdmin))
	}
	if fm.Order == nil {
		errs = append(errs, fmt.Errorf("%s: front matter order is required", where))
	}

	prose := inlineRE.ReplaceAllString(fenceRE.ReplaceAllString(body, ""), "")
	if m := htmlRE.FindString(autoRE.ReplaceAllString(prose, "")); m != "" {
		errs = append(errs, fmt.Errorf("%s: raw HTML %q is not allowed outside code", where, m))
	}
	for _, p := range infraPatterns {
		if m := p.re.FindString(body); m != "" {
			errs = append(errs, fmt.Errorf("%s: infrastructure detail (%s) %q belongs in docs/ops/runbooks", where, p.what, m))
		}
	}
	if len(errs) > 0 {
		return Page{}, errs
	}

	headings := []Heading{}
	for _, h := range headingRE.FindAllStringSubmatch(fenceRE.ReplaceAllString(body, ""), -1) {
		headings = append(headings, Heading{Level: len(h[1]), Text: h[2]})
	}
	order := 0
	if fm.Order != nil {
		order = *fm.Order
	}
	return Page{
		Meta:     Meta{Slug: slug, Title: fm.Title, Audience: fm.Audience, Menu: fm.Menu, Order: order, Headings: headings},
		Locale:   loc,
		Markdown: body,
		dir:      dir,
	}, nil
}

// checkLinks validates link targets: external https/mailto, anchors, app routes, or relative .md files that
// resolve to an existing page of the same locale.
func checkLinks(where, body, dir string, pages map[string]Page, routes map[string]bool) []error {
	var errs []error
	prose := inlineRE.ReplaceAllString(fenceRE.ReplaceAllString(body, ""), "")
	for _, m := range linkRE.FindAllStringSubmatch(prose, -1) {
		target := m[1]
		if strings.HasPrefix(m[0], "!") {
			continue // images: checkImages
		}
		switch {
		case strings.HasPrefix(target, "https://"), strings.HasPrefix(target, "mailto:"), strings.HasPrefix(target, "#"):
		case strings.HasPrefix(target, "/"):
			if !routes[strings.SplitN(target, "#", 2)[0]] {
				errs = append(errs, fmt.Errorf("%s: link %q is not a known app route", where, target))
			}
		case strings.Contains(target, ":"):
			errs = append(errs, fmt.Errorf("%s: link scheme not allowed in %q", where, target))
		default:
			file := strings.SplitN(target, "#", 2)[0]
			if !strings.HasSuffix(file, ".md") {
				errs = append(errs, fmt.Errorf("%s: relative link %q must point to a .md page", where, target))
				continue
			}
			if slug := SlugFromLink(dir, file); slug == "" || pages[slug].Slug == "" {
				errs = append(errs, fmt.Errorf("%s: link %q points to a missing page", where, target))
			}
		}
	}
	return errs
}

// SlugFromLink resolves a relative .md link written in the page directory dir to a slug ("" if outside the tree).
func SlugFromLink(dir, file string) string {
	p := path.Clean(path.Join(dir, file))
	if strings.HasPrefix(p, "..") || strings.HasPrefix(p, "/") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSuffix(p, ".md"), "/index")
}

func yamlToJSON(src []byte) (json.RawMessage, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	if doc["openapi"] == nil || doc["paths"] == nil {
		return nil, errors.New("document needs openapi and paths")
	}
	return json.Marshal(doc)
}

// Locales returns the locales of the catalog.
func (c *Catalog) Locales() []string {
	out := make([]string, 0, len(c.pages))
	for l := range c.pages {
		out = append(out, l)
	}
	slices.Sort(out)
	return out
}

// Index returns page metadata of a locale sorted by order, then slug. ok is false for an unknown locale.
func (c *Catalog) Index(locale string) ([]Meta, bool) {
	pages, ok := c.pages[locale]
	if !ok {
		return nil, false
	}
	out := make([]Meta, 0, len(pages))
	for _, p := range pages {
		out = append(out, p.Meta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Slug < out[j].Slug
	})
	return out, true
}

// Page returns one page. The slug is only ever used as a map key (whitelist), never as a file path.
func (c *Catalog) Page(locale, slug string) (Page, bool) {
	p, ok := c.pages[locale][slug]
	return p, ok
}

// OpenAPI returns the specification as JSON.
func (c *Catalog) OpenAPI() json.RawMessage { return c.openAPI }

// loadAssets reads assets/*.png|svg. Only well-named, safe files; anything else in the directory is an error.
func loadAssets(fsys fs.FS) (map[string]Asset, []error) {
	out := map[string]Asset{}
	entries, err := fs.ReadDir(fsys, "assets")
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, []error{fmt.Errorf("assets: %w", err)}
	}
	var errs []error
	for _, e := range entries {
		name := e.Name()
		m := assetNameRE.FindStringSubmatch(name)
		if e.IsDir() || m == nil {
			errs = append(errs, fmt.Errorf("assets/%s: only lower-case kebab-case .png/.svg files are allowed", name))
			continue
		}
		data, err := fs.ReadFile(fsys, "assets/"+name)
		if err != nil {
			errs = append(errs, fmt.Errorf("assets/%s: %w", name, err))
			continue
		}
		if len(data) > 512<<10 {
			errs = append(errs, fmt.Errorf("assets/%s: larger than 512 KiB", name))
		}
		ct := "image/png"
		if m[1] == "svg" {
			ct = "image/svg+xml"
			if svgUnsafeRE.Match(data) {
				errs = append(errs, fmt.Errorf("assets/%s: SVG with scripts, event handlers or external references", name))
			}
		} else if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			errs = append(errs, fmt.Errorf("assets/%s: not a PNG file", name))
		}
		out[name] = Asset{ContentType: ct, Data: data}
	}
	return out, errs
}

// checkImages: every image is an existing help asset with a meaningful alt text (accessibility, WCAG 1.1.1).
func checkImages(where, body string, assets map[string]Asset) []error {
	var errs []error
	prose := inlineRE.ReplaceAllString(fenceRE.ReplaceAllString(body, ""), "")
	for _, m := range imageRE.FindAllStringSubmatch(prose, -1) {
		alt, target := strings.TrimSpace(m[1]), m[2]
		name, ok := strings.CutPrefix(target, AssetPrefix)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("%s: image %q must be %s<file>", where, target, AssetPrefix))
		case assets[name].Data == nil:
			errs = append(errs, fmt.Errorf("%s: image %q is not in docs/help/assets", where, target))
		case len([]rune(alt)) < 8:
			errs = append(errs, fmt.Errorf("%s: image %q needs a descriptive alt text", where, target))
		}
	}
	return errs
}
