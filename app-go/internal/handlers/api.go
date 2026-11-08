package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/auth"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

// ServicioImagenes es lo que la capa HTTP necesita de la lógica de negocio.
// Es una interfaz para poder probar los handlers sin disco real.
type ServicioImagenes interface {
	Subir(usuario models.Usuario, meta models.MetaImagen, datosArchivo io.Reader) (models.Imagen, error)
	Listar(usuario models.Usuario, filtro models.Filtro) []models.Imagen
	Descargar(usuario models.Usuario, id string) (models.Imagen, io.ReadCloser, error)
	Actualizar(usuario models.Usuario, id string, nuevaMeta models.MetaImagen) (models.Imagen, error)
	Eliminar(usuario models.Usuario, id string) error
}

// Registrador recibe los eventos del registro de actividad.
type Registrador interface {
	Registrar(evento models.Evento)
}

// API agrupa las rutas de gestión de imágenes.
type API struct {
	servicio     ServicioImagenes
	autenticador auth.Autenticador
	actividad    Registrador
}

// NuevaAPI crea la capa HTTP. actividad puede ser nil para no registrar.
func NuevaAPI(servicio ServicioImagenes, autenticador auth.Autenticador, actividad Registrador) *API {
	return &API{servicio: servicio, autenticador: autenticador, actividad: actividad}
}

// Rutas registra las rutas de imágenes. Todas exigen "Authorization: Bearer <token>".
func (api *API) Rutas(enrutador *http.ServeMux) {
	enrutador.HandleFunc("POST /imagenes", api.autenticado("subir", api.subir))
	enrutador.HandleFunc("GET /imagenes", api.autenticado("listar", api.listar))
	enrutador.HandleFunc("GET /imagenes/{id}", api.autenticado("descargar", api.descargar))
	enrutador.HandleFunc("PATCH /imagenes/{id}", api.autenticado("actualizar", api.actualizar))
	enrutador.HandleFunc("DELETE /imagenes/{id}", api.autenticado("eliminar", api.eliminar))
}

// resultado es lo que devuelve cada operación para que el envoltorio responda
// los errores y registre la actividad en un solo lugar.
type resultado struct {
	recurso string
	tamano  int64
	err     error
	escrito bool // ya se empezó a escribir la respuesta: no escribir otro error
}

type operacion func(respuesta http.ResponseWriter, peticion *http.Request, usuario models.Usuario) resultado

// autenticado valida el token, ejecuta la operación, responde el error si lo
// hubo y registra el evento con su tiempo de respuesta.
func (api *API) autenticado(nombreOperacion string, ejecutar operacion) http.HandlerFunc {
	return func(respuesta http.ResponseWriter, peticion *http.Request) {
		inicio := time.Now()
		var resultadoOperacion resultado
		usuario, err := api.usuarioDe(peticion)
		if err != nil {
			resultadoOperacion.err = err
		} else {
			resultadoOperacion = ejecutar(respuesta, peticion, usuario)
		}
		if resultadoOperacion.err != nil && !resultadoOperacion.escrito {
			EscribirError(respuesta, resultadoOperacion.err)
		}
		api.registrar(peticion, usuario, nombreOperacion, resultadoOperacion, inicio)
	}
}

func (api *API) usuarioDe(peticion *http.Request) (models.Usuario, error) {
	token, err := auth.TokenDeCabecera(peticion.Header.Get("Authorization"))
	if err != nil {
		return models.Usuario{}, err
	}
	return api.autenticador.Validar(token)
}

func (api *API) registrar(peticion *http.Request, usuario models.Usuario, nombreOperacion string, resultadoOperacion resultado, inicio time.Time) {
	if api.actividad == nil {
		return
	}
	idUsuario := usuario.ID()
	if idUsuario == "" {
		idUsuario = "anonimo"
	}
	evento := models.Evento{
		Fecha:      inicio.UTC(),
		Usuario:    idUsuario,
		Sede:       sedeDe(peticion),
		Operacion:  nombreOperacion,
		Recurso:    resultadoOperacion.recurso,
		Resultado:  resultadoDe(resultadoOperacion.err),
		Tamano:     resultadoOperacion.tamano,
		DuracionMs: float64(time.Since(inicio).Microseconds()) / 1000,
	}
	if resultadoOperacion.err != nil {
		evento.Detalle = resultadoOperacion.err.Error()
	}
	api.actividad.Registrar(evento)
}

func resultadoDe(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, models.ErrNoAutorizado), errors.Is(err, models.ErrNoAutenticado):
		return "denegado"
	default:
		return "error"
	}
}

// sedeDe lee la sede de origen que informa el cliente (cabecera X-Sede).
func sedeDe(peticion *http.Request) string {
	const largoMaximo = 40
	sede := []rune(strings.TrimSpace(peticion.Header.Get("X-Sede")))
	if len(sede) == 0 {
		return "no-indicada"
	}
	if len(sede) > largoMaximo {
		sede = sede[:largoMaximo]
	}
	return string(sede)
}

