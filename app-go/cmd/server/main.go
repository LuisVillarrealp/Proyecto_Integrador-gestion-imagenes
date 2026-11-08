// Comando server: servidor del sistema de gestión de imágenes. Atiende por
// HTTPS las peticiones de los clientes (procesos independientes).
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/auth"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/handlers"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/red"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/services"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/storage"
)

func main() {
	if err := iniciar(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func iniciar() error {
	direccion := flag.String("addr", ":8443", "dirección HTTPS donde escucha el servidor (host:puerto)")
	dirDatos := flag.String("data", "data", "directorio de datos (imágenes y metadatos)")
	rutaCertificado := flag.String("cert", "", "certificado TLS del servidor (PEM)")
	rutaClave := flag.String("key", "", "clave privada del certificado (PEM)")
	direccionRedireccion := flag.String("redirect", "", "si se indica (ej. :8080), escucha HTTP solo para redirigir GET/HEAD a HTTPS")
	hostPublico := flag.String("host-publico", "", "host:puerto HTTPS al que redirige -redirect (ej. 10.10.0.50:8443)")
	sinTLS := flag.Bool("sin-tls", false, "SOLO DESARROLLO: HTTP sin cifrar; únicamente en loopback")
	authDev := flag.Bool("auth-dev", false, "SOLO DESARROLLO: tokens de prueba; únicamente en loopback")
	usuarios := flag.String("usuarios", "data/usuarios_dev.json", "tokens de prueba (con -auth-dev)")
	flag.Parse()

	// Configuración insegura: solo se tolera si el servidor no sale de esta máquina.
	if *sinTLS && !red.EsLoopback(*direccion) {
		return errors.New("-sin-tls solo se permite escuchando en loopback (ej. 127.0.0.1:8443)")
	}
	if !*sinTLS && (*rutaCertificado == "" || *rutaClave == "") {
		return errors.New("falta -cert y -key: el tráfico debe ir cifrado con TLS (RF-SEC-07)")
	}
	if !*authDev {
		return errors.New("autenticación JWT aún no disponible: para pruebas use -auth-dev en loopback")
	}
	if !red.EsLoopback(*direccion) {
		return errors.New("-auth-dev usa tokens públicos de prueba: solo se permite en loopback")
	}
	if *direccionRedireccion != "" && *hostPublico == "" {
		return errors.New("-redirect requiere -host-publico")
	}

	// Si algo falta o está mal, el servidor no inicia y lo dice.
	archivos, err := storage.NuevoArchivos(filepath.Join(*dirDatos, "imagenes"))
	if err != nil {
		return err
	}
	metadatos, err := storage.NuevoMetadatos(filepath.Join(*dirDatos, "metadatos.json"))
	if err != nil {
		return err
	}
	actividad, err := storage.NuevaActividad(filepath.Join(*dirDatos, "actividad.jsonl"))
	if err != nil {
		return err
	}
	defer actividad.Cerrar()
	autenticador, err := auth.NuevoDev(*usuarios)
	if err != nil {
		return err
	}

	servicio := services.NuevoImagenes(metadatos, archivos)
	api := handlers.NuevaAPI(servicio, autenticador, actividad)

	enrutador := http.NewServeMux()
	enrutador.Handle("GET /salud", handlers.Salud())
	api.Rutas(enrutador)

	nuevoServidor := func(direccionEscucha string, manejador http.Handler) *http.Server {
		return &http.Server{
			Addr:              direccionEscucha,
			Handler:           manejador,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       60 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
	}
	principal := nuevoServidor(*direccion, handlers.CabecerasSeguras(enrutador))
	servidores := []*http.Server{principal}
	if !*sinTLS {
		principal.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if *direccionRedireccion != "" {
		servidores = append(servidores, nuevoServidor(*direccionRedireccion, handlers.RedirigirAHTTPS(*hostPublico)))
	}

	senalDeCierre, detenerEscucha := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer detenerEscucha()

	erroresDeServidores := make(chan error, len(servidores))
	for _, servidor := range servidores {
		go func(servidor *http.Server) {
			if servidor == principal && !*sinTLS {
				log.Printf("servidor HTTPS escuchando en %s (datos en %s)", servidor.Addr, *dirDatos)
				erroresDeServidores <- servidor.ListenAndServeTLS(*rutaCertificado, *rutaClave)
				return
			}
			if servidor == principal {
				log.Printf("ATENCIÓN: HTTP sin cifrar en %s (solo desarrollo)", servidor.Addr)
			} else {
				log.Printf("redirección HTTP→HTTPS en %s", servidor.Addr)
			}
			erroresDeServidores <- servidor.ListenAndServe()
		}(servidor)
	}

	select {
	case err := <-erroresDeServidores:
		return err
	case <-senalDeCierre.Done():
	}

	log.Println("cerrando servidor...")
	plazoDeCierre, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	var erroresAlCerrar []error
	for _, servidor := range servidores {
		erroresAlCerrar = append(erroresAlCerrar, servidor.Shutdown(plazoDeCierre))
	}
	return errors.Join(erroresAlCerrar...)
}
