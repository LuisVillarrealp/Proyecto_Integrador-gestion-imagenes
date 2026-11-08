package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedirigirAHTTPS(t *testing.T) {
	h := RedirigirAHTTPS("10.10.0.50:8443")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/imagenes?campana=x", nil)
	req.Host = "malo.example"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("GET: código %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://10.10.0.50:8443/imagenes?campana=x" {
		t.Errorf("Location = %q", loc)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/imagenes", strings.NewReader("datos")))
	if rec.Code != http.StatusUpgradeRequired || rec.Header().Get("Location") != "" {
		t.Errorf("POST debe rechazarse sin redirigir: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestCabecerasSeguras(t *testing.T) {
	rec := httptest.NewRecorder()
	CabecerasSeguras(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, k := range []string{"X-Content-Type-Options", "Cache-Control", "Strict-Transport-Security"} {
		if rec.Header().Get(k) == "" {
			t.Errorf("falta la cabecera %s", k)
		}
	}
}
