package httpd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitbay.org/gitbay/internal/config"
	"gitbay.org/gitbay/internal/store"
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
