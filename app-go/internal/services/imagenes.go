package services

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registra el decodificador JPEG para image.DecodeConfig
	_ "image/png"  // registra el decodificador PNG para image.DecodeConfig
	"io"
	"net/http"
	"time"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/storage"
)

// Imagenes implementa las operaciones del sistema sobre las imágenes.
// Por composición usa dos almacenes distintos: uno para los metadatos y otro
// para los archivos (el enunciado pide gestionarlos por separado).
type Imagenes struct {
	metadatos *storage.Metadatos
	archivos  *storage.Archivos
	reloj     func() time.Time // se reemplaza en las pruebas para fijar la hora
}

// NuevoImagenes crea el servicio con su almacenamiento.
func NuevoImagenes(metadatos *storage.Metadatos, archivos *storage.Archivos) *Imagenes {
	return &Imagenes{metadatos: metadatos, archivos: archivos, reloj: time.Now}
}

// puedeVer aplica RF-SEC-03 de la Fase 2: el Administrador ve todo y un
// Empleado solo ve imágenes de las campañas que tiene asignadas, sea cual sea
// su nivel de confidencialidad. Todo lo demás se niega por defecto.
func puedeVer(usuario models.Usuario, imagen models.Imagen) bool {
	return usuario.EsAdministrador() || usuario.TieneCampana(imagen.Campana())
}

// puedeModificar: el Administrador, o el autor mientras la campaña siga
// asignada a él.
func puedeModificar(usuario models.Usuario, imagen models.Imagen) bool {
	esAutor := imagen.Autor() == usuario.ID()
	return usuario.EsAdministrador() || (esAutor && usuario.TieneCampana(imagen.Campana()))
}

// exigirCampana impide que un Empleado suba o mueva imágenes a una campaña
// que no tiene asignada.
func exigirCampana(usuario models.Usuario, campana string) error {
	if usuario.EsAdministrador() || usuario.TieneCampana(campana) {
		return nil
	}
	return fmt.Errorf("%w: no tiene asignada la campaña %q", models.ErrNoAutorizado, campana)
}

// detectarFormato verifica el formato por el CONTENIDO del archivo, no por su
// nombre ni extensión: primero la firma de bytes y luego la cabecera de imagen.
// Devuelve "png" o "jpg".
func detectarFormato(contenido []byte) (string, error) {
	var formatoPorFirma string
	switch http.DetectContentType(contenido) { // mira solo los primeros 512 bytes
	case "image/png":
		formatoPorFirma = models.FormatoPNG
	case "image/jpeg":
		formatoPorFirma = models.FormatoJPG
	default:
		return "", models.ErrFormatoInvalido
	}

	_, formatoDecodificado, err := image.DecodeConfig(bytes.NewReader(contenido))
	if err != nil {
		return "", fmt.Errorf("%w: el contenido no es una imagen válida", models.ErrFormatoInvalido)
	}
	if formatoDecodificado == "jpeg" {
		formatoDecodificado = models.FormatoJPG
	}
	if formatoDecodificado != formatoPorFirma {
		return "", fmt.Errorf("%w: el contenido no coincide con su formato", models.ErrFormatoInvalido)
	}
	return formatoPorFirma, nil
}

