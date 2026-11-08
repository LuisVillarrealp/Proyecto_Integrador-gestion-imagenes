package models

import (
	"fmt"
	"slices"
	"strings"
)

// Rol determina qué operaciones puede hacer un usuario (matriz de permisos
// del documento de seguridad de la Fase 2).
type Rol string

const (
	RolEmpleado      Rol = "Empleado"
	RolAdministrador Rol = "Administrador"
)

// Usuario es la identidad que entrega el servicio de autenticación.
// El servidor Go no guarda contraseñas: solo sabe quién es, su rol y sus
// campañas asignadas. Los campos son privados: una vez validada, la identidad
// no se puede alterar desde otro paquete.
type Usuario struct {
	id       string
	nombre   string
	rol      Rol
	campanas []string // campañas asignadas (RF-SW-07)
}

// NuevoUsuario crea un Usuario con estado válido.
func NuevoUsuario(id, nombre string, rol Rol, campanasAsignadas ...string) (Usuario, error) {
	id = strings.TrimSpace(id)
	nombre = strings.TrimSpace(nombre)
	if id == "" || nombre == "" {
		return Usuario{}, fmt.Errorf("%w: id y nombre son obligatorios", ErrDatosInvalidos)
	}
	if rol != RolEmpleado && rol != RolAdministrador {
		return Usuario{}, fmt.Errorf("%w: rol desconocido %q", ErrDatosInvalidos, rol)
	}
	campanasLimpias := make([]string, 0, len(campanasAsignadas))
	for _, campana := range campanasAsignadas {
		if campana = strings.TrimSpace(campana); campana != "" {
			campanasLimpias = append(campanasLimpias, campana)
		}
	}
	return Usuario{id: id, nombre: nombre, rol: rol, campanas: campanasLimpias}, nil
}

// Métodos de acceso (solo lectura).

func (usuario Usuario) ID() string            { return usuario.id }
func (usuario Usuario) Nombre() string        { return usuario.nombre }
func (usuario Usuario) Rol() Rol              { return usuario.rol }
func (usuario Usuario) Campanas() []string    { return slices.Clone(usuario.campanas) }
func (usuario Usuario) EsAdministrador() bool { return usuario.rol == RolAdministrador }

// TieneCampana indica si la campaña está asignada al usuario (sin distinguir
// mayúsculas). El Administrador no necesita asignación: se decide en el servicio.
func (usuario Usuario) TieneCampana(campana string) bool {
	campana = strings.TrimSpace(campana)
	for _, campanaAsignada := range usuario.campanas {
		if strings.EqualFold(campanaAsignada, campana) {
			return true
		}
	}
	return false
}
