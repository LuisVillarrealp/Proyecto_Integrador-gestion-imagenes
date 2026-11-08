// Package auth define cómo el servidor Go identifica al usuario de cada petición.
//
// El servidor NO implementa autenticación propia: eso es responsabilidad del
// servicio FastAPI. Aquí solo se define la interfaz que el resto del código
// usa. En este prototipo la implementa Dev (tokens de prueba); en las semanas
// 11-12 se agregará una implementación que valide el JWT emitido por FastAPI,
// sin tocar el resto del código.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes/app-go/internal/models"
)

// Autenticador convierte un token en el usuario que representa.
// Devuelve models.ErrNoAutenticado si el token no es válido.
type Autenticador interface {
	Validar(token string) (models.Usuario, error)
}

// TokenDeCabecera extrae el token de una cabecera "Authorization: Bearer <token>".
func TokenDeCabecera(cabecera string) (string, error) {
	esquema, token, separado := strings.Cut(strings.TrimSpace(cabecera), " ")
	token = strings.TrimSpace(token)
	if !separado || !strings.EqualFold(esquema, "Bearer") || token == "" {
		return "", models.ErrNoAutenticado
	}
	return token, nil
}

// Dev es un Autenticador SOLO PARA DESARROLLO: busca el token en una tabla
// cargada desde un JSON con tokens ficticios. No verifica firmas ni expiración,
// por eso el servidor solo lo acepta escuchando en loopback (ver cmd/server).
// En la integración (semanas 11-12) se reemplaza por la validación del JWT de
// FastAPI, sin cambiar el resto del código.
type Dev struct {
	usuariosPorToken map[string]models.Usuario
}

var _ Autenticador = (*Dev)(nil)

// usuarioDevJSON es la forma de cada usuario de prueba en el archivo JSON.
type usuarioDevJSON struct {
	ID       string     `json:"id"`
	Nombre   string     `json:"nombre"`
	Rol      models.Rol `json:"rol"`
	Campanas []string   `json:"campanas"`
}

// NuevoDev carga la tabla de tokens de prueba desde un archivo JSON con la
// forma {"token": {"id": "...", "nombre": "...", "rol": "...", "campanas": [...]}}.
// Cada usuario pasa por models.NuevoUsuario, así que se valida al cargar.
func NuevoDev(rutaArchivo string) (*Dev, error) {
	contenido, err := os.ReadFile(rutaArchivo)
	if err != nil {
		return nil, fmt.Errorf("leer usuarios de prueba: %w", err)
	}
	var usuariosLeidos map[string]usuarioDevJSON
	if err := json.Unmarshal(contenido, &usuariosLeidos); err != nil {
		return nil, fmt.Errorf("usuarios de prueba inválidos en %s: %w", rutaArchivo, err)
	}
	autenticadorDev := &Dev{usuariosPorToken: make(map[string]models.Usuario, len(usuariosLeidos))}
	for token, leido := range usuariosLeidos {
		usuarioValido, err := models.NuevoUsuario(leido.ID, leido.Nombre, leido.Rol, leido.Campanas...)
		if err != nil {
			return nil, fmt.Errorf("usuario de prueba %q: %w", leido.ID, err)
		}
		autenticadorDev.usuariosPorToken[token] = usuarioValido
	}
	return autenticadorDev, nil
}

// Validar devuelve el usuario asociado al token de prueba.
func (autenticadorDev *Dev) Validar(token string) (models.Usuario, error) {
	usuario, existe := autenticadorDev.usuariosPorToken[token]
	if !existe {
		return models.Usuario{}, models.ErrNoAutenticado
	}
	return usuario, nil
}
