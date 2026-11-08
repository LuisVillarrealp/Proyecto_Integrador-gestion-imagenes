package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

func TestCodigoHTTP(t *testing.T) {
	casos := []struct {
		err    error
		quiere int
	}{
		{models.ErrNoEncontrado, 404},
		{models.ErrNoAutenticado, 401},
		{models.ErrNoAutorizado, 403},
		{models.ErrFormatoInvalido, 415},
		{models.ErrTamanoExcedido, 413},
		{fmt.Errorf("%w: falta el nombre", models.ErrDatosInvalidos), 400}, // errores envueltos
		{errors.New("disco lleno"), 500},
	}
	for _, c := range casos {
		if got := CodigoHTTP(c.err); got != c.quiere {
			t.Errorf("CodigoHTTP(%v) = %d, se esperaba %d", c.err, got, c.quiere)
		}
	}
}

func TestEscribirErrorOcultaDetallesInternos(t *testing.T) {
	rec := httptest.NewRecorder()
	EscribirError(rec, errors.New("open /srv/secreto/ruta: permission denied"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("código = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secreto") {
		t.Errorf("la respuesta filtra detalles internos: %s", rec.Body.String())
	}
}

func TestSalud(t *testing.T) {
	rec := httptest.NewRecorder()
	Salud()(rec, httptest.NewRequest(http.MethodGet, "/salud", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"estado":"ok"}` {
		t.Errorf("respuesta inesperada: %d %s", rec.Code, rec.Body.String())
	}
}
