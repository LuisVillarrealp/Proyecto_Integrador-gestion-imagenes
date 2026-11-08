package models

import "time"

// Evento es una línea del registro de actividad (RF-SW-09): quién hizo qué,
// desde qué sede, con qué resultado y cuánto tardó.
type Evento struct {
	Fecha      time.Time `json:"fecha"`
	Usuario    string    `json:"usuario"`   // ID del usuario, o "anonimo" si no se autenticó
	Sede       string    `json:"sede"`      // sede de origen, informada por el cliente
	Operacion  string    `json:"operacion"` // subir, listar, descargar, actualizar, eliminar
	Recurso    string    `json:"recurso,omitempty"`
	Resultado  string    `json:"resultado"` // ok, denegado o error
	Detalle    string    `json:"detalle,omitempty"`
	Tamano     int64     `json:"tamano,omitempty"` // bytes del archivo, si aplica
	DuracionMs float64   `json:"duracion_ms"`
}
