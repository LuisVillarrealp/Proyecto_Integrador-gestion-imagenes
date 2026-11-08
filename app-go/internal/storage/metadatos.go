package storage

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

// Metadatos guarda los metadatos de las imágenes en memoria y los persiste en
// un archivo JSON. Es seguro para uso concurrente (varios clientes a la vez).
//
// Limitación conocida: reescribe el JSON completo en cada cambio. Es suficiente
// para el prototipo; una base de datos reemplazaría esto en fases posteriores
// sin cambiar los métodos públicos.
type Metadatos struct {
	candado       sync.RWMutex
	rutaArchivo   string
	imagenesPorID map[string]models.Imagen
}

// NuevoMetadatos carga el archivo JSON si existe, o parte vacío si no.
// Cada imagen leída se valida (ver Imagen.UnmarshalJSON): un archivo alterado
// a mano no puede introducir datos inválidos.
func NuevoMetadatos(rutaArchivo string) (*Metadatos, error) {
	if err := os.MkdirAll(filepath.Dir(rutaArchivo), 0o750); err != nil {
		return nil, fmt.Errorf("crear directorio de metadatos: %w", err)
	}
	metadatos := &Metadatos{rutaArchivo: rutaArchivo, imagenesPorID: make(map[string]models.Imagen)}

	contenido, err := os.ReadFile(rutaArchivo)
	if errors.Is(err, os.ErrNotExist) {
		return metadatos, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer metadatos: %w", err)
	}
	var imagenesLeidas []models.Imagen
	if err := json.Unmarshal(contenido, &imagenesLeidas); err != nil {
		return nil, fmt.Errorf("metadatos corruptos en %s: %w", rutaArchivo, err)
	}
	for _, imagen := range imagenesLeidas {
		metadatos.imagenesPorID[imagen.ID()] = imagen
	}
	return metadatos, nil
}

// persistir escribe el JSON de forma atómica (archivo temporal + renombrar).
// Debe llamarse con el candado de escritura tomado.
func (metadatos *Metadatos) persistir() error {
	imagenesOrdenadas := make([]models.Imagen, 0, len(metadatos.imagenesPorID))
	for _, imagen := range metadatos.imagenesPorID {
		imagenesOrdenadas = append(imagenesOrdenadas, imagen)
	}
	slices.SortFunc(imagenesOrdenadas, func(primera, segunda models.Imagen) int {
		return cmp.Compare(primera.ID(), segunda.ID())
	})

	contenido, err := json.MarshalIndent(imagenesOrdenadas, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar metadatos: %w", err)
	}
	rutaTemporal := metadatos.rutaArchivo + ".tmp"
	if err := os.WriteFile(rutaTemporal, contenido, 0o640); err != nil {
		return fmt.Errorf("escribir metadatos: %w", err)
	}
	if err := os.Rename(rutaTemporal, metadatos.rutaArchivo); err != nil {
		return fmt.Errorf("guardar metadatos: %w", err)
	}
	return nil
}

// Crear agrega una imagen nueva. Falla si el ID ya existe.
func (metadatos *Metadatos) Crear(imagen models.Imagen) error {
	metadatos.candado.Lock()
	defer metadatos.candado.Unlock()
	if _, existe := metadatos.imagenesPorID[imagen.ID()]; existe {
		return fmt.Errorf("%w: ya existe una imagen con id %s", models.ErrDatosInvalidos, imagen.ID())
	}
	metadatos.imagenesPorID[imagen.ID()] = imagen
	if err := metadatos.persistir(); err != nil {
		delete(metadatos.imagenesPorID, imagen.ID()) // deshace el cambio en memoria
		return err
	}
	return nil
}

// Obtener devuelve la imagen con ese ID o ErrNoEncontrado.
func (metadatos *Metadatos) Obtener(id string) (models.Imagen, error) {
	metadatos.candado.RLock()
	defer metadatos.candado.RUnlock()
	imagen, existe := metadatos.imagenesPorID[id]
	if !existe {
		return models.Imagen{}, models.ErrNoEncontrado
	}
	return imagen, nil
}

// Listar devuelve las imágenes que cumplen el filtro, de la más reciente a la
// más antigua. Si no hay coincidencias devuelve una lista vacía (no nil).
func (metadatos *Metadatos) Listar(filtro models.Filtro) []models.Imagen {
	metadatos.candado.RLock()
	defer metadatos.candado.RUnlock()
	coincidencias := make([]models.Imagen, 0, len(metadatos.imagenesPorID))
	for _, imagen := range metadatos.imagenesPorID {
		if filtro.Coincide(imagen) {
			coincidencias = append(coincidencias, imagen)
		}
	}
	slices.SortFunc(coincidencias, func(primera, segunda models.Imagen) int {
		if porFecha := segunda.Fecha().Compare(primera.Fecha()); porFecha != 0 {
			return porFecha
		}
		return cmp.Compare(primera.ID(), segunda.ID())
	})
	return coincidencias
}

// Actualizar reemplaza los metadatos de una imagen existente.
func (metadatos *Metadatos) Actualizar(imagen models.Imagen) error {
	metadatos.candado.Lock()
	defer metadatos.candado.Unlock()
	imagenAnterior, existe := metadatos.imagenesPorID[imagen.ID()]
	if !existe {
		return models.ErrNoEncontrado
	}
	metadatos.imagenesPorID[imagen.ID()] = imagen
	if err := metadatos.persistir(); err != nil {
		metadatos.imagenesPorID[imagen.ID()] = imagenAnterior
		return err
	}
	return nil
}

// Eliminar quita los metadatos de una imagen.
func (metadatos *Metadatos) Eliminar(id string) error {
	metadatos.candado.Lock()
	defer metadatos.candado.Unlock()
	imagenAnterior, existe := metadatos.imagenesPorID[id]
	if !existe {
		return models.ErrNoEncontrado
	}
	delete(metadatos.imagenesPorID, id)
	if err := metadatos.persistir(); err != nil {
		metadatos.imagenesPorID[id] = imagenAnterior
		return err
	}
	return nil
}

// Total devuelve la cantidad de imágenes registradas.
func (metadatos *Metadatos) Total() int {
	metadatos.candado.RLock()
	defer metadatos.candado.RUnlock()
	return len(metadatos.imagenesPorID)
}
