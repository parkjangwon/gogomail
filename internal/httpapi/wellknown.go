package httpapi

import "net/http"

// RegisterWellKnownRoutes mounts RFC 6764 autodiscovery redirects.
// Clients that follow /.well-known/caldav or /.well-known/carddav will be
// sent a 301 to the actual DAV service root so they can locate principal URLs
// without manual server configuration.
func RegisterWellKnownRoutes(mux *http.ServeMux, caldavURL, carddavURL string) {
	if caldavURL == "" {
		caldavURL = "/caldav/"
	}
	if carddavURL == "" {
		carddavURL = "/carddav/"
	}
	mux.HandleFunc("/.well-known/caldav", wellKnownRedirect(caldavURL))
	mux.HandleFunc("/.well-known/carddav", wellKnownRedirect(carddavURL))
}

func wellKnownRedirect(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}
