// Package handlers contiene la capa HTTP del servidor: recibe las peticiones,
// llama a la lógica y devuelve respuestas JSON con códigos HTTP coherentes.
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

type respuestaError struct {
	Error string `json:"error"`
}

// CodigoHTTP traduce un error de dominio al código HTTP correspondiente.
func CodigoHTTP(err error) int {
	switch {
	case errors.Is(err, models.ErrNoEncontrado):
		return http.StatusNotFound // 404
	case errors.Is(err, models.ErrNoAutenticado):
		return http.StatusUnauthorized // 401
	case errors.Is(err, models.ErrNoAutorizado):
		return http.StatusForbidden // 403
	case errors.Is(err, models.ErrFormatoInvalido):
		return http.StatusUnsupportedMediaType // 415
	case errors.Is(err, models.ErrTamanoExcedido):
		return http.StatusRequestEntityTooLarge // 413
	case errors.Is(err, models.ErrDatosInvalidos):
		return http.StatusBadRequest // 400
	default:
		return http.StatusInternalServerError // 500
	}
}

// EscribirJSON responde con el estado y el cuerpo en JSON.
func EscribirJSON(respuesta http.ResponseWriter, codigoEstado int, cuerpo any) {
	respuesta.Header().Set("Content-Type", "application/json; charset=utf-8")
	respuesta.WriteHeader(codigoEstado)
	if err := json.NewEncoder(respuesta).Encode(cuerpo); err != nil {
		log.Printf("escribir respuesta: %v", err)
	}
}

// EscribirError responde con el código que corresponde al error. Los errores
// inesperados (500) se registran en el log pero no revelan detalles al cliente.
func EscribirError(respuesta http.ResponseWriter, err error) {
	codigoEstado := CodigoHTTP(err)
	mensaje := err.Error()
	if codigoEstado == http.StatusInternalServerError {
		log.Printf("error interno: %v", err)
		mensaje = "error interno del servidor"
	}
	EscribirJSON(respuesta, codigoEstado, respuestaError{Error: mensaje})
}
