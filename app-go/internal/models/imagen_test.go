package models

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"
)

func metaValida() MetaImagen {
	return MetaImagen{Nombre: "camiseta-frente", Campana: "Eliminatorias", Etiquetas: []string{"camiseta"}, Confidencialidad: Confidencial}
}

func TestParseConfidencialidad(t *testing.T) {
	casos := []struct {
		entrada    string
		esperado   Confidencialidad
		debeFallar bool
	}{
		{"interna", Interna, false},
		{"  CONFIDENCIAL ", Confidencial, false},
		{"secreta", "", true},
		{"", "", true},
	}
	for _, caso := range casos {
		obtenido, err := ParseConfidencialidad(caso.entrada)
		if (err != nil) != caso.debeFallar || obtenido != caso.esperado {
			t.Errorf("ParseConfidencialidad(%q) = %q, %v", caso.entrada, obtenido, err)
		}
		if caso.debeFallar && !errors.Is(err, ErrDatosInvalidos) {
			t.Errorf("ParseConfidencialidad(%q): se esperaba ErrDatosInvalidos, llegó %v", caso.entrada, err)
		}
	}
}

func TestNuevaImagenValidaciones(t *testing.T) {
	ahora := time.Now()
	sinNombre := metaValida()
	sinNombre.Nombre = "   "
	nivelInvalido := metaValida()
	nivelInvalido.Confidencialidad = "secreta"

	casos := []struct {
		nombre        string
		meta          MetaImagen
		autor         string
		formato       string
		tamano        int64
		errorEsperado error
	}{
		{"válida", metaValida(), "u1", "png", 1024, nil},
		{"formato no permitido", metaValida(), "u1", "gif", 1024, ErrFormatoInvalido},
		{"supera 10 MB", metaValida(), "u1", "jpg", TamanoMaximo + 1, ErrTamanoExcedido},
		{"exactamente 10 MB", metaValida(), "u1", "jpg", TamanoMaximo, nil},
		{"archivo vacío", metaValida(), "u1", "png", 0, ErrDatosInvalidos},
		{"sin nombre", sinNombre, "u1", "png", 1024, ErrDatosInvalidos},
		{"confidencialidad inválida", nivelInvalido, "u1", "png", 1024, ErrDatosInvalidos},
		{"sin autor", metaValida(), "", "png", 1024, ErrDatosInvalidos},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := NuevaImagen("abc", caso.meta, caso.autor, caso.formato, caso.tamano, ahora)
			if !errors.Is(err, caso.errorEsperado) {
				t.Errorf("error = %v, se esperaba %v", err, caso.errorEsperado)
			}
		})
	}
}

