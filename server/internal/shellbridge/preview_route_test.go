package shellbridge

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPermittedPreviewRelayRoutes(t *testing.T) {
	base := "/api/shell/issues/11111111-1111-4111-8111-111111111111/preview/" +
		"22222222-2222-4222-8222-222222222222/1/"
	for _, test := range []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodGet, base, true},
		{http.MethodHead, base + ".well-known/multica-preview-identity", true},
		{http.MethodGet, base + "styles.css", true},
		{http.MethodPost, base, false},
		{http.MethodGet, base + "?token=secret", false},
		{http.MethodGet, "/api/shell/issues/11111111-1111-4111-8111-111111111111/preview/22222222-2222-4222-8222-222222222222/2/", false},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		if got := permitted(request); got != test.want {
			t.Fatalf("permitted(%s %s) = %v, want %v", test.method, test.path, got, test.want)
		}
	}
}
