// Package models define las entidades del sistema de gestión de imágenes
// y los errores de dominio que comparten todas las capas.
package models

import "errors"

// Errores de dominio. Las capas superiores los envuelven con fmt.Errorf("%w")
// para dar contexto, y el paquete handlers los traduce a códigos HTTP.
var (
	ErrNoEncontrado    = errors.New("recurso no encontrado")
	ErrNoAutenticado   = errors.New("token ausente, inválido o expirado")
	ErrNoAutorizado    = errors.New("acceso denegado: operación no autorizada para este usuario")
	ErrFormatoInvalido = errors.New("formato de archivo no permitido (solo JPG y PNG)")
	ErrTamanoExcedido  = errors.New("el archivo supera el tamaño máximo de 10 MB")
	ErrDatosInvalidos  = errors.New("datos vacíos, incompletos o inválidos")
)
