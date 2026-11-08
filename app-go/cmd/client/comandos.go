package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/cliente"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

func nuevasOpciones(nombreComando string) *flag.FlagSet {
	opciones := flag.NewFlagSet(nombreComando, flag.ContinueOnError)
	opciones.SetOutput(os.Stderr)
	return opciones
}

// leerOpciones lee las opciones del comando y rechaza argumentos sueltos.
func leerOpciones(opciones *flag.FlagSet, argumentos []string) error {
	if err := opciones.Parse(argumentos); err != nil {
		return err
	}
	if opciones.NArg() > 0 {
		return fmt.Errorf("argumentos inesperados: %s", strings.Join(opciones.Args(), " "))
	}
	return nil
}

func tamanoLegible(cantidadBytes int64) string {
	switch {
	case cantidadBytes >= 1<<20:
		return cliente.MegabytesLegibles(cantidadBytes)
	case cantidadBytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(cantidadBytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", cantidadBytes)
	}
}

func mostrarImagen(salida io.Writer, imagen models.Imagen) {
	fmt.Fprintf(salida, "  ID:               %s\n", imagen.ID())
	fmt.Fprintf(salida, "  Nombre:           %s\n", imagen.Nombre())
	fmt.Fprintf(salida, "  Campaña:          %s\n", imagen.Campana())
	fmt.Fprintf(salida, "  Etiquetas:        %s\n", strings.Join(imagen.Etiquetas(), ", "))
	fmt.Fprintf(salida, "  Confidencialidad: %s\n", imagen.Confidencialidad())
	fmt.Fprintf(salida, "  Formato y tamaño: %s, %s\n", imagen.Formato(), tamanoLegible(imagen.Tamano()))
}

func cmdPing(conexion *cliente.Cliente, direccion string, salida io.Writer) error {
	salud, err := conexion.Ping()
	if err != nil {
		return err
	}
	fmt.Fprintf(salida, "Servidor %s en %s (conexión verificada)\n", salud.Estado, direccion)
	return nil
}

func cmdSubir(conexion *cliente.Cliente, argumentos []string, salida io.Writer) error {
	opciones := nuevasOpciones("subir")
	rutaArchivo := opciones.String("archivo", "", "ruta de la imagen JPG o PNG [obligatorio]")
	nombre := opciones.String("nombre", "", "nombre de la imagen (por defecto, el del archivo sin extensión)")
	campana := opciones.String("campana", "", "campaña a la que pertenece [obligatorio]")
	etiquetas := opciones.String("etiquetas", "", "etiquetas separadas por coma")
	nivel := opciones.String("confidencialidad", "", "publica, interna o confidencial [obligatorio]")
	if err := leerOpciones(opciones, argumentos); err != nil {
		return err
	}
	if *rutaArchivo == "" {
		return errors.New("falta -archivo con la ruta de la imagen")
	}
	nombreImagen := *nombre
	if nombreImagen == "" {
		nombreDelArchivo := filepath.Base(*rutaArchivo)
		nombreImagen = strings.TrimSuffix(nombreDelArchivo, filepath.Ext(nombreDelArchivo))
	}

	imagen, err := conexion.Subir(*rutaArchivo, models.MetaImagen{
		Nombre:           nombreImagen,
		Campana:          *campana,
		Etiquetas:        strings.Split(*etiquetas, ","),
		Confidencialidad: models.Confidencialidad(*nivel),
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(salida, "Imagen subida correctamente")
	mostrarImagen(salida, imagen)
	return nil
}

func cmdListar(conexion *cliente.Cliente, argumentos []string, salida io.Writer) error {
	opciones := nuevasOpciones("listar")
	var consulta cliente.Consulta
	opciones.StringVar(&consulta.Campana, "campana", "", "solo de esta campaña")
	opciones.StringVar(&consulta.Etiqueta, "etiqueta", "", "solo con esta etiqueta")
	opciones.StringVar(&consulta.Confidencialidad, "confidencialidad", "", "solo de este nivel (publica, interna o confidencial)")
	opciones.StringVar(&consulta.Desde, "desde", "", "solo desde esta fecha (AAAA-MM-DD)")
	if err := leerOpciones(opciones, argumentos); err != nil {
		return err
	}

	imagenes, err := conexion.Listar(consulta)
	if err != nil {
		return err
	}
	if len(imagenes) == 0 {
		fmt.Fprintln(salida, "No hay imágenes que coincidan.")
		return nil
	}
	tabla := tabwriter.NewWriter(salida, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tabla, "ID\tNOMBRE\tCAMPAÑA\tNIVEL\tFORMATO\tTAMAÑO\tAUTOR\tFECHA")
	for _, imagen := range imagenes {
		fmt.Fprintf(tabla, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			imagen.ID(), imagen.Nombre(), imagen.Campana(), imagen.Confidencialidad(), imagen.Formato(),
			tamanoLegible(imagen.Tamano()), imagen.Autor(), imagen.Fecha().Local().Format("2006-01-02 15:04"))
	}
	if err := tabla.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(salida, "\n%d imagen(es)\n", len(imagenes))
	return nil
}

func cmdDescargar(conexion *cliente.Cliente, argumentos []string, salida io.Writer) error {
	opciones := nuevasOpciones("descargar")
	id := opciones.String("id", "", "ID de la imagen [obligatorio]")
	rutaSalida := opciones.String("salida", "", "archivo de destino (por defecto, el nombre original en el directorio actual)")
	forzar := opciones.Bool("forzar", false, "sobrescribir si el archivo de destino ya existe")
	if err := leerOpciones(opciones, argumentos); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("falta -id con el identificador de la imagen")
	}

	descarga, err := conexion.Descargar(*id)
	if err != nil {
		return err
	}
	defer descarga.Cuerpo.Close()

	rutaDestino := *rutaSalida
	if rutaDestino == "" {
		rutaDestino = descarga.Nombre
	}
	modoApertura := os.O_WRONLY | os.O_CREATE | os.O_EXCL // no pisar archivos existentes
	if *forzar {
		modoApertura = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	archivoDestino, err := os.OpenFile(rutaDestino, modoApertura, 0o640)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%s ya existe (use -forzar para sobrescribirlo o -salida para otro nombre)", rutaDestino)
	}
	if err != nil {
		return fmt.Errorf("crear %s: %w", rutaDestino, err)
	}

	bytesRecibidos, err := io.Copy(archivoDestino, descarga.Cuerpo)
	if errorAlCerrar := archivoDestino.Close(); err == nil {
		err = errorAlCerrar
	}
	if err == nil && descarga.Tamano >= 0 && bytesRecibidos != descarga.Tamano {
		err = fmt.Errorf("descarga incompleta: %d de %d bytes", bytesRecibidos, descarga.Tamano)
	}
	if err != nil {
		os.Remove(rutaDestino) // no dejar un archivo a medias
		return fmt.Errorf("guardar %s: %w", rutaDestino, err)
	}
	fmt.Fprintf(salida, "Imagen descargada en %s (%s)\n", rutaDestino, tamanoLegible(bytesRecibidos))
	return nil
}

func cmdActualizar(conexion *cliente.Cliente, argumentos []string, salida io.Writer) error {
	opciones := nuevasOpciones("actualizar")
	id := opciones.String("id", "", "ID de la imagen [obligatorio]")
	nombre := opciones.String("nombre", "", "nuevo nombre")
	campana := opciones.String("campana", "", "nueva campaña")
	etiquetas := opciones.String("etiquetas", "", "nuevas etiquetas separadas por coma (reemplazan a las actuales)")
	nivel := opciones.String("confidencialidad", "", "nuevo nivel: publica, interna o confidencial")
	if err := leerOpciones(opciones, argumentos); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("falta -id con el identificador de la imagen")
	}
	camposIndicados := map[string]bool{}
	opciones.Visit(func(opcion *flag.Flag) { camposIndicados[opcion.Name] = true })
	if !camposIndicados["nombre"] && !camposIndicados["campana"] && !camposIndicados["etiquetas"] && !camposIndicados["confidencialidad"] {
		return errors.New("indique al menos un campo a cambiar: -nombre, -campana, -etiquetas o -confidencialidad")
	}

	// Se parte de los metadatos actuales y solo se cambia lo indicado.
	imagenActual, err := conexion.Buscar(*id)
	if err != nil {
		return err
	}
	nuevaMeta := imagenActual.Meta()
	if camposIndicados["nombre"] {
		nuevaMeta.Nombre = *nombre
	}
	if camposIndicados["campana"] {
		nuevaMeta.Campana = *campana
	}
	if camposIndicados["etiquetas"] {
		nuevaMeta.Etiquetas = strings.Split(*etiquetas, ",")
	}
	if camposIndicados["confidencialidad"] {
		nuevaMeta.Confidencialidad = models.Confidencialidad(*nivel)
	}

	imagenActualizada, err := conexion.Actualizar(*id, nuevaMeta)
	if err != nil {
		return err
	}
	fmt.Fprintln(salida, "Imagen actualizada")
	mostrarImagen(salida, imagenActualizada)
	return nil
}

func cmdEliminar(conexion *cliente.Cliente, argumentos []string, entrada io.Reader, salida io.Writer) error {
	opciones := nuevasOpciones("eliminar")
	id := opciones.String("id", "", "ID de la imagen [obligatorio]")
	sinConfirmar := opciones.Bool("si", false, "no pedir confirmación")
	if err := leerOpciones(opciones, argumentos); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("falta -id con el identificador de la imagen")
	}

	if !*sinConfirmar {
		fmt.Fprintf(salida, "¿Eliminar la imagen %s? Esta acción no se puede deshacer [s/N]: ", *id)
		lineaLeida, _ := bufio.NewReader(entrada).ReadString('\n')
		confirmacion := strings.ToLower(strings.TrimSpace(lineaLeida))
		if confirmacion != "s" && confirmacion != "si" && confirmacion != "sí" {
			fmt.Fprintln(salida, "Operación cancelada.")
			return nil
		}
	}
	if err := conexion.Eliminar(*id); err != nil {
		return err
	}
	fmt.Fprintf(salida, "Imagen %s eliminada\n", *id)
	return nil
}
