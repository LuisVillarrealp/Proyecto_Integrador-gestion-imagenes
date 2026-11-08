package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/services"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/storage"
)

type autFalso map[string]models.Usuario

func (a autFalso) Validar(token string) (models.Usuario, error) {
	if u, ok := a[token]; ok {
		return u, nil
	}
	return models.Usuario{}, models.ErrNoAutenticado
}

type registroFalso struct {
	mu      sync.Mutex
	eventos []models.Evento
}

// usuarioDePrueba crea un usuario válido para las pruebas.
func usuarioDePrueba(id, nombre string, rol models.Rol, campanas ...string) models.Usuario {
	usuario, err := models.NuevoUsuario(id, nombre, rol, campanas...)
	if err != nil {
		panic(err) // solo en pruebas: un dato de prueba mal escrito
	}
	return usuario
}

func (r *registroFalso) Registrar(e models.Evento) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventos = append(r.eventos, e)
}

type entorno struct {
	mux      *http.ServeMux
	registro *registroFalso
}

func nuevoEntorno(t *testing.T) *entorno {
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
	aut := autFalso{
		"t-admin": usuarioDePrueba("u1", "Admin", models.RolAdministrador),
		"t-autor": usuarioDePrueba("u2", "Autor", models.RolEmpleado, "Eliminatorias"),
		"t-otro":  usuarioDePrueba("u3", "Otro", models.RolEmpleado, "Redes"),
	}
	reg := &registroFalso{}
	mux := http.NewServeMux()
	NuevaAPI(services.NuevoImagenes(meta, arch), aut, reg).Rutas(mux)
	return &entorno{mux: mux, registro: reg}
}

