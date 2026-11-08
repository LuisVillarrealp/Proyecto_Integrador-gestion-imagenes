#!/usr/bin/env bash
# Demostración reproducible del prototipo (cliente contra un servidor YA iniciado
# en otra terminal). Recorre las operaciones principales y varios errores.
#
# Uso (desde app-go/):
#   scripts/demo.sh                 # servidor sin TLS en 127.0.0.1:8443
#   PI_CA=certs/ca.pem scripts/demo.sh   # servidor con HTTPS
set -uo pipefail

DIRECCION="${PI_ADDR:-localhost:8443}"
if [ -n "${PI_CA:-}" ]; then
  CONEXION=(-addr "$DIRECCION" -ca "$PI_CA")
else
  CONEXION=(-addr "$DIRECCION" -sin-tls)
fi
PRUEBAS="data/pruebas"
DESCARGAS="data/descargas"

paso() { printf '\n\033[1;36m== %s ==\033[0m\n$ cliente %s\n' "$1" "${*:2}"; }
# En Windows (Git Bash) el binario necesita la extensión .exe.
case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) EJECUTABLE="bin/cliente.exe" ;; *) EJECUTABLE="bin/cliente" ;; esac
cliente() { "./$EJECUTABLE" "${CONEXION[@]}" "$@"; }

go build -o "$EJECUTABLE" ./cmd/client || exit 1
mkdir -p "$DESCARGAS"

# Archivo de más de 10 MB para la prueba de tamaño (no se sube al repositorio).
[ -f "$PRUEBAS/grande.png" ] || head -c 11000000 /dev/urandom > "$PRUEBAS/grande.png"

paso "1. Conexión con el servidor" ping
cliente ping

EMPLEADO=(-token token-dev-empleado -sede sede-a)
paso "2. Empleado sube una imagen confidencial (PNG)" "${EMPLEADO[*]} subir ..."
SALIDA=$(cliente "${EMPLEADO[@]}" subir -archivo "$PRUEBAS/camiseta-prueba.png" \
  -campana "Camiseta 2026" -etiquetas camiseta,diseno -confidencialidad confidencial)
echo "$SALIDA"
ID_CAMISETA=$(echo "$SALIDA" | awk '/ID:/ {print $2}')

paso "3. Empleado sube una imagen pública (JPG)" "${EMPLEADO[*]} subir ..."
cliente "${EMPLEADO[@]}" subir -archivo "$PRUEBAS/post-redes.jpg" \
  -campana "Camiseta 2026" -etiquetas redes -confidencialidad publica

paso "4. Listar y buscar por etiqueta" "${EMPLEADO[*]} listar -etiqueta diseno"
cliente "${EMPLEADO[@]}" listar
cliente "${EMPLEADO[@]}" listar -etiqueta diseno

paso "5. Descargar" "${EMPLEADO[*]} descargar -id $ID_CAMISETA"
cliente "${EMPLEADO[@]}" descargar -id "$ID_CAMISETA" -salida "$DESCARGAS/camiseta-descargada.png" -forzar
cmp -s "$PRUEBAS/camiseta-prueba.png" "$DESCARGAS/camiseta-descargada.png" && echo "La imagen descargada es idéntica a la original."

OTRO=(-token token-dev-empleado2 -sede sede-b)
paso "6. ERROR: empleado de otra campaña no ve ni descarga (acceso denegado)" "${OTRO[*]} listar / descargar"
cliente "${OTRO[@]}" listar
cliente "${OTRO[@]}" descargar -id "$ID_CAMISETA" -salida "$DESCARGAS/robada.png"

paso "7. ERROR: archivo de texto con extensión .png" "subir -archivo falsa.png"
cliente "${EMPLEADO[@]}" subir -archivo "$PRUEBAS/falsa.png" -campana "Camiseta 2026" -confidencialidad interna

paso "8. ERROR: archivo de más de 10 MB" "subir -archivo grande.png"
cliente "${EMPLEADO[@]}" subir -archivo "$PRUEBAS/grande.png" -campana "Camiseta 2026" -confidencialidad interna

paso "9. ERROR: datos obligatorios vacíos (sin campaña)" "subir -archivo banner-redes.png"
cliente "${EMPLEADO[@]}" subir -archivo "$PRUEBAS/banner-redes.png" -confidencialidad interna

paso "10. ERROR: subir a una campaña no asignada" "subir -campana Redes"
cliente "${EMPLEADO[@]}" subir -archivo "$PRUEBAS/banner-redes.png" -campana Redes -confidencialidad interna

paso "11. ERROR: token inválido" "-token falso listar"
cliente -token falso listar

ADMIN=(-token token-dev-admin -sede sede-a)
paso "12. Administrador: ve todo, clasifica y elimina" "${ADMIN[*]} listar / actualizar / eliminar"
cliente "${ADMIN[@]}" listar
cliente "${ADMIN[@]}" actualizar -id "$ID_CAMISETA" -confidencialidad interna -etiquetas camiseta,aprobada
cliente "${ADMIN[@]}" eliminar -id "$ID_CAMISETA" -si

paso "13. ERROR: recurso no encontrado (ya eliminado)" "descargar -id $ID_CAMISETA"
cliente "${ADMIN[@]}" descargar -id "$ID_CAMISETA" -salida "$DESCARGAS/x.png"

printf '\n\033[1;36m== Registro de actividad (últimas 5 líneas de data/actividad.jsonl) ==\033[0m\n'
tail -n 5 data/actividad.jsonl 2>/dev/null || echo "(el servidor usa otro directorio de datos)"
