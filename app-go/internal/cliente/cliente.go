// Package cliente implementa las llamadas HTTP al servidor de imágenes.
// No imprime nada ni lee flags: eso es responsabilidad de cmd/client.
package cliente

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/red"
)

// Cliente se comunica con un servidor concreto usando un token y una sede.
type Cliente struct {
	direccion  string // host:puerto del servidor
	token      string
	sede       string
	esquema    string // "https" (siempre, salvo desarrollo en loopback)
	clienteWeb *http.Client
}

// Opciones configura la seguridad de la conexión.
type Opciones struct {
	// CA es la ruta del certificado de la autoridad del proyecto (PEM). Si se
	// indica, SOLO se confía en esa autoridad. Si está vacía se usan las del sistema.
	CA string
	// SinTLS usa HTTP sin cifrar. Solo se acepta hacia loopback (desarrollo).
	SinTLS bool
}

// Nuevo crea un cliente. direccion tiene la forma "host:puerto". La conexión
// usa HTTPS verificando el certificado del servidor; nunca se desactiva la
// verificación y no se siguen redirecciones (así el token no viaja a otro destino).
func Nuevo(direccion, token, sede string, opciones Opciones) (*Cliente, error) {
	direccion = strings.TrimSpace(direccion)
	direccion = strings.TrimPrefix(strings.TrimPrefix(direccion, "https://"), "http://")
	direccion = strings.TrimSuffix(direccion, "/")
	nuevoCliente := &Cliente{direccion: direccion, token: token, sede: sede, esquema: "https"}

	configuracionTLS := &tls.Config{MinVersion: tls.VersionTLS12}
	if opciones.SinTLS {
		if !red.EsLoopback(direccion) {
			return nil, errors.New("conexión sin TLS solo permitida hacia loopback: use HTTPS con -ca")
		}
		nuevoCliente.esquema = "http"
	} else if opciones.CA != "" {
		certificadoPEM, err := os.ReadFile(opciones.CA)
		if err != nil {
			return nil, fmt.Errorf("leer certificado de la CA: %w", err)
		}
		autoridadesConfiables := x509.NewCertPool()
		if !autoridadesConfiables.AppendCertsFromPEM(certificadoPEM) {
			return nil, fmt.Errorf("el archivo %s no contiene un certificado PEM válido", opciones.CA)
		}
		configuracionTLS.RootCAs = autoridadesConfiables
	}
	nuevoCliente.clienteWeb = &http.Client{
		Timeout:   60 * time.Second,
		Transport: &http.Transport{TLSClientConfig: configuracionTLS},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return nuevoCliente, nil
}

// ErrorAPI es un rechazo del servidor (código HTTP 4xx o 5xx con su mensaje).
type ErrorAPI struct {
	Estado  int
	Mensaje string
}

func (errorAPI *ErrorAPI) Error() string {
	return fmt.Sprintf("%s (código %d)", errorAPI.Mensaje, errorAPI.Estado)
}

func errorDeRespuesta(respuesta *http.Response) error {
	var cuerpo struct {
		Error string `json:"error"`
	}
	datos, _ := io.ReadAll(io.LimitReader(respuesta.Body, 1<<20))
	_ = json.Unmarshal(datos, &cuerpo)
	mensaje := cuerpo.Error
	if mensaje == "" {
		mensaje = http.StatusText(respuesta.StatusCode)
	}
	return &ErrorAPI{Estado: respuesta.StatusCode, Mensaje: mensaje}
}

// enviar manda la petición con las cabeceras comunes. Si el servidor responde
// 4xx/5xx devuelve *ErrorAPI; si no se puede conectar, un error que lo explica.
func (cliente *Cliente) enviar(contexto context.Context, metodo, ruta string, cuerpo io.Reader, tipoContenido string) (*http.Response, error) {
	direccionCompleta := cliente.esquema + "://" + cliente.direccion + ruta
	peticion, err := http.NewRequestWithContext(contexto, metodo, direccionCompleta, cuerpo)
	if err != nil {
		return nil, fmt.Errorf("armar la petición: %w", err)
	}
	if tipoContenido != "" {
		peticion.Header.Set("Content-Type", tipoContenido)
	}
	if cliente.token != "" {
		peticion.Header.Set("Authorization", "Bearer "+cliente.token)
	}
	if cliente.sede != "" {
		peticion.Header.Set("X-Sede", cliente.sede)
	}
	respuesta, err := cliente.clienteWeb.Do(peticion)
	if err != nil {
		var errorDeURL *url.Error
		if errors.As(err, &errorDeURL) {
			err = errorDeURL.Err // deja solo la causa («connection refused»), sin repetir la URL
		}
		return nil, fmt.Errorf("no se pudo conectar con el servidor en %s: %w", cliente.direccion, err)
	}
	if respuesta.StatusCode >= 300 && respuesta.StatusCode < 400 {
		respuesta.Body.Close()
		return nil, fmt.Errorf("el servidor respondió con una redirección (código %d) y no se sigue por seguridad; verifique la dirección y el puerto HTTPS", respuesta.StatusCode)
	}
	if respuesta.StatusCode >= 400 {
		defer respuesta.Body.Close()
		return nil, errorDeRespuesta(respuesta)
	}
	return respuesta, nil
}

func decodificar[Tipo any](respuesta *http.Response) (Tipo, error) {
	defer respuesta.Body.Close()
	var valor Tipo
	if err := json.NewDecoder(respuesta.Body).Decode(&valor); err != nil {
		return valor, fmt.Errorf("respuesta no válida del servidor: %w", err)
	}
	return valor, nil
}

// Salud es la respuesta de GET /salud.
type Salud struct {
	Estado string `json:"estado"`
}

// Ping comprueba que el servidor responde (espera como máximo 5 segundos).
func (cliente *Cliente) Ping() (Salud, error) {
	contexto, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	respuesta, err := cliente.enviar(contexto, http.MethodGet, "/salud", nil, "")
	if err != nil {
		return Salud{}, err
	}
	return decodificar[Salud](respuesta)
}

// MegabytesLegibles convierte bytes a un texto como "10.5 MB".
func MegabytesLegibles(bytes int64) string {
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
}

// Subir envía una imagen con sus metadatos. El archivo se transmite sin
// cargarlo completo en memoria.
func (cliente *Cliente) Subir(rutaArchivo string, meta models.MetaImagen) (models.Imagen, error) {
	archivo, err := os.Open(rutaArchivo)
	if err != nil {
		return models.Imagen{}, fmt.Errorf("abrir el archivo: %w", err)
	}
	defer archivo.Close()
	informacion, err := archivo.Stat()
	if err != nil {
		return models.Imagen{}, fmt.Errorf("leer el archivo: %w", err)
	}
	if informacion.IsDir() {
		return models.Imagen{}, fmt.Errorf("%s es un directorio, no una imagen", rutaArchivo)
	}
	// El servidor vuelve a validar; esto evita enviar un archivo que ya se sabe
	// que será rechazado y le muestra al usuario cuánto pesa.
	if informacion.Size() > models.TamanoMaximo {
		return models.Imagen{}, fmt.Errorf("%w: «%s» pesa %s", models.ErrTamanoExcedido,
			filepath.Base(rutaArchivo), MegabytesLegibles(informacion.Size()))
	}

	lectorTuberia, escritorTuberia := io.Pipe()
	defer lectorTuberia.Close()
	formulario := multipart.NewWriter(escritorTuberia)
	go func() {
		escritorTuberia.CloseWithError(escribirFormulario(formulario, archivo, filepath.Base(rutaArchivo), meta))
	}()

	respuesta, err := cliente.enviar(context.Background(), http.MethodPost, "/imagenes", lectorTuberia, formulario.FormDataContentType())
	if err != nil {
		return models.Imagen{}, err
	}
	return decodificar[models.Imagen](respuesta)
}

func escribirFormulario(formulario *multipart.Writer, archivo io.Reader, nombreArchivo string, meta models.MetaImagen) error {
	camposDeTexto := [][2]string{
		{"nombre", meta.Nombre},
		{"campana", meta.Campana},
		{"etiquetas", strings.Join(meta.Etiquetas, ",")},
		{"confidencialidad", string(meta.Confidencialidad)},
	}
	for _, campo := range camposDeTexto {
		if err := formulario.WriteField(campo[0], campo[1]); err != nil {
			return err
		}
	}
	parteArchivo, err := formulario.CreateFormFile("archivo", nombreArchivo)
	if err != nil {
		return err
	}
	if _, err := io.Copy(parteArchivo, archivo); err != nil {
		return err
	}
	return formulario.Close()
}

// Consulta son los filtros de un listado; los campos vacíos no filtran y el
// servidor valida su contenido.
type Consulta struct {
	Campana          string
	Etiqueta         string
	Confidencialidad string
	Desde            string // AAAA-MM-DD o RFC 3339
}

// Listar devuelve los metadatos de las imágenes que el usuario puede ver.
func (cliente *Cliente) Listar(consulta Consulta) ([]models.Imagen, error) {
	parametros := url.Values{}
	filtros := map[string]string{
		"campana":          consulta.Campana,
		"etiqueta":         consulta.Etiqueta,
		"confidencialidad": consulta.Confidencialidad,
		"desde":            consulta.Desde,
	}
	for nombreFiltro, valorFiltro := range filtros {
		if valorFiltro != "" {
			parametros.Set(nombreFiltro, valorFiltro)
		}
	}
	ruta := "/imagenes"
	if len(parametros) > 0 {
		ruta += "?" + parametros.Encode()
	}
	respuesta, err := cliente.enviar(context.Background(), http.MethodGet, ruta, nil, "")
	if err != nil {
		return nil, err
	}
	return decodificar[[]models.Imagen](respuesta)
}

// Buscar devuelve los metadatos de una imagen entre las que el usuario puede ver.
func (cliente *Cliente) Buscar(id string) (models.Imagen, error) {
	imagenesVisibles, err := cliente.Listar(Consulta{})
	if err != nil {
		return models.Imagen{}, err
	}
	for _, imagen := range imagenesVisibles {
		if imagen.ID() == id {
			return imagen, nil
		}
	}
	return models.Imagen{}, &ErrorAPI{Estado: http.StatusNotFound, Mensaje: "no se encontró esa imagen entre las que puede ver"}
}

// Descarga es una imagen en transferencia. Quien la recibe debe cerrar Cuerpo.
type Descarga struct {
	Nombre string // nombre sugerido, ya sin rutas
	Tamano int64  // -1 si el servidor no lo informó
	Cuerpo io.ReadCloser
}

// Descargar pide el archivo de una imagen.
func (cliente *Cliente) Descargar(id string) (*Descarga, error) {
	respuesta, err := cliente.enviar(context.Background(), http.MethodGet, "/imagenes/"+url.PathEscape(id), nil, "")
	if err != nil {
		return nil, err
	}
	return &Descarga{Nombre: nombreSugerido(respuesta, id), Tamano: respuesta.ContentLength, Cuerpo: respuesta.Body}, nil
}

// nombreSugerido obtiene el nombre de archivo que propone el servidor. No se
// confía en él: se eliminan separadores de ruta para que no pueda escribir
// fuera del directorio elegido por el usuario.
func nombreSugerido(respuesta *http.Response, id string) string {
	if _, parametros, err := mime.ParseMediaType(respuesta.Header.Get("Content-Disposition")); err == nil {
		sinSeparadores := strings.NewReplacer("/", "_", "\\", "_").Replace(parametros["filename"])
		nombre := strings.TrimSpace(sinSeparadores)
		if nombre != "" && nombre != "." && nombre != ".." {
			return nombre
		}
	}
	extension := ".bin"
	switch respuesta.Header.Get("Content-Type") {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	}
	return id + extension
}

// Actualizar reemplaza los metadatos editables de una imagen.
func (cliente *Cliente) Actualizar(id string, nuevaMeta models.MetaImagen) (models.Imagen, error) {
	cuerpoJSON, err := json.Marshal(nuevaMeta)
	if err != nil {
		return models.Imagen{}, fmt.Errorf("armar la petición: %w", err)
	}
	respuesta, err := cliente.enviar(context.Background(), http.MethodPatch, "/imagenes/"+url.PathEscape(id),
		strings.NewReader(string(cuerpoJSON)), "application/json")
	if err != nil {
		return models.Imagen{}, err
	}
	return decodificar[models.Imagen](respuesta)
}

// Eliminar borra una imagen.
func (cliente *Cliente) Eliminar(id string) error {
	respuesta, err := cliente.enviar(context.Background(), http.MethodDelete, "/imagenes/"+url.PathEscape(id), nil, "")
	if err != nil {
		return err
	}
	respuesta.Body.Close()
	return nil
}