// POST /imagenes — multipart con el archivo en "archivo" y los campos
// nombre, campana, etiquetas (separadas por coma) y confidencialidad.
func (api *API) subir(respuesta http.ResponseWriter, peticion *http.Request, usuario models.Usuario) resultado {
	const margenFormulario = 1 << 20 // 1 MB extra para los campos de texto
	peticion.Body = http.MaxBytesReader(respuesta, peticion.Body, models.TamanoMaximo+margenFormulario)
	if err := peticion.ParseMultipartForm(margenFormulario); err != nil {
		return resultado{err: errorDeFormulario(err)}
	}
	defer peticion.MultipartForm.RemoveAll()

	archivo, _, err := peticion.FormFile("archivo")
	if err != nil {
		return resultado{err: fmt.Errorf("%w: falta el campo «archivo»", models.ErrDatosInvalidos)}
	}
	defer archivo.Close()

	meta := models.MetaImagen{
		Nombre:           peticion.FormValue("nombre"),
		Campana:          peticion.FormValue("campana"),
		Etiquetas:        strings.Split(peticion.FormValue("etiquetas"), ","),
		Confidencialidad: models.Confidencialidad(peticion.FormValue("confidencialidad")),
	}
	imagen, err := api.servicio.Subir(usuario, meta, archivo)
	if err != nil {
		return resultado{err: err}
	}
	EscribirJSON(respuesta, http.StatusCreated, imagen)
	return resultado{recurso: imagen.ID(), tamano: imagen.Tamano()}
}

func errorDeFormulario(err error) error {
	var limiteSuperado *http.MaxBytesError
	if errors.As(err, &limiteSuperado) {
		return models.ErrTamanoExcedido
	}
	return fmt.Errorf("%w: formulario multipart inválido", models.ErrDatosInvalidos)
}

// GET /imagenes?campana=&etiqueta=&confidencialidad=&desde=
func (api *API) listar(respuesta http.ResponseWriter, peticion *http.Request, usuario models.Usuario) resultado {
	parametros := peticion.URL.Query()
	filtro := models.Filtro{Campana: parametros.Get("campana"), Etiqueta: parametros.Get("etiqueta")}
	if nivelTexto := parametros.Get("confidencialidad"); nivelTexto != "" {
		nivel, err := models.ParseConfidencialidad(nivelTexto)
		if err != nil {
			return resultado{err: err}
		}
		filtro.Confidencialidad = nivel
	}
	if fechaTexto := parametros.Get("desde"); fechaTexto != "" {
		fecha, err := parseFecha(fechaTexto)
		if err != nil {
			return resultado{err: err}
		}
		filtro.Desde = fecha
	}
	EscribirJSON(respuesta, http.StatusOK, api.servicio.Listar(usuario, filtro))
	return resultado{}
}

func parseFecha(fechaTexto string) (time.Time, error) {
	for _, formato := range []string{time.RFC3339, "2006-01-02"} {
		if fecha, err := time.Parse(formato, fechaTexto); err == nil {
			return fecha, nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: fecha %q no válida (use AAAA-MM-DD o RFC 3339)", models.ErrDatosInvalidos, fechaTexto)
}

// GET /imagenes/{id} — entrega el archivo.
func (api *API) descargar(respuesta http.ResponseWriter, peticion *http.Request, usuario models.Usuario) resultado {
	id := peticion.PathValue("id")
	imagen, contenido, err := api.servicio.Descargar(usuario, id)
	if err != nil {
		return resultado{recurso: id, err: err}
	}
	defer contenido.Close()

	tipoContenido := "image/png"
	if imagen.Formato() == models.FormatoJPG {
		tipoContenido = "image/jpeg"
	}
	nombreSugerido := imagen.Nombre() + "." + imagen.Formato()
	disposicion := mime.FormatMediaType("attachment", map[string]string{"filename": nombreSugerido})
	if disposicion == "" {
		disposicion = "attachment"
	}
	respuesta.Header().Set("Content-Type", tipoContenido)
	respuesta.Header().Set("Content-Length", strconv.FormatInt(imagen.Tamano(), 10))
	respuesta.Header().Set("Content-Disposition", disposicion)
	respuesta.WriteHeader(http.StatusOK)

	if _, err := io.Copy(respuesta, contenido); err != nil {
		log.Printf("descarga de %s interrumpida: %v", id, err)
		return resultado{recurso: id, tamano: imagen.Tamano(), err: err, escrito: true}
	}
	return resultado{recurso: id, tamano: imagen.Tamano()}
}

// PATCH /imagenes/{id} — cuerpo JSON con nombre, campana, etiquetas y confidencialidad.
func (api *API) actualizar(respuesta http.ResponseWriter, peticion *http.Request, usuario models.Usuario) resultado {
	id := peticion.PathValue("id")
	var nuevaMeta models.MetaImagen
	decodificador := json.NewDecoder(http.MaxBytesReader(respuesta, peticion.Body, 1<<20))
	decodificador.DisallowUnknownFields()
	if err := decodificador.Decode(&nuevaMeta); err != nil {
		return resultado{recurso: id, err: fmt.Errorf("%w: cuerpo JSON inválido", models.ErrDatosInvalidos)}
	}
	imagen, err := api.servicio.Actualizar(usuario, id, nuevaMeta)
	if err != nil {
		return resultado{recurso: id, err: err}
	}
	EscribirJSON(respuesta, http.StatusOK, imagen)
	return resultado{recurso: id}
}

// DELETE /imagenes/{id}
func (api *API) eliminar(respuesta http.ResponseWriter, peticion *http.Request, usuario models.Usuario) resultado {
	id := peticion.PathValue("id")
	if err := api.servicio.Eliminar(usuario, id); err != nil {
		return resultado{recurso: id, err: err}
	}
	EscribirJSON(respuesta, http.StatusOK, map[string]string{"mensaje": "imagen eliminada", "id": id})
	return resultado{recurso: id}
}
