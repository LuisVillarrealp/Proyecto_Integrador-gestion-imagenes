// Package red reúne utilidades de direcciones de red compartidas por el
// servidor y el cliente.
package red

import (
	"net"
	"strings"
)

// EsLoopback indica si direccion ("host:puerto" o "host") apunta SOLO a esta
// máquina. Una dirección sin host (":8443") escucha en todas las interfaces y
// por eso NO es loopback.
func EsLoopback(direccion string) bool {
	direccion = strings.TrimSpace(direccion)
	host := direccion
	if soloHost, _, err := net.SplitHostPort(direccion); err == nil {
		host = soloHost
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	direccionIP := net.ParseIP(host)
	return direccionIP != nil && direccionIP.IsLoopback()
}
