package httpd

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitbay.org/gitbay/internal/control"
	"gitbay.org/gitbay/internal/protocol"
	"gitbay.org/gitbay/internal/store"
)

// queryPerPage is how many rows a saved query's page shows before
// offering the next, through query run's own cursor.
const queryPerPage = 50

// queriesPage lists the viewer's saved queries (/{owner}/-/queries), runs
// an ad-hoc query (?q=...), or runs a saved one (/{owner}/-/queries/{name}).
// Saved queries are private: under anyone else's name the page is not found.
func (s *Server) queriesPage(w http.ResponseWriter, r *http.Request, viewer store.User) {
	if r.PathValue("owner") != viewer.Username {
		s.notFound(w, r)
		return
	}
	name := r.PathValue("name")
	adHoc := strings.TrimSpace(r.URL.Query().Get("q"))
	data := struct {
		basePage
		Tab    string
		Saved  []control.SavedQueryOut
		Query  control.SavedQueryOut
		AdHoc  string
		Count  int
		Items  []control.QueryItem
		Next   string
		Notice string
	}{basePage: s.baseFor(viewer), Tab: "dashboard", AdHoc: adHoc, Notice: s.takeFlash(w, r)}
	if name == "" {
		if msg, ok := s.runControlInto(viewer, []string{"query", "list"}, &data.Saved); !ok {
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		if adHoc != "" {
			items, count, next, err := control.RunAdHocQuery(s.st, viewer, adHoc, r.URL.Query().Get("cursor"), queryPerPage)
			if err != nil {
				data.Notice = err.Error()
			} else {
				data.Items = items
				data.Count = count
				if next != "" {
					data.Next = "?" + url.Values{"q": {adHoc}, "cursor": {next}}.Encode()
				}
			}
		}
		s.render(w, "queries.html", data)
		return
	}
	switch code, msg := s.runControlIntoCode(viewer, []string{"query", "show", name}, &data.Query); code {
	case protocol.ExitOK:
	case protocol.ExitNotFound:
		s.notFound(w, r)
		return
	default:
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	if data.Query.Count != nil {
		data.Count = *data.Query.Count
	}
	argv := []string{"query", "run", name, "--limit", strconv.Itoa(queryPerPage)}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		argv = append(argv, "--cursor", cursor)
	}
	var page struct {
		Items []control.QueryItem `json:"items"`
		Next  string              `json:"next"`
	}
	switch code, msg := s.runControlIntoCode(viewer, argv, &page); code {
	case protocol.ExitOK:
	case protocol.ExitUsage:
		http.Error(w, msg, http.StatusBadRequest)
		return
	default:
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	data.Items = page.Items
	if page.Next != "" {
		data.Next = "?" + url.Values{"cursor": {page.Next}}.Encode()
	}
	s.render(w, "queries.html", data)
}

// queriesSubmit saves, pins, unpins, or removes a saved query through the
// same query commands the CLI runs.
func (s *Server) queriesSubmit(w http.ResponseWriter, r *http.Request, viewer store.User) {
	if r.PathValue("owner") != viewer.Username {
		s.notFound(w, r)
		return
	}
	base := "/" + viewer.Username + "/-/queries"
	action := r.FormValue("action")
	name := strings.TrimSpace(r.FormValue("name"))
	switch action {
	case "save":
		qText := strings.TrimSpace(r.FormValue("query"))
		argv := []string{"query", "save", name, qText}
		if r.FormValue("force") == "on" {
			argv = append(argv, "--force")
		}
		if _, msg, ok := s.runControl(viewer, argv); !ok {
			s.setFlash(w, msg)
			dest := base
			if qText != "" {
				dest += "?" + url.Values{"q": {qText}}.Encode()
			}
			http.Redirect(w, r, dest, http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, base+"/"+url.PathEscape(name), http.StatusSeeOther)
	case "pin", "unpin":
		if _, msg, ok := s.runControl(viewer, []string{"query", action, name}); !ok {
			s.setFlash(w, msg)
		}
		http.Redirect(w, r, safeReturnTo(r, base), http.StatusSeeOther)
	case "remove":
		if _, msg, ok := s.runControl(viewer, []string{"query", "remove", name}); !ok {
			s.setFlash(w, msg)
		}
		http.Redirect(w, r, base, http.StatusSeeOther)
	default:
		s.setFlash(w, "unknown query action")
		http.Redirect(w, r, base, http.StatusSeeOther)
	}
}
