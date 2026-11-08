package cliente

import (
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNombreDeNoPermiteRutas(t *testing.T) {
	casos := []struct {
		disposicion string
		tipo        string
		quiere      string
	}{
		{`attachment; filename="foto.png"`, "image/png", "foto.png"},
		{`attachment; filename="../../etc/passwd"`, "image/png", ".._.._etc_passwd"},
		{`attachment; filename="..\\..\\evil.png"`, "image/png", ".._.._evil.png"},
		{`attachment; filename=".."`, "image/jpeg", "abc.jpg"},
		{`attachment`, "image/png", "abc.png"},
		{``, "text/plain", "abc.bin"},
	}
	for _, c := range casos {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Content-Disposition", c.disposicion)
		resp.Header.Set("Content-Type", c.tipo)
		got := nombreSugerido(resp, "abc")
		if got != c.quiere || strings.ContainsAny(got, `/\`) {
			t.Errorf("nombreSugerido(%q) = %q, se esperaba %q", c.disposicion, got, c.quiere)
		}
	}
}

func TestErrorAPIConservaCodigoYMensaje(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"operación no autorizada para este usuario"}`))
	}))
	defer srv.Close()

	c, _ := Nuevo(strings.TrimPrefix(srv.URL, "http://"), "t", "", Opciones{SinTLS: true})
	_, err := c.Listar(Consulta{})
	var rechazo *ErrorAPI
	if !errors.As(err, &rechazo) || rechazo.Estado != 403 || !strings.Contains(rechazo.Mensaje, "no autorizada") {
		t.Errorf("error = %#v", err)
	}
}

func TestRespuestaSinJSONUsaTextoEstandar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c, _ := Nuevo(srv.URL, "t", "", Opciones{SinTLS: true}) // acepta la dirección con http://
	_, err := c.Listar(Consulta{})
	var rechazo *ErrorAPI
	if !errors.As(err, &rechazo) || rechazo.Estado != 502 || rechazo.Mensaje != "Bad Gateway" {
		t.Errorf("error = %#v", err)
	}
}

func TestTLSConCAyRechazoSinCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"estado":"ok"}`))
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "https://")

	ca := filepath.Join(t.TempDir(), "ca.pem")
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(ca, pemCert, 0o600); err != nil {
		t.Fatal(err)
	}

	conCA, err := Nuevo(addr, "", "", Opciones{CA: ca})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := conCA.Ping(); err != nil || s.Estado != "ok" {
		t.Errorf("con la CA debe conectar: %v %v", s, err)
	}

	sinCA, _ := Nuevo(addr, "", "", Opciones{})
	if _, err := sinCA.Ping(); err == nil {
		t.Error("sin confiar en la CA el certificado debe rechazarse")
	}
}

func TestSinTLSSoloLoopback(t *testing.T) {
	if _, err := Nuevo("10.10.0.50:8443", "", "", Opciones{SinTLS: true}); err == nil {
		t.Error("sin TLS hacia una dirección de red debe rechazarse")
	}
	if _, err := Nuevo("127.0.0.1:8443", "", "", Opciones{SinTLS: true}); err != nil {
		t.Errorf("sin TLS hacia loopback es válido en desarrollo: %v", err)
	}
	if _, err := Nuevo("localhost:1", "", "", Opciones{CA: "no-existe.pem"}); err == nil {
		t.Error("una CA inexistente debe fallar al crear el cliente")
	}
}

func TestNoSigueRedirecciones(t *testing.T) {
	destino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("el cliente no debía seguir la redirección")
	}))
	defer destino.Close()
	srv := httptest.NewServer(http.RedirectHandler(destino.URL, http.StatusPermanentRedirect))
	defer srv.Close()

	c, _ := Nuevo(srv.URL, "secreto", "", Opciones{SinTLS: true})
	if _, err := c.Listar(Consulta{}); err == nil || !strings.Contains(err.Error(), "redirección") {
		t.Errorf("error = %v", err)
	}
}
