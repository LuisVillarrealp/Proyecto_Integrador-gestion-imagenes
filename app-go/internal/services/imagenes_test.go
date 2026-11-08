package services

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/storage"
)

var (
	admin     = usuarioDePrueba("u1", "Admin", models.RolAdministrador)
	autor     = usuarioDePrueba("u2", "Autor", models.RolEmpleado, "Eliminatorias")
	colega    = usuarioDePrueba("u4", "Colega", models.RolEmpleado, "eliminatorias")
	otroEmpl  = usuarioDePrueba("u3", "Otro", models.RolEmpleado, "Redes")
	sinFiltro = models.Filtro{}
)

// usuarioDePrueba crea un usuario válido para las pruebas.
func usuarioDePrueba(id, nombre string, rol models.Rol, campanas ...string) models.Usuario {
	usuario, err := models.NuevoUsuario(id, nombre, rol, campanas...)
	if err != nil {
		panic(err) // solo en pruebas: un dato de prueba mal escrito
	}
	return usuario
}

func nuevoServicio(t *testing.T) (*Imagenes, string) {
	t.Helper()
	dir := t.TempDir()
	arch, err := storage.NuevoArchivos(filepath.Join(dir, "imagenes"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := storage.NuevoMetadatos(filepath.Join(dir, "metadatos.json"))
	if err != nil {
		t.Fatal(err)
	}
	return NuevoImagenes(meta, arch), dir
}

func metaDe(nivel models.Confidencialidad) models.MetaImagen {
	return models.MetaImagen{Nombre: "camiseta", Campana: "Eliminatorias", Etiquetas: []string{"camiseta"}, Confidencialidad: nivel}
}

func pngValido(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpgValido(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func archivosEnDisco(t *testing.T, dir string) int {
	t.Helper()
	entradas, err := os.ReadDir(filepath.Join(dir, "imagenes"))
	if err != nil {
		t.Fatal(err)
	}
	return len(entradas)
}

func TestSubirYDescargarIdentica(t *testing.T) {
	casos := map[string]struct {
		contenido func(*testing.T) []byte
		formato   string
	}{
		"png": {pngValido, "png"},
		"jpg": {jpgValido, "jpg"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			s, dir := nuevoServicio(t)
			original := c.contenido(t)

			img, err := s.Subir(autor, metaDe(models.Interna), bytes.NewReader(original))
			if err != nil {
				t.Fatal(err)
			}
			if img.ID() == "" || img.Autor() != autor.ID() || img.Formato() != c.formato || img.Tamano() != int64(len(original)) {
				t.Errorf("imagen inesperada: %+v", img)
			}
			if archivosEnDisco(t, dir) != 1 {
				t.Errorf("debía haber 1 archivo en disco")
			}

			_, lector, err := s.Descargar(autor, img.ID())
			if err != nil {
				t.Fatal(err)
			}
			defer lector.Close()
			descargado, _ := io.ReadAll(lector)
			if sha256.Sum256(descargado) != sha256.Sum256(original) {
				t.Error("la imagen descargada no es idéntica a la subida")
			}
		})
	}
}

func TestSubirRechazaYNoGuardaNada(t *testing.T) {
	casos := []struct {
		nombre    string
		contenido []byte
		meta      models.MetaImagen
		quiere    error
	}{
		{"texto renombrado como imagen", []byte("esto no es una imagen, solo texto"), metaDe(models.Interna), models.ErrFormatoInvalido},
		{"gif", append([]byte("GIF89a"), make([]byte, 64)...), metaDe(models.Interna), models.ErrFormatoInvalido},
		{"firma PNG con contenido corrupto", append([]byte("\x89PNG\r\n\x1a\n"), []byte("basura sin estructura de imagen")...), metaDe(models.Interna), models.ErrFormatoInvalido},
		{"más de 10 MB", make([]byte, models.TamanoMaximo+1), metaDe(models.Interna), models.ErrTamanoExcedido},
		{"archivo vacío", nil, metaDe(models.Interna), models.ErrDatosInvalidos},
		{"metadatos inválidos", nil, metaDe("secreta"), models.ErrDatosInvalidos},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s, dir := nuevoServicio(t)
			contenido := c.contenido
			if c.nombre == "metadatos inválidos" {
				contenido = pngValido(t)
			}
			_, err := s.Subir(autor, c.meta, bytes.NewReader(contenido))
			if !errors.Is(err, c.quiere) {
				t.Errorf("error = %v, se esperaba %v", err, c.quiere)
			}
			if s.metadatos.Total() != 0 || archivosEnDisco(t, dir) != 0 {
				t.Errorf("no debía guardarse nada (metadatos=%d, archivos=%d)", s.metadatos.Total(), archivosEnDisco(t, dir))
			}
		})
	}
}

func TestPermisosConfidencial(t *testing.T) {
	s, _ := nuevoServicio(t)
	img, err := s.Subir(autor, metaDe(models.Confidencial), bytes.NewReader(pngValido(t)))
	if err != nil {
		t.Fatal(err)
	}

	// Otro empleado no puede verla, descargarla, modificarla ni eliminarla.
	if got := s.Listar(otroEmpl, sinFiltro); len(got) != 0 {
		t.Errorf("otro empleado no debe listar confidenciales ajenas, vio %d", len(got))
	}
	if _, _, err := s.Descargar(otroEmpl, img.ID()); !errors.Is(err, models.ErrNoAutorizado) {
		t.Errorf("Descargar ajena: error = %v", err)
	}
	if _, err := s.Actualizar(otroEmpl, img.ID(), metaDe(models.Publica)); !errors.Is(err, models.ErrNoAutorizado) {
		t.Errorf("Actualizar ajena: error = %v", err)
	}
	if err := s.Eliminar(otroEmpl, img.ID()); !errors.Is(err, models.ErrNoAutorizado) {
		t.Errorf("Eliminar ajena: error = %v", err)
	}

	// El autor y el administrador sí la ven.
	for _, u := range []models.Usuario{autor, admin} {
		if got := s.Listar(u, sinFiltro); len(got) != 1 {
			t.Errorf("%s debía ver 1 imagen, vio %d", u.ID(), len(got))
		}
	}
	_, lector, err := s.Descargar(admin, img.ID())
	if err != nil {
		t.Fatalf("el administrador debe poder descargar: %v", err)
	}
	lector.Close()
}

func TestVisibilidadPorCampana(t *testing.T) {
	// RF-SEC-03 (Fase 2): se ve solo lo de las campañas asignadas, incluso lo
	// público o interno; lo de otra campaña no existe para el empleado.
	s, _ := nuevoServicio(t)
	for _, nivel := range []models.Confidencialidad{models.Publica, models.Interna, models.Confidencial} {
		if _, err := s.Subir(autor, metaDe(nivel), bytes.NewReader(pngValido(t))); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.Listar(colega, sinFiltro); len(got) != 3 {
		t.Errorf("un colega de la campaña debe ver 3 imágenes, vio %d", len(got))
	}
	if got := s.Listar(otroEmpl, sinFiltro); len(got) != 0 {
		t.Errorf("un empleado de otra campaña no debe ver nada, vio %d", len(got))
	}
	img := s.Listar(colega, sinFiltro)[0]
	if err := s.Eliminar(colega, img.ID()); !errors.Is(err, models.ErrNoAutorizado) {
		t.Errorf("un colega no autor no debe eliminar: %v", err)
	}
	if _, _, err := s.Descargar(otroEmpl, img.ID()); !errors.Is(err, models.ErrNoAutorizado) {
		t.Errorf("descarga de otra campaña: %v", err)
	}
}

func TestSoloSubeYMueveAMisCampanas(t *testing.T) {
	s, dir := nuevoServicio(t)
	ajena := models.MetaImagen{Nombre: "x", Campana: "Redes", Confidencialidad: models.Interna}
	if _, err := s.Subir(autor, ajena, bytes.NewReader(pngValido(t))); !errors.Is(err, models.ErrNoAutorizado) {
		t.Errorf("subir a campaña no asignada: %v", err)
	}
	if s.metadatos.Total() != 0 || archivosEnDisco(t, dir) != 0 {
		t.Error("no debía guardarse nada")
	}
	img, err := s.Subir(autor, metaDe(models.Interna), bytes.NewReader(pngValido(t)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Actualizar(autor, img.ID(), ajena); !errors.Is(err, models.ErrNoAutorizado) {
		t.Errorf("mover a campaña no asignada: %v", err)
	}
	if _, err := s.Subir(admin, ajena, bytes.NewReader(pngValido(t))); err != nil {
		t.Errorf("el administrador sube a cualquier campaña: %v", err)
	}
}

func TestEliminarBorraArchivoYMetadatos(t *testing.T) {
	s, dir := nuevoServicio(t)
	img, err := s.Subir(autor, metaDe(models.Interna), bytes.NewReader(pngValido(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Eliminar(autor, img.ID()); err != nil {
		t.Fatal(err)
	}
	if archivosEnDisco(t, dir) != 0 || s.metadatos.Total() != 0 {
		t.Error("tras eliminar no debe quedar archivo ni metadatos")
	}
	if _, _, err := s.Descargar(autor, img.ID()); !errors.Is(err, models.ErrNoEncontrado) {
		t.Errorf("Descargar eliminada: error = %v", err)
	}
	if err := s.Eliminar(autor, img.ID()); !errors.Is(err, models.ErrNoEncontrado) {
		t.Errorf("Eliminar dos veces: error = %v", err)
	}
}

func TestActualizarMetadatos(t *testing.T) {
	s, _ := nuevoServicio(t)
	img, err := s.Subir(autor, metaDe(models.Confidencial), bytes.NewReader(pngValido(t)))
	if err != nil {
		t.Fatal(err)
	}

	nuevo := metaDe(models.Publica)
	nuevo.Etiquetas = []string{"Oficial", "oficial"}
	if _, err := s.Actualizar(autor, img.ID(), nuevo); err != nil {
		t.Fatal(err)
	}
	guardada, _ := s.metadatos.Obtener(img.ID())
	if guardada.Confidencialidad() != models.Publica || len(guardada.Etiquetas()) != 1 || guardada.Etiquetas()[0] != "oficial" {
		t.Errorf("el cambio no persistió bien: %+v", guardada)
	}

	invalido := metaDe("secreta")
	if _, err := s.Actualizar(admin, img.ID(), invalido); !errors.Is(err, models.ErrDatosInvalidos) {
		t.Errorf("metadatos inválidos: error = %v", err)
	}
	if despues, _ := s.metadatos.Obtener(img.ID()); despues.Confidencialidad() != models.Publica {
		t.Error("una actualización inválida no debe cambiar nada")
	}
	if _, err := s.Actualizar(admin, "no-existe", nuevo); !errors.Is(err, models.ErrNoEncontrado) {
		t.Errorf("id inexistente: error = %v", err)
	}
}
