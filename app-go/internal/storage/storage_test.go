package storage

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

func imagenDePrueba(t *testing.T, id string) models.Imagen {
	t.Helper()
	meta := models.MetaImagen{Nombre: "foto-" + id, Campana: "Eliminatorias", Confidencialidad: models.Interna}
	img, err := models.NuevaImagen(id, meta, "u1", "png", 100, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestMetadatosPersisteYRecarga(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "metadatos.json")
	m, err := NuevoMetadatos(ruta)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if err := m.Crear(imagenDePrueba(t, id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Crear(imagenDePrueba(t, "a")); !errors.Is(err, models.ErrDatosInvalidos) {
		t.Errorf("ID duplicado: error = %v", err)
	}

	recargado, err := NuevoMetadatos(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if recargado.Total() != 2 {
		t.Fatalf("tras recargar hay %d imágenes, se esperaban 2", recargado.Total())
	}

	if err := recargado.Eliminar("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := recargado.Obtener("a"); !errors.Is(err, models.ErrNoEncontrado) {
		t.Errorf("Obtener de eliminada: error = %v", err)
	}
	if err := recargado.Eliminar("a"); !errors.Is(err, models.ErrNoEncontrado) {
		t.Errorf("Eliminar inexistente: error = %v", err)
	}
	if lista := recargado.Listar(models.Filtro{Campana: "no-existe"}); lista == nil || len(lista) != 0 {
		t.Errorf("sin coincidencias debe devolver lista vacía no nula, llegó %v", lista)
	}
}

func TestArchivos(t *testing.T) {
	a, err := NuevoArchivos(filepath.Join(t.TempDir(), "imagenes"))
	if err != nil {
		t.Fatal(err)
	}

	for _, malo := range []string{"../escape.png", "a/b.png", "..", ""} {
		if _, err := a.Guardar(malo, strings.NewReader("x")); !errors.Is(err, models.ErrDatosInvalidos) {
			t.Errorf("Guardar(%q) debe rechazarse, error = %v", malo, err)
		}
	}

	if n, err := a.Guardar("ok.png", strings.NewReader("contenido")); err != nil || n != 9 {
		t.Fatalf("Guardar = %d, %v", n, err)
	}
	f, err := a.Abrir("ok.png")
	if err != nil {
		t.Fatal(err)
	}
	datos, _ := io.ReadAll(f)
	f.Close()
	if string(datos) != "contenido" {
		t.Errorf("contenido leído = %q", datos)
	}

	if err := a.Eliminar("ok.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Abrir("ok.png"); !errors.Is(err, models.ErrNoEncontrado) {
		t.Errorf("Abrir de eliminado: error = %v", err)
	}
}
