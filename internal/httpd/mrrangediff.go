package httpd

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/krz-labs/gitbay/internal/protocol"
	"github.com/krz-labs/gitbay/internal/store"
)

type rangeDiffLine struct {
	Class string
	Text  string
}

func parseRangeDiffLines(raw string) []rangeDiffLine {
	if raw == "" {
		return nil
	}
	rawLines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	out := make([]rangeDiffLine, 0, len(rawLines))
	for _, l := range rawLines {
		cls := "rd-ctx"
		switch {
		case strings.HasPrefix(l, "++"), strings.HasPrefix(l, "+"):
			cls = "rd-add"
		case strings.HasPrefix(l, "--"), strings.HasPrefix(l, "-"):
			cls = "rd-del"
		case strings.HasPrefix(l, "    +"):
			cls = "rd-add"
		case strings.HasPrefix(l, "    -"):
			cls = "rd-del"
		case strings.Contains(l, ":  ") && (strings.Contains(l, " ! ") || strings.Contains(l, " = ") || strings.Contains(l, " < ") || strings.Contains(l, " > ")):
			cls = "rd-hdr"
		}
		out = append(out, rangeDiffLine{Class: cls, Text: l})
	}
	return out
}

// mrRangeDiff renders what changed between two revisions of a merge
// request — the same comparison `mr range-diff` prints on the CLI and
// the iOS app already show — so a reviewer whose approval a force-push
// staled can see what moved without leaving the browser (#269).
func (s *Server) mrRangeDiff(w http.ResponseWriter, r *http.Request) {
	p, ok := s.repoFor(w, r, "")
	if !ok {
		return
	}
	p.Tab = "merge requests"
	n, err := strconv.ParseInt(r.PathValue("n"), 10, 64)
	if err != nil {
		s.notFound(w, r)
		return
	}
	m, err := s.st.MRByNumber(p.Repo.ID, n)
	if err != nil {
		s.notFound(w, r)
		return
	}

	viewer := s.webViewer(r)
	argv := mrArgs(r, "range-diff")
	if from := r.URL.Query().Get("from"); from != "" {
		argv = append(argv, "--from", from)
	}
	if to := r.URL.Query().Get("to"); to != "" {
		argv = append(argv, "--to", to)
	}
	// Only an unknown MR 404s (checked above). A bad --from/--to also
	// resolves to nothing in git, which range-diff reports as
	// ExitNotFound too, so that result renders on the page instead of
	// turning a bad query parameter into a 404 (#271).
	out, msg, code := s.runControlCode(viewer, argv)
	errMsg := ""
	if code != protocol.ExitOK {
		errMsg = msg
	}
	s.render(w, "mrrangediff.html", struct {
		repoPage
		MR    store.MR
		Diff  string
		Lines []rangeDiffLine
		Error string
	}{p, m, out, parseRangeDiffLines(out), errMsg})
}
