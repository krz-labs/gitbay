package httpd

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"gitbay.org/gitbay/internal/control"
	"gitbay.org/gitbay/internal/gitutil"
	"gitbay.org/gitbay/internal/policy"
	"gitbay.org/gitbay/internal/store"
)

// searchResult is one hit as the template renders it: the link is built
// here so the template does not have to know each kind's URL shape.
type searchResult struct {
	control.SearchResult
	Marker string
	Href   string
}

type paletteEntry struct {
	Group string `json:"group"`
	Label string `json:"label"`
	Href  string `json:"href"`
}

// globalSearch is /search: repositories, issues and merge requests across
// the instance. It is readable without a session — a visitor sees exactly
// the public rows — so it runs control.Search directly rather than
// dispatching a command as a user who may not exist.
func (s *Server) globalSearch(w http.ResponseWriter, r *http.Request) {
	var viewer store.User
	if s.cfg.Web.Mode == "accounts" {
		viewer = s.viewer(r)
	}
	if r.URL.Query().Get("format") == "palette" {
		s.servePalette(w, r, viewer)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kind := r.URL.Query().Get("kind")
	if kind != "repo" && kind != "issue" && kind != "mr" {
		kind = ""
	}
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	}
	var results []searchResult
	var queryErr string
	if q != "" {
		if len(q) < 2 || len(q) > 200 {
			queryErr = "query must be 2 to 200 characters"
		} else {
			hits, err := control.Search(s.st, s.cfg.Server.Root, viewer.ID, q, kinds)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			for _, h := range hits {
				results = append(results, searchResult{h, control.SearchMarker(h.Kind), searchHref(h)})
			}
		}
	}
	s.render(w, "globalsearch.html", struct {
		basePage
		Tab      string
		Query    string
		Kind     string
		QueryErr string
		Results  []searchResult
	}{s.baseFor(viewer), "sitesearch", q, kind, queryErr, results})
}

func searchHref(h control.SearchResult) string {
	switch h.Kind {
	case "issue":
		return "/" + h.Repo + "/issues/" + strconv.FormatInt(h.Number, 10)
	case "mr":
		return "/" + h.Repo + "/mrs/" + strconv.FormatInt(h.Number, 10)
	default:
		return "/" + h.Repo
	}
}

// servePalette answers Cmd+K / 't' jump queries with files, symbols, issues,
// merge requests, and repositories the viewer can read.
func (s *Server) servePalette(w http.ResponseWriter, r *http.Request, viewer store.User) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	repoPath := strings.TrimSpace(r.URL.Query().Get("repo"))
	var items []paletteEntry

	if repoPath != "" {
		if repo, err := s.st.RepoByPath(repoPath); err == nil {
			grant := ""
			if viewer.ID != 0 {
				grant, _ = s.st.AccessRole(repo.ID, viewer.ID)
			}
			if policy.CanRead(viewer, repo, grant) {
				dir := control.RepoDir(s.cfg.Server.Root, repo.OwnerName, repo.Name)
				ref := strings.TrimSpace(r.URL.Query().Get("ref"))
				if ref == "" {
					ref = repo.DefaultBranch
				}
				if entries, err := gitutil.ListTreeRecursive(dir, ref, ""); err == nil {
					count := 0
					for _, e := range entries {
						if e.Type != "blob" {
							continue
						}
						if q != "" && !strings.Contains(strings.ToLower(e.Name), q) {
							continue
						}
						items = append(items, paletteEntry{
							Group: "File",
							Label: e.Name,
							Href:  "/" + repo.Path() + "/blob/" + ref + "/" + e.Name,
						})
						count++
						if count >= 40 {
							break
						}
					}
				}
				if mrs, err := s.st.QueryMRs(repo.ID, store.MRFilter{State: "open", Limit: 15}); err == nil {
					for _, m := range mrs {
						lbl := "!" + strconv.FormatInt(m.Number, 10) + " " + m.Title
						if q == "" || strings.Contains(strings.ToLower(lbl), q) {
							items = append(items, paletteEntry{
								Group: "MR",
								Label: lbl,
								Href:  "/" + repo.Path() + "/mrs/" + strconv.FormatInt(m.Number, 10),
							})
						}
					}
				}
				if issues, err := s.st.QueryIssues(repo.ID, store.IssueFilter{State: "open", Limit: 15}); err == nil {
					for _, iss := range issues {
						lbl := "#" + strconv.FormatInt(iss.Number, 10) + " " + iss.Title
						if q == "" || strings.Contains(strings.ToLower(lbl), q) {
							items = append(items, paletteEntry{
								Group: "Issue",
								Label: lbl,
								Href:  "/" + repo.Path() + "/issues/" + strconv.FormatInt(iss.Number, 10),
							})
						}
					}
				}
			}
		}
	}

	if len(q) >= 2 && len(q) <= 100 {
		if hits, err := control.Search(s.st, s.cfg.Server.Root, viewer.ID, q, nil); err == nil {
			for i, h := range hits {
				if i >= 20 {
					break
				}
				lbl := h.Repo
				grp := "Repo"
				if h.Number > 0 {
					lbl = h.Repo + control.SearchMarker(h.Kind) + strconv.FormatInt(h.Number, 10) + " " + h.Title
					if h.Kind == "mr" {
						grp = "MR"
					} else {
						grp = "Issue"
					}
				}
				items = append(items, paletteEntry{Group: grp, Label: lbl, Href: searchHref(h)})
			}
		}
	} else if repoPath == "" {
		if repos, err := s.st.ListPublicReposByActivity(); err == nil {
			for i, rp := range repos {
				if i >= 15 {
					break
				}
				if q == "" || strings.Contains(strings.ToLower(rp.Path()), q) {
					items = append(items, paletteEntry{Group: "Repo", Label: rp.Path(), Href: "/" + rp.Path()})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"items": items})
}
