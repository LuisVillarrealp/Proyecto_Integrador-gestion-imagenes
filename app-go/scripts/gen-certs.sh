#!/usr/bin/env bash
# Genera una CA propia del proyecto y un certificado de servidor firmado por ella.
# Uso: scripts/gen-certs.sh [directorio] [IP o nombre del servidor]...
# Ejemplo: scripts/gen-certs.sh certs 10.10.0.50
# Resultado en el directorio: ca.pem (se distribuye a clientes/navegadores),
# ca.key (SE GUARDA APARTE, nunca en el repositorio), servidor.pem y servidor.key.
set -euo pipefail

# Git Bash (Windows) convierte "/O=..." en una ruta de archivo; esto lo evita.
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL="*"

DIR="${1:-certs}"
shift || true
HOSTS=("$@")
[ ${#HOSTS[@]} -eq 0 ] && HOSTS=("localhost" "127.0.0.1")

mkdir -p "$DIR"
umask 077

# Autoridad certificadora (EC P-256, 1 año)
openssl ecparam -name prime256v1 -genkey -noout -out "$DIR/ca.key"
openssl req -x509 -new -key "$DIR/ca.key" -sha256 -days 365 \
  -subj "/O=Proyecto Integrador UIDE/CN=CA Proyecto Integrador" \
  -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
  -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -out "$DIR/ca.pem"

# Los navegadores y Go validan el SAN (no el CN): se listan todos los destinos.
SAN=""
for h in "${HOSTS[@]}"; do
  if [[ "$h" =~ ^[0-9.]+$ || "$h" == *:* ]]; then SAN+="IP:$h,"; else SAN+="DNS:$h,"; fi
done
SAN="${SAN%,}"

openssl ecparam -name prime256v1 -genkey -noout -out "$DIR/servidor.key"
openssl req -new -key "$DIR/servidor.key" -subj "/O=Proyecto Integrador UIDE/CN=${HOSTS[0]}" -out "$DIR/servidor.csr"
# Archivo de extensiones real (no <(...)): openssl.exe de Windows no lee tuberías de Bash.
printf "subjectAltName=%s\nbasicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\n" "$SAN" > "$DIR/extensiones.cnf"
openssl x509 -req -in "$DIR/servidor.csr" -CA "$DIR/ca.pem" -CAkey "$DIR/ca.key" -CAcreateserial \
  -sha256 -days 180 -extfile "$DIR/extensiones.cnf" -out "$DIR/servidor.pem"
rm -f "$DIR/servidor.csr" "$DIR/ca.srl" "$DIR/extensiones.cnf"
chmod 600 "$DIR"/*.key 2>/dev/null || true
echo "Listo en $DIR/ (SAN: $SAN). Instale ca.pem como autoridad de confianza en los equipos cliente."
