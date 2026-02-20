package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	log "github.com/sirupsen/logrus"
)

func Test_IsOnline(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		wantErr  bool
	}{
		{
			"Succeeds with 200",
			"OK",
			false,
		},
		{
			"Succeeds with other 200-code (202)",
			"Accepted",
			false,
		},
		{
			"Succeeds after following a redirect (301)",
			"Redirect",
			false,
		},
		{
			"Fails if page replies with not generic error (400)",
			"BadRequest",
			true,
		},
		{
			"Fails if page replies with not found error (404)",
			"NotFound",
			true,
		},
		{
			"Fails if page replies with too many requests error (429)",
			"RateLimit",
			true,
		},
		{
			"Fails if page replies with internal server error (500)",
			"ServerError",
			true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ts := setUpMock(tt.scenario)
			defer ts.Close()
			testURL, _ := url.Parse(ts.URL)

			err := IsOnline(*testURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsOnline(%q) error = %v, wantErr %v", testURL.String(), err, tt.wantErr)
			}
		})
	}
}

func setUpMock(scenario string) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle redirect target path
		if scenario == "Redirect" && r.URL.Path == "/redirected" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "Redirected successfully")
			return
		}

		switch scenario {
		case "OK":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "OK")
			return
		case "Accepted":
			w.WriteHeader(http.StatusAccepted)
			_, _ = fmt.Fprintln(w, "Accepted")
			return
		case "Redirect":
			// Redirect to the same mock server to avoid external calls
			http.Redirect(w, r, "/redirected", http.StatusMovedPermanently)
			return
		case "BadRequest":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		case "NotFound":
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		case "RateLimit":
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		case "ServerError":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusNotFound)
			return
		default:
			log.Fatalf("Unimplemented scenario %q provided", scenario)
			return
		}
	}))

	return ts
}