func (e *entorno) peticion(metodo, ruta, token, tipo string, cuerpo io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, ruta, cuerpo)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if tipo != "" {
		req.Header.Set("Content-Type", tipo)
	}
	req.Header.Set("X-Sede", "sede-a")
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func pngDePrueba(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartDe(t *testing.T, nombreArchivo string, contenido []byte, nivel string) (*bytes.Buffer, string) {
	t.Helper()
	cuerpo := &bytes.Buffer{}
	mw := multipart.NewWriter(cuerpo)
	campos := map[string]string{"nombre": "camiseta", "campana": "Eliminatorias", "etiquetas": "Camiseta, oficial", "confidencialidad": nivel}
	for k, v := range campos {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if nombreArchivo != "" {
		fw, err := mw.CreateFormFile("archivo", nombreArchivo)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(contenido)
	}
	mw.Close()
	return cuerpo, mw.FormDataContentType()
}

func subir(t *testing.T, e *entorno, token, nombreArchivo string, contenido []byte, nivel string) *httptest.ResponseRecorder {
	t.Helper()
	cuerpo, tipo := multipartDe(t, nombreArchivo, contenido, nivel)
	return e.peticion(http.MethodPost, "/imagenes", token, tipo, cuerpo)
}

func TestFlujoCompleto(t *testing.T) {
	e := nuevoEntorno(t)
	original := pngDePrueba(t)

	// Sin token o con token inválido: 401.
	if rec := e.peticion(http.MethodGet, "/imagenes", "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("sin token: código = %d", rec.Code)
	}
	if rec := e.peticion(http.MethodGet, "/imagenes", "falso", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("token inválido: código = %d", rec.Code)
	}

	// Subir una confidencial: 201.
	rec := subir(t, e, "t-autor", "foto.png", original, "confidencial")
	if rec.Code != http.StatusCreated {
		t.Fatalf("subir: código = %d, cuerpo = %s", rec.Code, rec.Body)
	}
	var img models.Imagen
	if err := json.Unmarshal(rec.Body.Bytes(), &img); err != nil || img.ID() == "" {
		t.Fatalf("respuesta de subir inválida: %v %s", err, rec.Body)
	}

	// Listar con y sin coincidencias.
	var lista []models.Imagen
	rec = e.peticion(http.MethodGet, "/imagenes?etiqueta=camiseta", "t-autor", "", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &lista); err != nil || len(lista) != 1 {
		t.Errorf("listar: %d elementos, err=%v", len(lista), err)
	}
	rec = e.peticion(http.MethodGet, "/imagenes?campana=otra", "t-autor", "", nil)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("sin coincidencias debe ser lista vacía, llegó %q", rec.Body)
	}
	if rec = e.peticion(http.MethodGet, "/imagenes?confidencialidad=secreta", "t-autor", "", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("filtro inválido: código = %d", rec.Code)
	}

	// Descargar: idéntica a la original.
	rec = e.peticion(http.MethodGet, "/imagenes/"+img.ID(), "t-autor", "", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("descargar: código = %d, tipo = %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if sha256.Sum256(rec.Body.Bytes()) != sha256.Sum256(original) {
		t.Error("la descarga no es idéntica a la imagen subida")
	}

	// Texto renombrado como PNG: 415 y no se guarda.
	if rec = subir(t, e, "t-autor", "falsa.png", []byte("solo texto"), "interna"); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("texto como png: código = %d, cuerpo = %s", rec.Code, rec.Body)
	}
	// Falta el archivo: 400.
	if rec = subir(t, e, "t-autor", "", nil, "interna"); rec.Code != http.StatusBadRequest {
		t.Errorf("sin archivo: código = %d", rec.Code)
	}

	// Otro empleado: 403 al descargar y al eliminar la confidencial ajena.
	if rec = e.peticion(http.MethodGet, "/imagenes/"+img.ID(), "t-otro", "", nil); rec.Code != http.StatusForbidden {
		t.Errorf("descargar ajena: código = %d", rec.Code)
	}
	if rec = e.peticion(http.MethodDelete, "/imagenes/"+img.ID(), "t-otro", "", nil); rec.Code != http.StatusForbidden {
		t.Errorf("eliminar ajena: código = %d", rec.Code)
	}

	// Actualizar metadatos: el autor sí, con JSON inválido no.
	cambio := `{"nombre":"camiseta","campana":"Eliminatorias","etiquetas":["oficial"],"confidencialidad":"interna"}`
	if rec = e.peticion(http.MethodPatch, "/imagenes/"+img.ID(), "t-autor", "application/json", strings.NewReader(cambio)); rec.Code != http.StatusOK {
		t.Errorf("actualizar: código = %d, cuerpo = %s", rec.Code, rec.Body)
	}
	if rec = e.peticion(http.MethodPatch, "/imagenes/"+img.ID(), "t-autor", "application/json", strings.NewReader(`{"campo_raro":1}`)); rec.Code != http.StatusBadRequest {
		t.Errorf("JSON con campos desconocidos: código = %d", rec.Code)
	}

	// Ahora es interna, pero otro empleado de OTRA campaña no la ve (RF-SEC-03);
	// el administrador sí puede eliminarla.
	if rec = e.peticion(http.MethodGet, "/imagenes/"+img.ID(), "t-otro", "", nil); rec.Code != http.StatusForbidden {
		t.Errorf("descargar de otra campaña: código = %d", rec.Code)
	}
	if rec = e.peticion(http.MethodDelete, "/imagenes/"+img.ID(), "t-admin", "", nil); rec.Code != http.StatusOK {
		t.Errorf("eliminar como admin: código = %d", rec.Code)
	}
	if rec = e.peticion(http.MethodGet, "/imagenes/"+img.ID(), "t-admin", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("descargar eliminada: código = %d", rec.Code)
	}

	// Registro de actividad: hay denegados, con sede, usuario y tiempo.
	e.registro.mu.Lock()
	defer e.registro.mu.Unlock()
	var denegadoEliminar bool
	for _, ev := range e.registro.eventos {
		if ev.Sede != "sede-a" || ev.DuracionMs < 0 || ev.Usuario == "" || ev.Fecha.IsZero() {
			t.Errorf("evento incompleto: %+v", ev)
		}
		if ev.Operacion == "eliminar" && ev.Resultado == "denegado" && ev.Usuario == "u3" {
			denegadoEliminar = true
		}
	}
	if !denegadoEliminar {
		t.Error("el intento denegado de eliminar debe quedar registrado")
	}
}

func TestSubirDemasiadoGrande(t *testing.T) {
	e := nuevoEntorno(t)
	for _, tamano := range []int{int(models.TamanoMaximo) + 1, 12 << 20} {
		rec := subir(t, e, "t-autor", "grande.png", make([]byte, tamano), "interna")
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("archivo de %d bytes: código = %d, se esperaba 413", tamano, rec.Code)
		}
	}
	if rec := e.peticion(http.MethodGet, "/imagenes", "t-admin", "", nil); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("no debía guardarse nada, listado: %s", rec.Body)
	}
}
