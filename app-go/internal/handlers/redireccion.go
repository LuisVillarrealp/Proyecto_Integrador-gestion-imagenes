package handlers

import "net/http"

// RedirigirAHTTPS atiende el puerto HTTP (RF-SEC-07, PS-07): una petición GET
// o HEAD se redirige al mismo recurso por HTTPS. Cualquier otro método se
// rechaza SIN procesar: redirigir un POST ya enviado habría expuesto el
// cuerpo y el token en claro. El destino sale de la configuración
// (hostPublico), nunca de la cabecera Host, para evitar redirecciones abiertas.
func RedirigirAHTTPS(hostPublico string) http.Handler {
	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.Method != http.MethodGet && peticion.Method != http.MethodHead {
			EscribirJSON(respuesta, http.StatusUpgradeRequired, map[string]string{"error": "use HTTPS"})
			return
		}
		http.Redirect(respuesta, peticion, "https://"+hostPublico+peticion.URL.RequestURI(), http.StatusPermanentRedirect)
	})
}
