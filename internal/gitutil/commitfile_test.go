package gitutil

import "testing"

func TestCommitFileChangeEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	if err := InitBare(dir, "main", ""); err != nil {
		t.Fatal(err)
	}
	sha, err := CommitFileChange(dir, "main", "profile/README.md",
		[]byte("# hello\n"), "alice", "alice@example.org", "add about")
	if err != nil {
		t.Fatalf("first commit into an empty repository: %v", err)
	}
	if sha == "" {
		t.Fatal("no sha returned")
	}
	raw, err := ReadBlob(dir, "main", "profile/README.md", 1<<20)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if string(raw) != "# hello\n" {
		t.Errorf("read back %q", raw)
	}

	// The second commit still takes the parented path.
	if _, err := CommitFileChange(dir, "main", "profile/README.md",
		[]byte("# hello again\n"), "alice", "alice@example.org", "edit"); err != nil {
		t.Fatalf("second commit: %v", err)
	}

	// An unknown branch in a repository that has history is a typo, not a
	// new orphan branch.
	if _, err := CommitFileChange(dir, "nope", "x.md",
		[]byte("x"), "alice", "alice@example.org", "x"); err == nil {
		t.Error("committing to an unknown branch of a non-empty repository succeeded")
	}
}

// --from creates a branch; it never commits onto one that exists, and a
// branch created meanwhile is not overwritten.
func TestCommitFileFromCreatesOnly(t *testing.T) {
	dir := t.TempDir()
	if err := InitBare(dir, "main", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitFileChange(dir, "main", "a.md", []byte("a\n"), "alice", "alice@example.org", "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitFileFrom(dir, "topic", "main", "a.md", []byte("b\n"), "alice", "alice@example.org", "b"); err != nil {
		t.Fatalf("creating topic from main: %v", err)
	}
	if _, err := CommitFileFrom(dir, "topic", "main", "a.md", []byte("c\n"), "alice", "alice@example.org", "c"); err == nil {
		t.Error("--from committed onto an existing branch")
	}
	raw, _ := ReadBlob(dir, "topic", "a.md", 1<<20)
	if string(raw) != "b\n" {
		t.Errorf("topic holds %q, want %q", raw, "b\n")
	}
	main, _ := ResolveRef(dir, "refs/heads/main")
	if err := updateRefCreate(dir, "refs/heads/topic", main); err == nil {
		t.Error("updateRefCreate moved an existing ref")
	}
}

func TestFirstParents(t *testing.T) {
	dir := t.TempDir()
	if err := InitBare(dir, "main", ""); err != nil {
		t.Fatal(err)
	}
	a, err := CommitFileChange(dir, "main", "a.md", []byte("a\n"), "alice", "alice@example.org", "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CommitFileChange(dir, "main", "a.md", []byte("b\n"), "alice", "alice@example.org", "b")
	if err != nil {
		t.Fatal(err)
	}
	got := FirstParents(dir, []string{b, a})
	if got[b] != a || len(got) != 1 {
		t.Errorf("FirstParents = %v, want {%s: %s}", got, b, a)
	}
}
