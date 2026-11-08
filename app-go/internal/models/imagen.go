package models

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// TamanoMaximo es el tamaño máximo de una imagen (RNF-SW-01): 10 MB.
const TamanoMaximo int64 = 10 << 20

// Formatos de imagen permitidos (RF-SW-02).
const (
	FormatoPNG = "png"
	FormatoJPG = "jpg"
)

// Confidencialidad es el nivel de acceso de una imagen.
type Confidencialidad string

const (
	Publica      Confidencialidad = "publica"
	Interna      Confidencialidad = "interna"
	Confidencial Confidencialidad = "confidencial"
)

// ParseConfidencialidad convierte un texto del usuario en un nivel válido.
func ParseConfidencialidad(textoUsuario string) (Confidencialidad, error) {
	nivel := Confidencialidad(strings.ToLower(strings.TrimSpace(textoUsuario)))
	switch nivel {
	case Publica, Interna, Confidencial:
		return nivel, nil
	}
	return "", fmt.Errorf("%w: confidencialidad %q no válida (use publica, interna o confidencial)", ErrDatosInvalidos, textoUsuario)
}

// MetaImagen son los metadatos que el usuario envía y puede modificar.
// Sus campos son exportados porque es un objeto de transferencia: viaja del
// cliente al servidor y siempre se valida con Normalizar antes de usarse.
type MetaImagen struct {
	Nombre           string           `json:"nombre"`
	Campana          string           `json:"campana"`
	Etiquetas        []string         `json:"etiquetas"`
	Confidencialidad Confidencialidad `json:"confidencialidad"`
}

// Normalizar valida los metadatos y devuelve una copia limpia:
// sin espacios sobrantes, etiquetas en minúscula y sin repetidas.
func (meta MetaImagen) Normalizar() (MetaImagen, error) {
	meta.Nombre = strings.TrimSpace(meta.Nombre)
	meta.Campana = strings.TrimSpace(meta.Campana)
	if meta.Nombre == "" || meta.Campana == "" {
		return MetaImagen{}, fmt.Errorf("%w: nombre y campaña son obligatorios", ErrDatosInvalidos)
	}
	nivel, err := ParseConfidencialidad(string(meta.Confidencialidad))
	if err != nil {
		return MetaImagen{}, err
	}
	meta.Confidencialidad = nivel
	meta.Etiquetas = limpiarEtiquetas(meta.Etiquetas)
	return meta, nil
}

func limpiarEtiquetas(etiquetasRecibidas []string) []string {
	etiquetasLimpias := make([]string, 0, len(etiquetasRecibidas))
	for _, etiqueta := range etiquetasRecibidas {
		etiqueta = strings.ToLower(strings.TrimSpace(etiqueta))
		if etiqueta != "" && !slices.Contains(etiquetasLimpias, etiqueta) {
			etiquetasLimpias = append(etiquetasLimpias, etiqueta)
		}
	}
	return etiquetasLimpias
}

// Imagen son los metadatos completos de una imagen almacenada. El archivo en
// sí vive aparte (paquete storage), identificado por NombreArchivo.
//
// Encapsulamiento: los campos son privados. Una Imagen solo se crea con
// NuevaImagen (o al leerse desde JSON, que también valida) y solo cambia con
// CambiarMeta, así que nunca queda en un estado inválido.
// Composición: contiene una MetaImagen en lugar de repetir sus campos.
type Imagen struct {
	id      string
	meta    MetaImagen
	autor   string
	formato string    // "jpg" o "png"
	tamano  int64     // bytes
	fecha   time.Time // momento de la subida, en UTC
}

// NuevaImagen crea una Imagen con estado válido o devuelve el error de dominio
// que corresponda (formato, tamaño o datos inválidos).
func NuevaImagen(id string, meta MetaImagen, autor, formato string, tamano int64, fecha time.Time) (Imagen, error) {
	id = strings.TrimSpace(id)
	autor = strings.TrimSpace(autor)
	if id == "" || autor == "" {
		return Imagen{}, fmt.Errorf("%w: id y autor son obligatorios", ErrDatosInvalidos)
	}
	formato = strings.ToLower(strings.TrimSpace(formato))
	if formato != FormatoJPG && formato != FormatoPNG {
		return Imagen{}, ErrFormatoInvalido
	}
	if tamano <= 0 {
		return Imagen{}, fmt.Errorf("%w: el archivo está vacío", ErrDatosInvalidos)
	}
	if tamano > TamanoMaximo {
		return Imagen{}, ErrTamanoExcedido
	}
	metaValidada, err := meta.Normalizar()
	if err != nil {
		return Imagen{}, err
	}
	return Imagen{
		id:      id,
		meta:    metaValidada,
		autor:   autor,
		formato: formato,
		tamano:  tamano,
		fecha:   fecha.UTC(),
	}, nil
}

// Métodos de acceso (solo lectura).

