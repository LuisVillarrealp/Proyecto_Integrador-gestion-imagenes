package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

// Actividad guarda el registro de actividad como líneas JSON (una por evento)
// en un archivo al que solo se agrega. Es la fuente de datos del análisis
// estadístico (RF-SW-09).
type Actividad struct {
	candado     sync.Mutex
	archivo     *os.File
	codificador *json.Encoder
}

// NuevaActividad abre (o crea) el archivo de registro.
func NuevaActividad(rutaArchivo string) (*Actividad, error) {
	if err := os.MkdirAll(filepath.Dir(rutaArchivo), 0o750); err != nil {
		return nil, fmt.Errorf("crear directorio del registro: %w", err)
	}
	archivo, err := os.OpenFile(rutaArchivo, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("abrir registro de actividad: %w", err)
	}
	return &Actividad{archivo: archivo, codificador: json.NewEncoder(archivo)}, nil
}

// Registrar agrega un evento. Un fallo al registrar se anota en el log del
// servidor pero nunca hace fallar la petición del usuario.
func (actividad *Actividad) Registrar(evento models.Evento) {
	actividad.candado.Lock()
	defer actividad.candado.Unlock()
	if err := actividad.codificador.Encode(evento); err != nil {
		log.Printf("registro de actividad: %v", err)
	}
}

// Cerrar libera el archivo.
func (actividad *Actividad) Cerrar() error {
	actividad.candado.Lock()
	defer actividad.candado.Unlock()
	return actividad.archivo.Close()
}
