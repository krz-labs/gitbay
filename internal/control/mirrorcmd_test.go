package control

import (
	"strconv"
	"strings"
	"testing"

	"github.com/krz-labs/gitbay/internal/protocol"
)

// mirror add refuses a host sync would refuse as numeric, on an
// instance that allows local targets or not, and stores nothing (#298).
func TestMirrorAddRefusesNumericHost(t *testing.T) {
	for _, allowLocal := range []bool{true, false} {
		for _, host := range []string{"127.1", "2130706433", "0x7f.1"} {
			c, errOut, st, _ := importCtx(t, allowLocal)
			code := Dispatch(c, []string{"repo", "mirror", "add", "alice/app", "https://" + host + "/x.git", "--direction", "pull"})
			if code != protocol.ExitUsage || !strings.Contains(errOut.String(), "numeric address") {
				t.Fatalf("%s (allow_local %v): exit %d, %q", host, allowLocal, code, errOut.String())
			}
			repo, err := st.RepoByPath("alice/app")
			if err != nil {
				t.Fatal(err)
			}
			if ms, err := st.ListMirrors(repo.ID); err != nil || len(ms) != 0 {
				t.Fatalf("%s: mirrors %v, %v", host, ms, err)
			}
		}
	}
}

// set-url changes the URL in place, keeping the id and the sealed token,
// and refuses a different host or an unknown id.
func TestMirrorSetURL(t *testing.T) {
	c, errOut, st, _ := importCtx(t, false)
	repo, err := st.RepoByPath("alice/app")
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.AddMirror(repo.ID, "push", "https://github.com/old/app.git", "u", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetMirrorResult(id, ""); err != nil {
		t.Fatal(err)
	}
	ids := strconv.FormatInt(id, 10)

	if code := Dispatch(c, []string{"repo", "mirror", "set-url", "alice/app", ids, "https://example.com/new/app.git"}); code != protocol.ExitUsage || !strings.Contains(errOut.String(), "host cannot change") {
		t.Fatalf("other host: exit %d, %q", code, errOut.String())
	}
	if code := Dispatch(c, []string{"repo", "mirror", "set-url", "alice/app", "999", "https://github.com/new/app.git"}); code != protocol.ExitNotFound {
		t.Fatalf("unknown id: exit %d, %q", code, errOut.String())
	}
	if code := Dispatch(c, []string{"repo", "mirror", "set-url", "alice/app", ids, "https://github.com/new/app.git"}); code != protocol.ExitOK {
		t.Fatalf("set-url: exit %d, %q", code, errOut.String())
	}
	ms, err := st.ListMirrors(repo.ID)
	if err != nil || len(ms) != 1 {
		t.Fatalf("mirrors %v, %v", ms, err)
	}
	m := ms[0]
	if m.ID != id || m.URL != "https://github.com/new/app.git" || m.Token != "secret" || m.Username != "u" || !m.Dirty {
		t.Fatalf("mirror after set-url: %+v", m)
	}
}
