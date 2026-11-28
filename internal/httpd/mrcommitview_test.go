package httpd

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/krz-labs/gitbay/internal/control"
)

// The per-commit diff is read-only (no comment links: review comments
// are anchored to the head diff's lines), and only a commit the merge
// request carries is shown.
func TestMRCommitViewReadOnlyAndInRange(t *testing.T) {
	st, cfg, alice, _, repo, n, v1, v2, _ := rangeDiffFixture(t)
	git := rangeDiffGitRunner(t)
	dir := control.RepoDir(cfg.Server.Root, repo.OwnerName, repo.Name)
	git(dir, "update-ref", "refs/merge-requests/"+strconv.FormatInt(n, 10)+"/head", v2)
	s := New(cfg, st, nil)
	cookie := loginCookie(t, st, alice.ID)

	get := func(query string) string {
		t.Helper()
		req := httptest.NewRequest("GET", "/alice/secret/mrs/"+strconv.FormatInt(n, 10)+query, nil)
		req.SetPathValue("owner", repo.OwnerName)
		req.SetPathValue("repo", repo.Name)
		req.SetPathValue("n", strconv.FormatInt(n, 10))
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		s.mr(rr, req)
		if rr.Code != 200 {
			t.Fatalf("%s: status %d", query, rr.Code)
		}
		return rr.Body.String()
	}

	full := get("?view=diff")
	if !strings.Contains(full, `class="cmt"`) {
		t.Fatal("full diff has no comment links")
	}
	one := get("?view=diff&commit=" + v2)
	if strings.Contains(one, `class="cmt"`) {
		t.Error("per-commit diff offers comment links")
	}
	if !strings.Contains(one, "read-only") {
		t.Error("per-commit diff does not say it is read-only")
	}
	// v1 was amended away: it is in the repository's objects but not in
	// the merge request's range, so the page falls back to the full diff.
	stale := get("?view=diff&commit=" + v1)
	if strings.Contains(stale, "read-only") || !strings.Contains(stale, `class="cmt"`) {
		t.Error("a commit outside the range was shown as the selected commit")
	}
}
