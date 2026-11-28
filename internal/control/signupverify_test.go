package control

import (
	"testing"
	"time"

	"github.com/krz-labs/gitbay/internal/config"
	"github.com/krz-labs/gitbay/internal/store"
)

// A code minted by `email add` on an active account does not sign in
// through the signup screen; a pending signup's code does.
func TestVerifyPendingSignupOnlyPending(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.MigrateUp(); err != nil {
		t.Fatal(err)
	}

	active, err := st.CreateUser("alice", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddEmail(active, "bob@example.test", "", false); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateEmailToken(active, "bob@example.test", store.HashToken("c0de"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, msg := VerifyPendingSignup(config.Config{}, st, "alice", "c0de"); msg == "" {
		t.Fatal("active account signed in with an email-add code")
	}

	pending, err := st.RegisterOpen("carol", "carol@example.test", "SHA256:x", "ssh-ed25519", []byte("blob"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateEmailToken(pending, "carol@example.test", store.HashToken("f00d"), time.Hour); err != nil {
		t.Fatal(err)
	}
	u, msg := VerifyPendingSignup(config.Config{}, st, "carol", "f00d")
	if msg != "" || u.ID != pending || u.Pending {
		t.Fatalf("pending signup: user %+v, msg %q", u, msg)
	}
}
