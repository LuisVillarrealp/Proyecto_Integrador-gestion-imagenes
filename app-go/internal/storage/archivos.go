// Package storage guarda por separado los archivos de imagen (en disco) y sus
// metadatos (en un JSON), como pide el enunciado del entregable.
package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

// Archivos gestiona los archivos de imagen dentro de un directorio.
type Archivos struct {
	directorio string
}

// NuevoArchivos crea el directorio si no existe y devuelve el gestor.
func NuevoArchivos(directorio string) (*Archivos, error) {
	if err := os.MkdirAll(directorio, 0o750); err != nil {
		return nil, fmt.Errorf("crear directorio de imágenes %s: %w", directorio, err)
	}
	return &Archivos{directorio: directorio}, nil
}

// rutaSegura devuelve la ruta completa de un archivo, rechazando cualquier
// nombre que intente salir del directorio (por ejemplo "../../etc/passwd").
func (archivos *Archivos) rutaSegura(nombreArchivo string) (string, error) {
	if nombreArchivo == "" || nombreArchivo == "." || nombreArchivo == ".." || nombreArchivo != filepath.Base(nombreArchivo) {
		return "", fmt.Errorf("%w: nombre de archivo no válido", models.ErrDatosInvalidos)
	}
	return filepath.Join(archivos.directorio, nombreArchivo), nil
}

// Guardar escribe el contenido en un archivo temporal y lo renombra al final,
// de modo que nunca queda un archivo a medio escribir. Devuelve los bytes guardados.
func (archivos *Archivos) Guardar(nombreArchivo string, contenido io.Reader) (int64, error) {
	rutaDestino, err := archivos.rutaSegura(nombreArchivo)
	if err != nil {
		return 0, err
	}
	archivoTemporal, err := os.CreateTemp(archivos.directorio, "subida-*.tmp")
	if err != nil {
		return 0, fmt.Errorf("crear archivo temporal: %w", err)
	}
	defer os.Remove(archivoTemporal.Name()) // si ya se renombró, falla sin consecuencias

	bytesEscritos, err := io.Copy(archivoTemporal, contenido)
	if errorAlCerrar := archivoTemporal.Close(); err == nil {
		err = errorAlCerrar
	}
	if err != nil {
		return 0, fmt.Errorf("escribir archivo: %w", err)
	}
	if err := os.Rename(archivoTemporal.Name(), rutaDestino); err != nil {
		return 0, fmt.Errorf("guardar archivo: %w", err)
	}
	return bytesEscritos, nil
}

// Abrir abre un archivo para lectura. Quien lo llama debe cerrarlo.
func (archivos *Archivos) Abrir(nombreArchivo string) (*os.File, error) {
	ruta, err := archivos.rutaSegura(nombreArchivo)
	if err != nil {
		return nil, err
	}
	archivoAbierto, err := os.Open(ruta)
	if errors.Is(err, os.ErrNotExist) {
		return nil, models.ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("abrir archivo: %w", err)
	}
	return archivoAbierto, nil
}

// Eliminar borra el archivo del disco.
func (archivos *Archivos) Eliminar(nombreArchivo string) error {
	ruta, err := archivos.rutaSegura(nombreArchivo)
	if err != nil {
		return err
	}
	err = os.Remove(ruta)
	if errors.Is(err, os.ErrNotExist) {
		return models.ErrNoEncontrado
	}
	if err != nil {
		return fmt.Errorf("eliminar archivo: %w", err)
	}
	return nil
}
