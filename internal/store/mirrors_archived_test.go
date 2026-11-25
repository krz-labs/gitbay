package store

import "testing"

func TestArchivedRepoDisablesMirrors(t *testing.T) {
	st, err := Open(":memory:")
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
	mid, err := st.AddMirror(repoID, "push", "https://github.com/alice/app.git", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetMirrorResult(mid, "403 archived"); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkMirrorsDirty(repoID, "push"); err != nil {
		t.Fatal(err)
	}

	// Active repo: mirror is due and shows up in admin queues.
	due, err := st.DueMirrors(3600)
	if err != nil || len(due) != 1 {
		t.Fatalf("active DueMirrors = %+v, err=%v", due, err)
	}
	q, err := st.QueueStatus()
	if err != nil || q.Mirrors.Errors != 1 || len(q.Mirrors.Items) != 1 {
		t.Fatalf("active Queues.Mirrors = %+v, err=%v", q.Mirrors, err)
	}

	// Archived repo: mirror is skipped by DueMirrors and hidden from admin mirror errors.
	if _, err := st.UpdateRepoSettings(repoID, func(s *RepoSettings) { s.Archived = true }); err != nil {
		t.Fatal(err)
	}
	due, err = st.DueMirrors(3600)
	if err != nil || len(due) != 0 {
		t.Fatalf("archived DueMirrors = %+v, err=%v", due, err)
	}
	q, err = st.QueueStatus()
	if err != nil || q.Mirrors.Errors != 0 || q.Mirrors.Dirty != 0 || len(q.Mirrors.Items) != 0 {
		t.Fatalf("archived Queues.Mirrors = %+v, err=%v", q.Mirrors, err)
	}

	// ClearMirrorsPending wipes dirty and last_error so unarchiving starts clean.
	if err := st.ClearMirrorsPending(repoID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateRepoSettings(repoID, func(s *RepoSettings) { s.Archived = false }); err != nil {
		t.Fatal(err)
	}
	q, err = st.QueueStatus()
	if err != nil || q.Mirrors.Errors != 0 || q.Mirrors.Dirty != 0 {
		t.Fatalf("unarchived after clear Queues.Mirrors = %+v, err=%v", q.Mirrors, err)
	}
}
