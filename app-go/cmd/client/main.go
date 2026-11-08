// Comando client: cliente de consola del sistema de gestión de imágenes.
// Se ejecuta como un proceso independiente del servidor y se comunica con él
// por la red (HTTPS).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/cliente"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

func main() {
	if err := ejecutar(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		mostrarPista(err)
		os.Exit(1)
	}
}

// mostrarPista agrega una sugerencia para los errores más comunes.
func mostrarPista(err error) {
	var rechazoDelServidor *cliente.ErrorAPI
	switch {
	case errors.As(err, &rechazoDelServidor) && rechazoDelServidor.Estado == http.StatusUnauthorized:
		fmt.Fprintln(os.Stderr, "pista: revise el token (opción -token o variable PI_TOKEN)")
	case errors.Is(err, models.ErrTamanoExcedido),
		errors.As(err, &rechazoDelServidor) && rechazoDelServidor.Estado == http.StatusRequestEntityTooLarge:
		fmt.Fprintln(os.Stderr, "pista: reduzca o comprima la imagen; el máximo permitido es 10 MB")
	}
}

func valorDeEntorno(nombreVariable, valorPorDefecto string) string {
	if valor := os.Getenv(nombreVariable); valor != "" {
		return valor
	}
	return valorPorDefecto
}

func mostrarUso(salida io.Writer, opcionesGlobales *flag.FlagSet) {
	fmt.Fprintln(salida, "Uso: cliente [opciones globales] <comando> [opciones del comando]")
	fmt.Fprintln(salida, "\nComandos:")
	fmt.Fprintln(salida, "  ping         comprueba la conexión con el servidor")
	fmt.Fprintln(salida, "  subir        sube una imagen (JPG o PNG, hasta 10 MB)")
	fmt.Fprintln(salida, "  listar       lista las imágenes a las que tiene acceso")
	fmt.Fprintln(salida, "  descargar    descarga una imagen")
	fmt.Fprintln(salida, "  actualizar   cambia nombre, campaña, etiquetas o confidencialidad")
	fmt.Fprintln(salida, "  eliminar     elimina una imagen (pide confirmación)")
	fmt.Fprintln(salida, "\nOpciones globales (van ANTES del comando):")
	opcionesGlobales.SetOutput(salida)
	opcionesGlobales.PrintDefaults()
	fmt.Fprintln(salida, "\nAyuda de un comando: cliente <comando> -h")
}

func ejecutar(argumentos []string, entrada io.Reader, salida io.Writer) error {
	opcionesGlobales := flag.NewFlagSet("cliente", flag.ContinueOnError)
	direccion := opcionesGlobales.String("addr", valorDeEntorno("PI_ADDR", "localhost:8443"), "dirección del servidor (host:puerto) [PI_ADDR]")
	token := opcionesGlobales.String("token", os.Getenv("PI_TOKEN"), "token de acceso [PI_TOKEN]")
	sede := opcionesGlobales.String("sede", os.Getenv("PI_SEDE"), "sede de origen, por ejemplo sede-a [PI_SEDE]")
	rutaCA := opcionesGlobales.String("ca", os.Getenv("PI_CA"), "certificado de la CA del proyecto (PEM) [PI_CA]")
	sinTLS := opcionesGlobales.Bool("sin-tls", false, "SOLO DESARROLLO: HTTP sin cifrar hacia loopback")
	opcionesGlobales.SetOutput(io.Discard)
	opcionesGlobales.Usage = func() {}
	if err := opcionesGlobales.Parse(argumentos); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			mostrarUso(os.Stderr, opcionesGlobales)
		}
		return err
	}
	if opcionesGlobales.NArg() == 0 {
		mostrarUso(os.Stderr, opcionesGlobales)
		return errors.New("falta el comando")
	}

	comando, argumentosDelComando := opcionesGlobales.Arg(0), opcionesGlobales.Args()[1:]
	switch comando {
	case "ping", "subir", "listar", "descargar", "actualizar", "eliminar":
	case "ayuda":
		mostrarUso(salida, opcionesGlobales)
		return nil
	default:
		return fmt.Errorf("comando desconocido %q (use «ayuda» para ver los comandos)", comando)
	}
	if comando != "ping" && *token == "" {
		return errors.New("falta el token: use la opción -token o la variable de entorno PI_TOKEN")
	}

	conexion, err := cliente.Nuevo(*direccion, *token, *sede, cliente.Opciones{CA: *rutaCA, SinTLS: *sinTLS})
	if err != nil {
		return err
	}
	switch comando {
	case "ping":
		return cmdPing(conexion, *direccion, salida)
	case "subir":
		return cmdSubir(conexion, argumentosDelComando, salida)
	case "listar":
		return cmdListar(conexion, argumentosDelComando, salida)
	case "descargar":
		return cmdDescargar(conexion, argumentosDelComando, salida)
	case "actualizar":
		return cmdActualizar(conexion, argumentosDelComando, salida)
	default:
		return cmdEliminar(conexion, argumentosDelComando, entrada, salida)
	}
}