// Subir valida y guarda una imagen. Si cualquier validación falla no se
// guarda nada (RF-SW-02). El archivo y sus metadatos se almacenan por separado.
func (servicio *Imagenes) Subir(usuario models.Usuario, meta models.MetaImagen, datosArchivo io.Reader) (models.Imagen, error) {
	metaValidada, err := meta.Normalizar()
	if err != nil {
		return models.Imagen{}, err
	}
	if err := exigirCampana(usuario, metaValidada.Campana); err != nil {
		return models.Imagen{}, err
	}

	// Se lee un byte de más para distinguir "exactamente 10 MB" de "más de 10 MB".
	contenido, err := io.ReadAll(io.LimitReader(datosArchivo, models.TamanoMaximo+1))
	if err != nil {
		return models.Imagen{}, fmt.Errorf("leer archivo: %w", err)
	}
	tamanoArchivo := int64(len(contenido))
	if tamanoArchivo > models.TamanoMaximo {
		return models.Imagen{}, models.ErrTamanoExcedido
	}
	if tamanoArchivo == 0 {
		return models.Imagen{}, fmt.Errorf("%w: el archivo está vacío", models.ErrDatosInvalidos)
	}

	formato, err := detectarFormato(contenido)
	if err != nil {
		return models.Imagen{}, err
	}
	idNuevo, err := models.NuevoID()
	if err != nil {
		return models.Imagen{}, fmt.Errorf("generar id: %w", err)
	}
	imagen, err := models.NuevaImagen(idNuevo, metaValidada, usuario.ID(), formato, tamanoArchivo, servicio.reloj())
	if err != nil {
		return models.Imagen{}, err
	}

	if _, err := servicio.archivos.Guardar(imagen.NombreArchivo(), bytes.NewReader(contenido)); err != nil {
		return models.Imagen{}, fmt.Errorf("guardar archivo: %w", err)
	}
	if err := servicio.metadatos.Crear(imagen); err != nil {
		_ = servicio.archivos.Eliminar(imagen.NombreArchivo()) // no dejar un archivo huérfano
		return models.Imagen{}, fmt.Errorf("guardar metadatos: %w", err)
	}
	return imagen, nil
}

// Listar devuelve solo los metadatos de las imágenes que el usuario puede ver.
func (servicio *Imagenes) Listar(usuario models.Usuario, filtro models.Filtro) []models.Imagen {
	coincidencias := servicio.metadatos.Listar(filtro)
	imagenesVisibles := make([]models.Imagen, 0, len(coincidencias))
	for _, imagen := range coincidencias {
		if puedeVer(usuario, imagen) {
			imagenesVisibles = append(imagenesVisibles, imagen)
		}
	}
	return imagenesVisibles
}

// Descargar devuelve los metadatos y el contenido de una imagen.
// Quien lo llama debe cerrar el io.ReadCloser.
func (servicio *Imagenes) Descargar(usuario models.Usuario, id string) (models.Imagen, io.ReadCloser, error) {
	imagen, err := servicio.metadatos.Obtener(id)
	if err != nil {
		return models.Imagen{}, nil, err
	}
	if !puedeVer(usuario, imagen) {
		return models.Imagen{}, nil, models.ErrNoAutorizado
	}
	archivoAbierto, err := servicio.archivos.Abrir(imagen.NombreArchivo())
	if err != nil {
		return models.Imagen{}, nil, err
	}
	return imagen, archivoAbierto, nil
}

// Actualizar reemplaza los metadatos editables de una imagen (RF-SW-04).
func (servicio *Imagenes) Actualizar(usuario models.Usuario, id string, nuevaMeta models.MetaImagen) (models.Imagen, error) {
	imagen, err := servicio.metadatos.Obtener(id)
	if err != nil {
		return models.Imagen{}, err
	}
	if !puedeModificar(usuario, imagen) {
		return models.Imagen{}, models.ErrNoAutorizado
	}
	if err := imagen.CambiarMeta(nuevaMeta); err != nil {
		return models.Imagen{}, err
	}
	if err := exigirCampana(usuario, imagen.Campana()); err != nil {
		return models.Imagen{}, err
	}
	if err := servicio.metadatos.Actualizar(imagen); err != nil {
		return models.Imagen{}, err
	}
	return imagen, nil
}

// Eliminar borra el archivo y luego sus metadatos (RF-SW-06). Si el borrado
// del archivo falla no se confirma la eliminación.
func (servicio *Imagenes) Eliminar(usuario models.Usuario, id string) error {
	imagen, err := servicio.metadatos.Obtener(id)
	if err != nil {
		return err
	}
	if !puedeModificar(usuario, imagen) {
		return models.ErrNoAutorizado
	}
	if err := servicio.archivos.Eliminar(imagen.NombreArchivo()); err != nil && !errors.Is(err, models.ErrNoEncontrado) {
		return fmt.Errorf("eliminar archivo: %w", err)
	}
	return servicio.metadatos.Eliminar(id)
}
