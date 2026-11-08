package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

func TestTokenDeCabecera(t *testing.T) {
	casos := []struct {
		cabecera string
		quiere   string
		error    bool
	}{
		{"Bearer abc123", "abc123", false},
		{"bearer abc123", "abc123", false},
		{"Bearer", "", true},
		{"Bearer   ", "", true},
		{"Basic abc123", "", true},
		{"", "", true},
	}
	for _, c := range casos {
		got, err := TokenDeCabecera(c.cabecera)
		if got != c.quiere || (err != nil) != c.error {
			t.Errorf("TokenDeCabecera(%q) = %q, %v", c.cabecera, got, err)
		}
		if c.error && !errors.Is(err, models.ErrNoAutenticado) {
			t.Errorf("TokenDeCabecera(%q): se esperaba ErrNoAutenticado, llegó %v", c.cabecera, err)
		}
	}
}

func TestDev(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "usuarios.json")
	json := `{"t-admin": {"id":"u1","nombre":"Admin","rol":"Administrador"}}`
	if err := os.WriteFile(ruta, []byte(json), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := NuevoDev(ruta)
	if err != nil {
		t.Fatal(err)
	}
	u, err := d.Validar("t-admin")
	if err != nil || !u.EsAdministrador() {
		t.Errorf("Validar(t-admin) = %+v, %v", u, err)
	}
	if _, err := d.Validar("otro"); !errors.Is(err, models.ErrNoAutenticado) {
		t.Errorf("token desconocido: error = %v", err)
	}

	malo := filepath.Join(t.TempDir(), "malo.json")
	if err := os.WriteFile(malo, []byte(`{"t": {"id":"u1","nombre":"X","rol":"Dios"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NuevoDev(malo); err == nil {
		t.Error("un rol desconocido debe rechazarse al cargar")
	}
}