func (imagen Imagen) ID() string                         { return imagen.id }
func (imagen Imagen) Nombre() string                     { return imagen.meta.Nombre }
func (imagen Imagen) Campana() string                    { return imagen.meta.Campana }
func (imagen Imagen) Confidencialidad() Confidencialidad { return imagen.meta.Confidencialidad }
func (imagen Imagen) Autor() string                      { return imagen.autor }
func (imagen Imagen) Formato() string                    { return imagen.formato }
func (imagen Imagen) Tamano() int64                      { return imagen.tamano }
func (imagen Imagen) Fecha() time.Time                   { return imagen.fecha }

// Etiquetas devuelve una copia, para que quien la reciba no pueda modificar
// las etiquetas guardadas dentro de la imagen.
func (imagen Imagen) Etiquetas() []string { return slices.Clone(imagen.meta.Etiquetas) }

// Meta devuelve una copia de los metadatos editables.
func (imagen Imagen) Meta() MetaImagen {
	copia := imagen.meta
	copia.Etiquetas = slices.Clone(imagen.meta.Etiquetas)
	return copia
}

// TieneEtiqueta indica si la imagen tiene esa etiqueta (sin distinguir mayúsculas).
func (imagen Imagen) TieneEtiqueta(etiqueta string) bool {
	return slices.Contains(imagen.meta.Etiquetas, strings.ToLower(strings.TrimSpace(etiqueta)))
}

// NombreArchivo es el nombre con el que se guarda en disco. Se deriva del ID
// y del formato validado, nunca del nombre que envió el usuario.
func (imagen Imagen) NombreArchivo() string { return imagen.id + "." + imagen.formato }

// CambiarMeta reemplaza los metadatos editables (RF-SW-04). Usa receptor por
// puntero porque modifica la imagen; si los datos nuevos no son válidos, la
// imagen queda como estaba.
func (imagen *Imagen) CambiarMeta(nuevaMeta MetaImagen) error {
	metaValidada, err := nuevaMeta.Normalizar()
	if err != nil {
		return err
	}
	imagen.meta = metaValidada
	return nil
}

// imagenJSON es la forma de la Imagen en JSON (respuestas de la API y archivo
// de metadatos). Existe solo para convertir: el resto del código usa Imagen.
type imagenJSON struct {
	ID               string           `json:"id"`
	Nombre           string           `json:"nombre"`
	Campana          string           `json:"campana"`
	Etiquetas        []string         `json:"etiquetas"`
	Confidencialidad Confidencialidad `json:"confidencialidad"`
	Autor            string           `json:"autor"`
	Formato          string           `json:"formato"`
	Tamano           int64            `json:"tamano"`
	Fecha            time.Time        `json:"fecha"`
}

// MarshalJSON convierte la imagen a JSON a pesar de tener campos privados.
func (imagen Imagen) MarshalJSON() ([]byte, error) {
	return json.Marshal(imagenJSON{
		ID:               imagen.id,
		Nombre:           imagen.meta.Nombre,
		Campana:          imagen.meta.Campana,
		Etiquetas:        imagen.meta.Etiquetas,
		Confidencialidad: imagen.meta.Confidencialidad,
		Autor:            imagen.autor,
		Formato:          imagen.formato,
		Tamano:           imagen.tamano,
		Fecha:            imagen.fecha,
	})
}

// UnmarshalJSON lee una imagen desde JSON pasando por NuevaImagen: un archivo
// de metadatos alterado o una respuesta inválida nunca crean una Imagen inválida.
func (imagen *Imagen) UnmarshalJSON(datos []byte) error {
	var leida imagenJSON
	if err := json.Unmarshal(datos, &leida); err != nil {
		return err
	}
	meta := MetaImagen{
		Nombre:           leida.Nombre,
		Campana:          leida.Campana,
		Etiquetas:        leida.Etiquetas,
		Confidencialidad: leida.Confidencialidad,
	}
	validada, err := NuevaImagen(leida.ID, meta, leida.Autor, leida.Formato, leida.Tamano, leida.Fecha)
	if err != nil {
		return fmt.Errorf("imagen %q inválida: %w", leida.ID, err)
	}
	*imagen = validada
	return nil
}

// Filtro restringe un listado. Los campos vacíos no filtran.
type Filtro struct {
	Campana          string
	Etiqueta         string
	Confidencialidad Confidencialidad
	Desde            time.Time
}

// Coincide indica si la imagen cumple todos los criterios del filtro.
func (filtro Filtro) Coincide(imagen Imagen) bool {
	if filtro.Campana != "" && !strings.EqualFold(strings.TrimSpace(filtro.Campana), imagen.Campana()) {
		return false
	}
	if filtro.Etiqueta != "" && !imagen.TieneEtiqueta(filtro.Etiqueta) {
		return false
	}
	if filtro.Confidencialidad != "" && filtro.Confidencialidad != imagen.Confidencialidad() {
		return false
	}
	if !filtro.Desde.IsZero() && imagen.Fecha().Before(filtro.Desde) {
		return false
	}
	return true
}
