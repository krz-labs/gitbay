package httpd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/krz-labs/gitbay/internal/config"
	"github.com/krz-labs/gitbay/internal/store"
)

func TestWebUXPickersStatebarAndRailPins(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	uid, err := st.CreateUser("alice", false)
	if err != nil {
		t.Fatal(err)
	}
	repoID, err := st.CreateRepo("user", uid, "app", "public")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := st.RepoByID(repoID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PinRepo(uid, repoID); err != nil {
		t.Fatal(err)
	}
	if err := st.SetLabel(repo, "bug", "d73a4a"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateIssue(repoID, uid, "first bug", "body", "md"); err != nil {
		t.Fatal(err)
	}

	var cfg config.Config
	cfg.Server.Root = t.TempDir()
	cfg.Server.SiteURL = "https://forge.test"
	cfg.Web.Mode = "accounts"
	srv := New(cfg, st, nil)
	cookie := sessionCookieFor(t, srv, st, uid)
	h := srv.Handler()

	// Issues list carries quick statebar and rail pinned-repo switcher.
	req := httptest.NewRequest("GET", "/alice/app/issues", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /alice/app/issues: %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`class="statebar"`, `class="railpins"`, `href="/alice/app"`} {
		if !strings.Contains(body, want) {
			t.Errorf("issues list missing %s", want)
		}
	}

	// Issue detail carries popover label & assignee pickers, datalists, and Cmd+K palette.
	req = httptest.NewRequest("GET", "/alice/app/issues/1", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /alice/app/issues/1: %d", rec.Code)
	}
	body = rec.Body.String()
	for _, want := range []string{`id="repo-labels"`, `id="repo-people"`, `class="popover"`, `class="picker-btn"`, `id="cmdk"`, `/static/app.js`} {
		if !strings.Contains(body, want) {
			t.Errorf("issue page missing %s", want)
		}
	}

	// /static/app.js is served with JavaScript content-type.
	req = httptest.NewRequest("GET", "/static/app.js", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("GET /static/app.js: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}

	// Clicking a picker button applies the label.
	form := url.Values{"add": {"bug"}}
	req = httptest.NewRequest("POST", "https://forge.test/alice/app/issues/1/label", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://forge.test")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST label: %d", rec.Code)
	}
	iss, err := st.IssueByNumber(repoID, 1)
	if err != nil || len(iss.Labels) != 1 || iss.Labels[0] != "bug" {
		t.Fatalf("labels after picker click: %+v err=%v", iss.Labels, err)
	}
}

func TestAccountAndAdminUX(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	uid, err := st.CreateUser("admin", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("bob", false); err != nil {
		t.Fatal(err)
	}

	var cfg config.Config
	cfg.Server.Root = t.TempDir()
	cfg.Server.SiteURL = "https://forge.test"
	cfg.Web.Mode = "accounts"
	srv := New(cfg, st, nil)
	cookie := sessionCookieFor(t, srv, st, uid)
	h := srv.Handler()

	// /settings renders setcard sections, autosave forms, and popover confirmations.
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /settings: %d", rec.Code)
	}
	for _, want := range []string{`class="setcard"`, `data-autosave="1"`, `data-live-theme="1"`, `data-scrollspy="1"`, `class="popover"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("/settings missing %s", want)
		}
	}

	// /admin renders unified tabs, overview tiles, and in-place refresh trigger.
	req = httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin: %d", rec.Code)
	}
	for _, want := range []string{`admintabs`, `data-refresh="/admin"`, `id="admin-queues"`, `class="tiles"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("/admin missing %s", want)
		}
	}

	// /admin/users renders live filter input and popover confirm controls.
	req = httptest.NewRequest("GET", "/admin/users", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/users: %d", rec.Code)
	}
	for _, want := range []string{`admintabs`, `data-filter-table="accounts-table"`, `class="popover"`, `bob`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("/admin/users missing %s", want)
		}
	}
}

func TestPaletteJSONAndSafeReturnTo(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	uid, _ := st.CreateUser("alice", false)
	repoID, _ := st.CreateRepo("user", uid, "rocket", "public")
	st.CreateIssue(repoID, uid, "launch pad calibration", "body", "md")

	var cfg config.Config
	cfg.Server.Root = t.TempDir()
	cfg.Server.SiteURL = "https://forge.test"
	cfg.Web.Mode = "accounts"
	srv := New(cfg, st, nil)
	cookie := sessionCookieFor(t, srv, st, uid)
	h := srv.Handler()

	// Palette JSON endpoint returns matching repos and issues.
	req := httptest.NewRequest("GET", "/search?format=palette&repo=alice/rocket&q=launch", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("palette search: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "launch pad calibration") {
		t.Errorf("palette response missing issue: %s", rec.Body.String())
	}

	// safeReturnTo preserves subpage under repo and rejects off-site or unrelated paths.
	rSub := httptest.NewRequest("POST", "https://forge.test/alice/rocket/pin", nil)
	rSub.Host = "forge.test"
	rSub.Header.Set("Referer", "https://forge.test/alice/rocket/issues/1")
	if got := safeReturnTo(rSub, "/alice/rocket"); got != "/alice/rocket/issues/1" {
		t.Errorf("safeReturnTo subpage = %q, want /alice/rocket/issues/1", got)
	}
	rEvil := httptest.NewRequest("POST", "https://forge.test/alice/rocket/pin", nil)
	rEvil.Host = "forge.test"
	rEvil.Header.Set("Referer", "https://evil.example/alice/rocket/issues/1")
	if got := safeReturnTo(rEvil, "/alice/rocket"); got != "/alice/rocket" {
		t.Errorf("safeReturnTo cross-origin = %q, want /alice/rocket", got)
	}
}

func TestANSIBuildLogAndSavedQueriesWebWrite(t *testing.T) {
	got := string(ansiHTML("\x1b[1;31mFAIL\x1b[0m <script>alert(1)</script> \x1b[32mPASS\x1b[0m"))
	for _, want := range []string{`class="ansi-bold ansi-red"`, `FAIL</span>`, `&lt;script&gt;`, `class="ansi-green"`, `PASS</span>`} {
		if !strings.Contains(got, want) {
			t.Errorf("ansiHTML missing %q in %s", want, got)
		}
	}

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	uid, _ := st.CreateUser("alice", false)
	repoID, _ := st.CreateRepo("user", uid, "app", "public")
	st.CreateIssue(repoID, uid, "triage target", "", "md")

	var cfg config.Config
	cfg.Server.Root = t.TempDir()
	cfg.Server.SiteURL = "https://forge.test"
	cfg.Web.Mode = "accounts"
	srv := New(cfg, st, nil)
	cookie := sessionCookieFor(t, srv, st, uid)
	h := srv.Handler()

	// Save a query over the web, pin it, and run an ad-hoc query.
	form := url.Values{"action": {"save"}, "name": {"triage"}, "query": {"is:open"}}
	req := httptest.NewRequest("POST", "https://forge.test/alice/-/queries", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://forge.test")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/alice/-/queries/triage" {
		t.Fatalf("save query: %d loc=%q", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest("GET", "/alice/-/queries?q=is%3Aopen", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "triage target") {
		t.Fatalf("ad-hoc query: %d\n%s", rec.Code, rec.Body.String())
	}
}

func TestSettingsParitySectionsAndBrowserSessions(t *testing.T) {
	e := newSettingsEnv(t)
	rr := e.post(e.alice, url.Values{"field": {"secret-set"}, "secret-name": {"API_KEY"}, "secret-value": {"super-secret-val"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("secret-set: %d %s", rr.Code, rr.Body.String())
	}
	body := e.page(e.alice).Body.String()
	if !strings.Contains(body, "API_KEY") || strings.Contains(body, "super-secret-val") {
		t.Fatalf("secret visibility wrong on settings page:\n%s", body)
	}
	for _, want := range []string{`id="buildsecrets"`, `id="deploykeys"`, `id="mirrors"`, `id="domains"`} {
		if !strings.Contains(body, want) {
			t.Errorf("repo settings missing %s", want)
		}
	}

	// Account settings lists browser sessions and revokes one.
	cookie := sessionCookieFor(t, e.s, e.st, e.alice.ID)
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.s.accountPage(rec, req, e.alice)
	if !strings.Contains(rec.Body.String(), `id="sessions"`) || !strings.Contains(rec.Body.String(), `this browser`) {
		t.Errorf("account settings missing browser sessions:\n%s", rec.Body.String())
	}
}
