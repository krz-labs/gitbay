package httpd

import (
	"net/http"

	"github.com/krz-labs/gitbay/internal/store"
)

// railPin is one pinned repository in the rail's quick switcher.
type railPin struct {
	Owner string
	Name  string
}

// rail is the viewer's cross-repo state the layout needs: the unread count
// on the bell and pinned repositories for the quick switcher.
type rail struct {
	Unread int
	Pins   []railPin
}

// basePage is what the layout needs on every page, repo or not. Page
// structs embed it so the rail and the site name are always in scope.
type basePage struct {
	// Site is the instance's display name; Host is the name a command or
	// URL must use. Anything copy-pasteable takes Host.
	Site   string
	Host   string
	Viewer string
	Admin  bool // the viewer is an instance admin: the rail shows /admin
	Rail   rail
	// Theme is stamped on <html> as data-theme: light or dark when the
	// viewer chose one, empty when the browser's own scheme decides.
	Theme string
	// AbuseURL and TermsURL are the operator's footer links, empty when
	// not configured.
	AbuseURL string
	TermsURL string
}

// anonBase is the layout-wide data with no viewer.
func (s *Server) anonBase() basePage {
	return basePage{Site: s.siteName(), Host: s.cfg.SiteHost(), AbuseURL: s.cfg.Web.AbuseURL, TermsURL: s.cfg.Web.TermsURL}
}

// base builds the layout-wide data for a request that has not already
// resolved a viewer.
func (s *Server) base(r *http.Request) basePage {
	if s.cfg.Web.Mode != "accounts" {
		return s.anonBase()
	}
	return s.baseFor(s.viewer(r))
}

// baseFor is base for a handler that already holds the viewer, so the
// session lookup is not repeated.
func (s *Server) baseFor(viewer store.User) basePage {
	b := s.anonBase()
	if viewer.ID == 0 {
		return b
	}
	b.Viewer = viewer.Username
	b.Admin = viewer.IsAdmin
	b.Rail = s.railFor(viewer)
	if theme, err := s.st.Theme(viewer.ID); err == nil && theme != "system" {
		b.Theme = theme
	}
	return b
}

// railFor collects the viewer's unread count and pinned repositories.
func (s *Server) railFor(viewer store.User) rail {
	var rl rail
	rl.Unread = s.st.UnreadNotices(viewer.ID)
	if pinned, err := s.st.PinnedRepos(viewer.ID); err == nil {
		for _, r := range pinned {
			rl.Pins = append(rl.Pins, railPin{Owner: r.OwnerName, Name: r.Name})
		}
	}
	return rl
}
