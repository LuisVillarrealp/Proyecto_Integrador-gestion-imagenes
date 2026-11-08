package handlers

import "net/http"

// CabecerasSeguras agrega cabeceras de protección a todas las respuestas:
// el navegador no debe adivinar tipos, guardar copias en caché ni cargar
// contenido activo desde esta API.
func CabecerasSeguras(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		cabeceras := respuesta.Header()
		cabeceras.Set("X-Content-Type-Options", "nosniff")
		cabeceras.Set("Cache-Control", "no-store")
		cabeceras.Set("Referrer-Policy", "no-referrer")
		cabeceras.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		cabeceras.Set("Strict-Transport-Security", "max-age=31536000")
		siguiente.ServeHTTP(respuesta, peticion)
	})
}
