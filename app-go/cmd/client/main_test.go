package main

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/cliente"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/handlers"
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

// servidorReal levanta el servidor de verdad (mismas capas que cmd/server)
// sobre un directorio temporal y devuelve su dirección host:puerto.
// usuarioDePrueba crea un usuario válido para las pruebas.
func usuarioDePrueba(id, nombre string, rol models.Rol, campanas ...string) models.Usuario {
	usuario, err := models.NuevoUsuario(id, nombre, rol, campanas...)
	if err != nil {
		panic(err) // solo en pruebas: un dato de prueba mal escrito
	}
	return usuario
}

func servidorReal(t *testing.T) string {
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
	mux := http.NewServeMux()
	mux.Handle("GET /salud", handlers.Salud())
	handlers.NuevaAPI(services.NuevoImagenes(meta, arch), aut, nil).Rutas(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func correrCliente(stdin string, args ...string) (string, error) {
	var out bytes.Buffer
	err := ejecutar(args, strings.NewReader(stdin), &out)
	return out.String(), err
}

func pngEnDisco(t *testing.T, dir, nombre string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(dir, nombre)
	if err := os.WriteFile(ruta, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), ruta
}

func codigoDe(err error) int {
	var rechazo *cliente.ErrorAPI
	if errors.As(err, &rechazo) {
		return rechazo.Estado
	}
	return 0
}

func TestFlujoDelCliente(t *testing.T) {
	addr := servidorReal(t)
	dir := t.TempDir()
	original, rutaPNG := pngEnDisco(t, dir, "camiseta.png")
	autor := []string{"-addr", addr, "-sin-tls", "-token", "t-autor", "-sede", "sede-a"}
	otro := []string{"-addr", addr, "-sin-tls", "-token", "t-otro"}
	admin := []string{"-addr", addr, "-sin-tls", "-token", "t-admin"}
	con := func(base []string, resto ...string) []string { return append(append([]string{}, base...), resto...) }

	// ping no necesita token.
	if out, err := correrCliente("", "-addr", addr, "-sin-tls", "ping"); err != nil || !strings.Contains(out, "conexión verificada") {
		t.Fatalf("ping: %q, %v", out, err)
	}

	// subir: muestra el ID.
	out, err := correrCliente("", con(autor, "subir", "-archivo", rutaPNG, "-campana", "Eliminatorias", "-etiquetas", "camiseta,oficial", "-confidencialidad", "confidencial")...)
	if err != nil {
		t.Fatalf("subir: %v", err)
	}
	coincidencia := regexp.MustCompile(`ID:\s+([0-9a-f]{32})`).FindStringSubmatch(out)
	if coincidencia == nil {
		t.Fatalf("la salida de subir no muestra el ID:\n%s", out)
	}
	id := coincidencia[1]
	if !strings.Contains(out, "Nombre:           camiseta") {
		t.Errorf("el nombre debía tomarse del archivo:\n%s", out)
	}

	// un texto con extensión .png lo rechaza el servidor (415).
	falsa := filepath.Join(dir, "falsa.png")
	os.WriteFile(falsa, []byte("solo texto"), 0o600)
	if _, err := correrCliente("", con(autor, "subir", "-archivo", falsa, "-campana", "Eliminatorias", "-confidencialidad", "interna")...); codigoDe(err) != 415 {
		t.Errorf("texto como png: error = %v", err)
	}
	// faltan datos obligatorios (400) y archivo inexistente.
	if _, err := correrCliente("", con(autor, "subir", "-archivo", rutaPNG, "-confidencialidad", "interna")...); codigoDe(err) != 400 {
		t.Errorf("sin campaña: error = %v", err)
	}
	if _, err := correrCliente("", con(autor, "subir", "-archivo", filepath.Join(dir, "no-existe.png"), "-campana", "X", "-confidencialidad", "interna")...); err == nil {
		t.Error("un archivo inexistente debe dar error")
	}

	// listar: el autor la ve, el otro empleado no, y un filtro sin coincidencias avisa.
	if out, _ := correrCliente("", con(autor, "listar", "-etiqueta", "oficial")...); !strings.Contains(out, id) || !strings.Contains(out, "1 imagen(es)") {
		t.Errorf("el autor debía ver su imagen:\n%s", out)
	}
	if out, _ := correrCliente("", con(otro, "listar")...); !strings.Contains(out, "No hay imágenes") {
		t.Errorf("otro empleado no debe ver una confidencial ajena:\n%s", out)
	}
	if out, _ := correrCliente("", con(autor, "listar", "-campana", "otra")...); !strings.Contains(out, "No hay imágenes") {
		t.Errorf("sin coincidencias debe avisar:\n%s", out)
	}

	// descargar: idéntica; no pisa archivos salvo con -forzar.
	destino := filepath.Join(dir, "bajada.png")
	if _, err := correrCliente("", con(autor, "descargar", "-id", id, "-salida", destino)...); err != nil {
		t.Fatalf("descargar: %v", err)
	}
	if bajada, _ := os.ReadFile(destino); !bytes.Equal(bajada, original) {
		t.Error("el archivo descargado no es idéntico al original")
	}
	if _, err := correrCliente("", con(autor, "descargar", "-id", id, "-salida", destino)...); err == nil || !strings.Contains(err.Error(), "-forzar") {
		t.Errorf("sin -forzar no debe sobrescribir: %v", err)
	}
	if _, err := correrCliente("", con(autor, "descargar", "-id", id, "-salida", destino, "-forzar")...); err != nil {
		t.Errorf("con -forzar debe sobrescribir: %v", err)
	}
	// permisos: el otro empleado recibe 403.
	if _, err := correrCliente("", con(otro, "descargar", "-id", id, "-salida", filepath.Join(dir, "robada.png"))...); codigoDe(err) != 403 {
		t.Errorf("descarga ajena: error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "robada.png")); err == nil {
		t.Error("una descarga denegada no debe crear archivo")
	}

	// actualizar: cambia solo lo indicado; sin campos da error.
	if out, err := correrCliente("", con(autor, "actualizar", "-id", id, "-confidencialidad", "interna")...); err != nil || !strings.Contains(out, "interna") || !strings.Contains(out, "Eliminatorias") {
		t.Errorf("actualizar: %q, %v", out, err)
	}
	if _, err := correrCliente("", con(autor, "actualizar", "-id", id)...); err == nil {
		t.Error("actualizar sin campos debe dar error")
	}

	// eliminar: ahora es interna, el otro empleado la ve pero no puede eliminarla (403).
	if _, err := correrCliente("", con(otro, "eliminar", "-id", id, "-si")...); codigoDe(err) != 403 {
		t.Errorf("eliminar ajena: error = %v", err)
	}
	// la confirmación cancela con «n» y elimina con «s».
	if out, err := correrCliente("n\n", con(autor, "eliminar", "-id", id)...); err != nil || !strings.Contains(out, "cancelada") {
		t.Errorf("cancelar: %q, %v", out, err)
	}
	if out, _ := correrCliente("", con(autor, "listar")...); !strings.Contains(out, id) {
		t.Error("tras cancelar, la imagen debe seguir existiendo")
	}
	if out, err := correrCliente("s\n", con(admin, "eliminar", "-id", id)...); err != nil || !strings.Contains(out, "eliminada") {
		t.Errorf("eliminar: %q, %v", out, err)
	}
	if _, err := correrCliente("", con(admin, "descargar", "-id", id, "-salida", filepath.Join(dir, "x.png"))...); codigoDe(err) != 404 {
		t.Errorf("descargar eliminada: error = %v", err)
	}
}

func TestErroresDeUso(t *testing.T) {
	addr := servidorReal(t)

	if _, err := correrCliente("", "-addr", addr, "-sin-tls", "listar"); err == nil || !strings.Contains(err.Error(), "token") {
		t.Errorf("sin token: %v", err)
	}
	if _, err := correrCliente("", "-addr", addr, "-sin-tls", "-token", "falso", "listar"); codigoDe(err) != 401 {
		t.Errorf("token inválido: %v", err)
	}
	if _, err := correrCliente("", "-addr", addr, "-sin-tls", "-token", "t-autor", "volar"); err == nil || !strings.Contains(err.Error(), "desconocido") {
		t.Errorf("comando desconocido: %v", err)
	}
	if _, err := correrCliente("", "-addr", addr, "-sin-tls", "-token", "t-autor", "descargar"); err == nil || !strings.Contains(err.Error(), "-id") {
		t.Errorf("falta -id: %v", err)
	}
}

func TestErrorDeConexion(t *testing.T) {
	addr := servidorReal(t)
	// Se apaga el servidor: la dirección queda sin nadie escuchando.
	srvApagado := httptest.NewServer(http.NotFoundHandler())
	addr = strings.TrimPrefix(srvApagado.URL, "http://")
	srvApagado.Close()

	_, err := correrCliente("", "-addr", addr, "-sin-tls", "ping")
	if err == nil || !strings.Contains(err.Error(), "no se pudo conectar con el servidor en "+addr) {
		t.Errorf("error de conexión poco claro: %v", err)
	}
	if codigoDe(err) != 0 {
		t.Error("un fallo de conexión no es un rechazo HTTP")
	}
}
