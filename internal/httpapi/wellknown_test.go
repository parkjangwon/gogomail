package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogomail/gogomail/internal/httpapi"
)

func TestWellKnownCalDAVRedirect(t *testing.T) {
	mux := http.NewServeMux()
	httpapi.RegisterWellKnownRoutes(mux, "", "")

	for _, method := range []string{"GET", "PROPFIND", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/.well-known/caldav", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("expected 301, got %d", rec.Code)
			}
			loc := rec.Header().Get("Location")
			if loc != "/caldav/" {
				t.Fatalf("expected Location /caldav/, got %q", loc)
			}
		})
	}
}

func TestWellKnownCardDAVRedirect(t *testing.T) {
	mux := http.NewServeMux()
	httpapi.RegisterWellKnownRoutes(mux, "", "")

	for _, method := range []string{"GET", "PROPFIND", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/.well-known/carddav", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("expected 301, got %d", rec.Code)
			}
			loc := rec.Header().Get("Location")
			if loc != "/carddav/" {
				t.Fatalf("expected Location /carddav/, got %q", loc)
			}
		})
	}
}

func TestWellKnownCustomURL(t *testing.T) {
	mux := http.NewServeMux()
	httpapi.RegisterWellKnownRoutes(mux, "https://cal.example.com/", "https://card.example.com/")

	req := httptest.NewRequest("GET", "/.well-known/caldav", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://cal.example.com/" {
		t.Fatalf("expected custom CalDAV URL, got %q", loc)
	}
}

func TestWellKnownUnknownPath(t *testing.T) {
	mux := http.NewServeMux()
	httpapi.RegisterWellKnownRoutes(mux, "", "")

	req := httptest.NewRequest("GET", "/.well-known/unknown", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown well-known URI, got %d", rec.Code)
	}
}