func TestNormalizarEtiquetas(t *testing.T) {
	meta := metaValida()
	meta.Etiquetas = []string{" Camiseta", "camiseta", "", "OFICIAL "}
	imagen, err := NuevaImagen("abc", meta, "u1", "JPG", 10, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if esperadas := []string{"camiseta", "oficial"}; !slices.Equal(imagen.Etiquetas(), esperadas) {
		t.Errorf("etiquetas = %v, se esperaba %v", imagen.Etiquetas(), esperadas)
	}
	if imagen.Formato() != "jpg" || imagen.NombreArchivo() != "abc.jpg" {
		t.Errorf("formato/nombre de archivo = %q / %q", imagen.Formato(), imagen.NombreArchivo())
	}
}

func TestEncapsulamientoDeImagen(t *testing.T) {
	imagen, err := NuevaImagen("abc", metaValida(), "u1", "png", 10, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Modificar las copias que entregan los métodos no altera la imagen.
	etiquetasCopiadas := imagen.Etiquetas()
	etiquetasCopiadas[0] = "alterada"
	metaCopiada := imagen.Meta()
	metaCopiada.Etiquetas[0] = "alterada"
	if imagen.Etiquetas()[0] != "camiseta" {
		t.Error("las etiquetas internas no deben poder modificarse desde fuera")
	}

	// CambiarMeta rechaza datos inválidos y deja la imagen como estaba.
	invalida := metaValida()
	invalida.Campana = ""
	if err := imagen.CambiarMeta(invalida); !errors.Is(err, ErrDatosInvalidos) {
		t.Errorf("CambiarMeta con datos inválidos: %v", err)
	}
	if imagen.Campana() != "Eliminatorias" {
		t.Error("un cambio rechazado no debe modificar la imagen")
	}

	nueva := metaValida()
	nueva.Confidencialidad = Publica
	if err := imagen.CambiarMeta(nueva); err != nil || imagen.Confidencialidad() != Publica {
		t.Errorf("CambiarMeta válido: %v, nivel = %s", err, imagen.Confidencialidad())
	}
}

func TestImagenJSONIdaYVuelta(t *testing.T) {
	original, _ := NuevaImagen("abc", metaValida(), "u1", "png", 10, time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC))
	datos, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var leida Imagen
	if err := json.Unmarshal(datos, &leida); err != nil {
		t.Fatal(err)
	}
	if leida.ID() != "abc" || leida.Autor() != "u1" || leida.Campana() != "Eliminatorias" || !leida.Fecha().Equal(original.Fecha()) {
		t.Errorf("la imagen leída no coincide: %s", datos)
	}

	// Un JSON alterado (formato no permitido) no produce una Imagen.
	alterado := []byte(`{"id":"x","nombre":"n","campana":"c","confidencialidad":"interna","autor":"u1","formato":"exe","tamano":5}`)
	if err := json.Unmarshal(alterado, &leida); !errors.Is(err, ErrFormatoInvalido) {
		t.Errorf("JSON alterado debe rechazarse: %v", err)
	}
}

func TestFiltroCoincide(t *testing.T) {
	imagen, _ := NuevaImagen("abc", metaValida(), "u1", "png", 10, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC))
	casos := []struct {
		nombre   string
		filtro   Filtro
		esperado bool
	}{
		{"vacío no filtra", Filtro{}, true},
		{"campaña igual sin importar mayúsculas", Filtro{Campana: "eliminatorias"}, true},
		{"campaña distinta", Filtro{Campana: "otra"}, false},
		{"etiqueta presente", Filtro{Etiqueta: "Camiseta"}, true},
		{"etiqueta ausente", Filtro{Etiqueta: "balon"}, false},
		{"nivel distinto", Filtro{Confidencialidad: Publica}, false},
		{"fecha desde posterior", Filtro{Desde: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)}, false},
	}
	for _, caso := range casos {
		if obtenido := caso.filtro.Coincide(imagen); obtenido != caso.esperado {
			t.Errorf("%s: Coincide = %v, se esperaba %v", caso.nombre, obtenido, caso.esperado)
		}
	}
}

func TestUsuarioCampanas(t *testing.T) {
	usuario, err := NuevoUsuario(" u2 ", "Ana", RolEmpleado, " Camiseta 2026 ", "", "Redes")
	if err != nil {
		t.Fatal(err)
	}
	if usuario.ID() != "u2" || len(usuario.Campanas()) != 2 {
		t.Errorf("usuario mal construido: %q %v", usuario.ID(), usuario.Campanas())
	}
	if !usuario.TieneCampana("camiseta 2026") || usuario.TieneCampana("Eliminatorias") {
		t.Error("TieneCampana no respeta las campañas asignadas")
	}
	copia := usuario.Campanas()
	copia[0] = "otra"
	if !usuario.TieneCampana("Camiseta 2026") {
		t.Error("las campañas internas no deben poder modificarse desde fuera")
	}
	if _, err := NuevoUsuario("u9", "X", "Invitado"); !errors.Is(err, ErrDatosInvalidos) {
		t.Errorf("rol desconocido debe rechazarse: %v", err)
	}
}
