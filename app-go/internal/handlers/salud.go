package handlers

import "net/http"

// RespuestaSalud es el cuerpo de GET /salud. No revela datos del sistema
// (ni cantidad de imágenes ni versiones): solo indica que el servidor responde.
type RespuestaSalud struct {
	Estado string `json:"estado"`
}

// Salud responde GET /salud sin exigir token. Sirve para comprobar que el
// servidor está vivo, para el comando "ping" y para verificar en el navegador
// que el certificado TLS es válido (PS-07).
func Salud() http.HandlerFunc {
	return func(respuesta http.ResponseWriter, peticion *http.Request) {
		EscribirJSON(respuesta, http.StatusOK, RespuestaSalud{Estado: "ok"})
	}
}
