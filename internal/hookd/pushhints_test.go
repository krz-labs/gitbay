package hookd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/krz-labs/gitbay/internal/config"
	"github.com/krz-labs/gitbay/internal/control"
	"github.com/krz-labs/gitbay/internal/policy"
	"github.com/krz-labs/gitbay/internal/store"
)

func TestPushHints(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "gitbay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	uid, err := st.CreateUser("alice", true)
	if err != nil {
		t.Fatal(err)
	}
	repoID, err := st.CreateRepo("user", uid, "app", "public")
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	cfg.Server.Root = root
	cfg.Server.SiteURL = "https://gitbay.org"

	// Default branch push produces no hint.
	got := control.PushHints(st, cfg, repoID, []policy.RefUpdate{
		{Ref: "refs/heads/main", Old: strings.Repeat("0", 40), New: strings.Repeat("a", 40)},
	})
	if got != "" {
		t.Fatalf("default branch hint = %q, want empty", got)
	}

	// Feature branch without an MR suggests creating one.
	got = control.PushHints(st, cfg, repoID, []policy.RefUpdate{
		{Ref: "refs/heads/feat-x", Old: strings.Repeat("0", 40), New: strings.Repeat("a", 40)},
	})
	if !strings.Contains(got, "gitbay mr create --source feat-x") ||
		!strings.Contains(got, "https://gitbay.org/alice/app/mrs/new?source=feat-x") {
		t.Fatalf("new branch hint = %q", got)
	}

	// Branch deletion produces no hint.
	got = control.PushHints(st, cfg, repoID, []policy.RefUpdate{
		{Ref: "refs/heads/feat-x", Old: strings.Repeat("a", 40), New: strings.Repeat("0", 40), IsDelete: true},
	})
	if got != "" {
		t.Fatalf("delete hint = %q, want empty", got)
	}

	// Branch with an open MR prints the existing MR URL.
	n, err := st.CreateMR(repoID, uid, repoID, "feat-x", "main", "my feat", "", strings.Repeat("a", 40), "md", false)
	if err != nil {
		t.Fatal(err)
	}
	got = control.PushHints(st, cfg, repoID, []policy.RefUpdate{
		{Ref: "refs/heads/feat-x", Old: strings.Repeat("a", 40), New: strings.Repeat("b", 40)},
	})
	want := "open merge request !1: https://gitbay.org/alice/app/mrs/1"
	if n != 1 || got != want {
		t.Fatalf("open MR hint = %q, want %q", got, want)
	}
}
