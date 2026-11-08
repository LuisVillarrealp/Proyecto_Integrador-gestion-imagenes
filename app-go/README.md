# app-go: Gestión Segura de Imágenes (servidor y cliente en Go)

Prototipo funcional cliente-servidor del **Proyecto Integrador de tercer nivel**
(UIDE, Ingeniería en Sistemas de Información, sep 2026 – ene 2027).
Segundo entregable: *Programación Orientada a Objetos*, semana 10.

## Descripción

Una agencia de marketing digital con dos sedes maneja las redes sociales de la
FEF y guarda imágenes confidenciales, como los diseños de la nueva camiseta de la
Selección. Hoy las comparte por WhatsApp, Drive y correo, con riesgo de
filtración. Este sistema centraliza esas imágenes en un servidor propio con
control de acceso.

- El **servidor** recibe y procesa las peticiones, valida los archivos, aplica
  los permisos por rol y por campaña, guarda archivos y metadatos por separado y
  registra cada operación.
- El **cliente de consola** es un proceso independiente que se conecta al
  servidor por la red (HTTPS con JSON) y muestra respuestas comprensibles.

La autenticación definitiva corresponde al servicio FastAPI (`auth-services/`).
Go no la duplica: el servidor depende de la interfaz `auth.Autenticador` y en este
prototipo usa tokens de prueba ficticios (ver [Autenticación](#autenticación)).

## Funcionalidades implementadas

| Operación | Comando del cliente | Ruta del servidor | Requisito |
|---|---|---|---|
| Comprobar la conexión | `ping` | `GET /salud` | RF-SW-01 (PI) |
| Cargar una imagen con sus metadatos | `subir` | `POST /imagenes` | RF-SW-01 |
| Validar formato (por contenido), tamaño y datos obligatorios | `subir` | `POST /imagenes` | RF-SW-02, RNF-SW-01 |
| Consultar y buscar con filtros (campaña, etiqueta, nivel, fecha) | `listar` | `GET /imagenes` | RF-SW-03 |
| Clasificar y actualizar metadatos | `actualizar` | `PATCH /imagenes/{id}` | RF-SW-04 |
| Descargar (archivo idéntico al subido) | `descargar` | `GET /imagenes/{id}` | RF-SW-05 |
| Eliminar (con confirmación) | `eliminar` | `DELETE /imagenes/{id}` | RF-SW-06 |
| Ver solo imágenes de campañas asignadas | todos | todas | RF-SW-07, RF-SEC-03 (Fase 2) |
| Registro de actividad con tiempos en ms | automático | todas | RF-SW-09 |
| Tráfico cifrado con TLS y redirección HTTP→HTTPS | `-ca` | servidor | RF-SEC-07 (Fase 2) |

### Requisitos atendidos y pendientes

| Requisito | Estado |
|---|---|
| RF-SW-01 a RF-SW-06, RF-SW-09, RNF-SW-01 | Atendidos |
| RF-SW-07 Asignación de campañas | Parcial: la visibilidad por campaña ya se aplica; la asignación la hace el Administrador en FastAPI (CU-06) |
| RF-SW-08 Validar el JWT de FastAPI | Pendiente (semanas 11-12); el servidor ya está preparado con la interfaz `auth.Autenticador` |
| RNF-SW-02 Subir 10 MB en menos de 10 s | Pendiente de medición en la red de dos sedes (semanas 13-14) |
| CU-09 Estadísticas para el Administrador | Pendiente; los datos ya se registran en `data/actividad.jsonl` |

**Cambio justificado respecto a la Fase 1-2:** la matriz inicial permitía a
todos los empleados ver las imágenes públicas e internas. El documento de
seguridad firmado de la Fase 2 (RF-SEC-03) pide que un Empleado vea **solo** las
imágenes de sus campañas asignadas. Se aplica la regla de la Fase 2, que es la
más restrictiva y la vigente.

### Permisos

| Operación | Empleado | Administrador |
|---|---|---|
| Subir | Solo a campañas que tiene asignadas | Cualquier campaña |
| Listar / descargar | Solo imágenes de sus campañas (cualquier nivel) | Todas |
| Actualizar / eliminar | Solo las que subió, dentro de sus campañas | Todas |

Lo que no está permitido se niega por defecto (403 «acceso denegado») y el
intento queda en el registro de actividad.

## Requisitos de software

- **Go 1.22 o superior** (desarrollado y probado con Go 1.24). Solo se usa la
  librería estándar: no hay dependencias externas.
- **Git**, para clonar el repositorio.
- **OpenSSL** y **Bash**, solo para el modo con HTTPS (generar certificados) y
  para el script de demostración. En Windows vienen con *Git Bash*.
- Sistema operativo: Linux, macOS o Windows.

## Instalación

```bash
git clone https://github.com/LuisVillarrealp/Proyecto_Integrador-gestion-imagenes.git
cd Proyecto_Integrador-gestion-imagenes/app-go

go build -o bin/servidor ./cmd/server
go build -o bin/cliente ./cmd/client
```

### Notas para Windows

- Use **Git Bash** (viene con Git for Windows) para los comandos de este README y
  para los scripts `scripts/*.sh`. En PowerShell o CMD funcionan los comandos de Go,
  pero los scripts no.
- Los binarios se llaman `bin/servidor.exe` y `bin/cliente.exe`. En Git Bash:
  `bin/servidor.exe -addr 127.0.0.1:8443 -sin-tls -auth-dev` y
  `bin/cliente.exe -sin-tls ping`. `scripts/demo.sh` ya detecta el `.exe`.
- `scripts/gen-certs.sh` necesita OpenSSL: Git for Windows lo incluye en
  `C:\Program Files\Git\usr\bin\openssl.exe`. Si `openssl version` no responde en Git Bash,
  agregue esa carpeta al PATH.
- El repositorio incluye `.gitattributes` para que los scripts conserven los saltos
  de línea de Linux (LF) al clonar en Windows.
- Si Windows Defender pregunta por el firewall al iniciar el servidor, permita
  el acceso solo en redes privadas.

## Ejecución rápida (sin cifrado, misma máquina)

Sirve para probar en un solo equipo. Se necesitan **dos terminales**, porque el
servidor y el cliente son procesos independientes.

**Terminal 1: servidor**

```bash
bin/servidor -addr 127.0.0.1:8443 -sin-tls -auth-dev
```

**Terminal 2: cliente**

```bash
bin/cliente -sin-tls ping
bin/cliente -sin-tls -token token-dev-empleado -sede sede-a subir \
  -archivo data/pruebas/camiseta-prueba.png -campana "Camiseta 2026" \
  -etiquetas camiseta,diseno -confidencialidad confidencial
bin/cliente -sin-tls -token token-dev-empleado listar
```

O la demostración completa (operaciones principales y errores) en un solo paso:

```bash
scripts/demo.sh
```

## Ejecución con HTTPS (modo recomendado)

El documento de seguridad de la Fase 2 (RF-SEC-07) exige tráfico cifrado.

1. Generar la CA del proyecto y el certificado del servidor. Se indica la IP o
   el nombre con que los clientes llegarán al servidor (en la red del proyecto,
   `10.10.0.50`):

   ```bash
   scripts/gen-certs.sh certs localhost 127.0.0.1
   ```

   Se crean `certs/ca.pem` (se entrega a los clientes), `certs/servidor.pem` y las
   claves privadas `*.key`. **La carpeta `certs/` nunca se sube al repositorio.**

2. Servidor (terminal 1), con redirección opcional de HTTP a HTTPS:

   ```bash
   bin/servidor -addr 127.0.0.1:8443 -cert certs/servidor.pem -key certs/servidor.key \
     -auth-dev -redirect 127.0.0.1:8080 -host-publico localhost:8443
   ```

3. Cliente (terminal 2):

   ```bash
   bin/cliente -ca certs/ca.pem ping
   PI_CA=certs/ca.pem scripts/demo.sh
   ```

Para ver el candado en el navegador, importe `certs/ca.pem` como autoridad de
confianza y abra `https://localhost:8443/salud` (o `http://localhost:8080/salud`,
que redirige a HTTPS).

## Uso del cliente

```
bin/cliente [opciones globales] <comando> [opciones del comando]
```

| Comando | Ejemplo |
|---|---|
| `ping` | `bin/cliente ping` |
| `subir` | `bin/cliente subir -archivo foto.png -campana "Camiseta 2026" -etiquetas a,b -confidencialidad interna` |
| `listar` | `bin/cliente listar -campana "Camiseta 2026" -etiqueta diseno -confidencialidad publica -desde 2026-11-01` |
| `descargar` | `bin/cliente descargar -id <ID> -salida copia.png` (`-forzar` para sobrescribir) |
| `actualizar` | `bin/cliente actualizar -id <ID> -confidencialidad interna -etiquetas aprobada` |
| `eliminar` | `bin/cliente eliminar -id <ID>` (pide confirmación; `-si` la omite) |
| `ayuda` | `bin/cliente ayuda` o `bin/cliente <comando> -h` |

## Dirección IP, puerto y parámetros configurables

**Servidor (`bin/servidor`)**

| Parámetro | Por defecto | Descripción |
|---|---|---|
| `-addr` | `:8443` | IP y puerto donde escucha. En la red del proyecto: `10.10.0.50:8443` (VLAN 20, Sede A) |
| `-data` | `data` | Directorio de imágenes, metadatos y registro de actividad |
| `-cert`, `-key` | — | Certificado y clave TLS (obligatorios salvo `-sin-tls`) |
| `-redirect` | — | Puerto HTTP que solo redirige GET/HEAD a HTTPS (ej. `:8080`); otros métodos reciben 426 |
| `-host-publico` | — | IP:puerto HTTPS al que apunta la redirección (ej. `10.10.0.50:8443`) |
| `-auth-dev` | apagado | Usa los tokens de prueba de `-usuarios` (solo en loopback) |
| `-usuarios` | `data/usuarios_dev.json` | Archivo de usuarios de prueba |
| `-sin-tls` | apagado | HTTP sin cifrar, solo en loopback (desarrollo) |

El servidor **se niega a iniciar** con una configuración insegura: sin
certificado, sin TLS fuera de loopback o con tokens de prueba expuestos a la red.

**Cliente (`bin/cliente`)**: las opciones globales van **antes** del comando y
también se pueden dar con variables de entorno.

| Opción | Variable | Por defecto | Descripción |
|---|---|---|---|
| `-addr` | `PI_ADDR` | `localhost:8443` | IP y puerto del servidor |
| `-token` | `PI_TOKEN` | — | Token de acceso |
| `-sede` | `PI_SEDE` | — | Sede de origen (`sede-a`, `sede-b`), queda en el registro |
| `-ca` | `PI_CA` | — | Certificado de la CA del proyecto |
| `-sin-tls` | — | apagado | HTTP sin cifrar, solo hacia loopback |

El cliente siempre verifica el certificado del servidor, nunca sigue
redirecciones (el token no viaja a otro destino) y avisa si no puede conectarse.

## Datos y archivos de prueba

Todos son **ficticios**. Nunca se usan las imágenes reales de la camiseta.

| Archivo | Uso |
|---|---|
| `data/pruebas/camiseta-prueba.png` | Imagen PNG válida (campaña «Camiseta 2026», confidencial) |
| `data/pruebas/post-redes.jpg` | Imagen JPG válida (pública) |
| `data/pruebas/banner-redes.png` | Imagen PNG válida para probar campañas no asignadas |
| `data/pruebas/falsa.png` | Texto con extensión `.png`: el servidor lo rechaza (415) |
| `data/pruebas/grande.png` | Más de 10 MB: el cliente y el servidor lo rechazan (413). Lo crea `scripts/demo.sh`; no se sube al repositorio |

Para crear el archivo grande a mano:

```bash
head -c 11000000 /dev/urandom > data/pruebas/grande.png                 # Linux, macOS, Git Bash
```
```powershell
$f=[IO.File]::Create("data\pruebas\grande.png"); $f.SetLength(11000000); $f.Close()   # PowerShell
```

**Usuarios de prueba** (`data/usuarios_dev.json`):

| Token | Usuario | Rol | Campañas asignadas |
|---|---|---|---|
| `token-dev-admin` | u1 | Administrador | todas |
| `token-dev-empleado` | u2 | Empleado | Camiseta 2026 |
| `token-dev-empleado2` | u3 | Empleado | Redes |

## Autenticación

Los tokens anteriores son **datos de demostración ficticios**, no credenciales
reales: no abren ningún otro sistema y el servidor solo los acepta escuchando en
loopback (`127.0.0.1`). Permiten probar los dos roles y la visibilidad por
campaña.

En la integración (semanas 11-12) se reemplazan por el JWT que emite FastAPI al
iniciar sesión, de corta duración y verificado con la clave pública del
servicio. Solo cambia la implementación de `auth.Autenticador`; el resto del
código queda igual.

**Nunca se publican:** claves privadas (`certs/`), la clave de firma de los JWT,
contraseñas reales ni imágenes reales (`data/imagenes/`). Todo eso está en
`.gitignore`.

## Manejo de errores

Ningún error normal usa `panic`: todos se devuelven con el tipo `error` y se
muestran como mensajes claros.

| Situación | Respuesta |
|---|---|
| Datos vacíos o incompletos | 400 «nombre y campaña son obligatorios» |
| Token ausente o inválido | 401, con una pista para revisar el token |
| Operación no autorizada | 403 «acceso denegado» |
| Imagen inexistente | 404 «recurso no encontrado» |
| Archivo de más de 10 MB | El cliente lo detiene antes de enviarlo e indica cuánto pesa; si llega al servidor, 413 |
| Formato no permitido (por contenido, no por extensión) | 415 «solo JPG y PNG» |
| Comando u opción inexistente | Mensaje con la ayuda disponible |
| Servidor apagado o inalcanzable | «no se pudo conectar con el servidor en …» |
| Certificado no confiable | Se rechaza la conexión |
| Fallo de lectura o escritura en disco | 500 genérico al cliente; detalle solo en el log del servidor |

## Estructura de carpetas

```
app-go/
├── cmd/
│   ├── server/        arranque del servidor: parámetros, TLS, cierre ordenado
│   └── client/        cliente de consola: comandos, confirmaciones y salida en pantalla
├── internal/
│   ├── models/        entidades (Imagen, MetaImagen, Usuario, Evento), constructores y errores
│   ├── services/      lógica de negocio: validación por contenido y permisos
│   ├── handlers/      capa HTTP: rutas, respuestas JSON, cabeceras de seguridad, redirección
│   ├── storage/       archivos en disco, metadatos en JSON y registro de actividad (por separado)
│   ├── auth/          interfaz Autenticador e implementación de prueba
│   ├── cliente/       llamadas HTTP del cliente al servidor (sin imprimir nada)
│   └── red/           utilidades de direcciones de red
├── data/
│   ├── usuarios_dev.json   usuarios de prueba ficticios
│   └── pruebas/            imágenes de prueba ficticias
├── scripts/
│   ├── demo.sh        demostración reproducible
│   └── gen-certs.sh   genera la CA y el certificado del servidor
├── go.mod
└── README.md
```

Al ejecutar se crean `data/imagenes/`, `data/metadatos.json` y
`data/actividad.jsonl`. Ninguno se sube al repositorio.

### Programación orientada a objetos en Go

| Elemento | Dónde se ve |
|---|---|
| Structs de las entidades | `models.Imagen`, `models.MetaImagen`, `models.Usuario`, `models.Evento` |
| Campos no exportados y métodos de acceso | `Imagen` y `Usuario` tienen campos privados; se leen con `ID()`, `Campana()`, `Autor()`, etc. Las listas se entregan como copias |
| Constructores con estado válido | `NuevaImagen`, `NuevoUsuario`, `NuevoMetadatos`, `NuevoArchivos`, `NuevoImagenes`, `NuevaAPI`, `cliente.Nuevo` |
| Receptor por puntero para modificar | `(*Imagen).CambiarMeta`, `(*Metadatos).Crear/Actualizar/Eliminar` |
| Composición | `Imagen` contiene una `MetaImagen`; el servicio `Imagenes` se compone de `Metadatos` y `Archivos` |
| Interfaces con propósito | `auth.Autenticador` (cambiar tokens de prueba por JWT sin tocar el resto) y `handlers.ServicioImagenes` (probar la capa HTTP de forma aislada) |
| Errores explícitos | Errores de dominio en `models/errores.go`, envueltos con `%w` y traducidos a códigos HTTP en un solo lugar |

## Pruebas automáticas

```bash
go vet ./...
go test ./...
go test -race ./...    # además detecta problemas de concurrencia (Linux/macOS)
```

Cubren validaciones, permisos por campaña, encapsulamiento, almacenamiento,
la capa HTTP, TLS y el flujo completo del cliente contra un servidor real.

## Limitaciones conocidas

- Autenticación con tokens de prueba hasta integrar el JWT de FastAPI (semanas 11-12).
- La asignación de campañas a usuarios (CU-06) la hará FastAPI; hoy viene del archivo de prueba.
- La sede la informa el cliente en la cabecera `X-Sede`; con la red del proyecto
  se podrá deducir de la IP de origen.
- Los metadatos se guardan en un JSON que se reescribe completo en cada cambio
  (suficiente para el prototipo; reemplazable por una base de datos sin cambiar
  los métodos públicos de `storage.Metadatos`).
- Cada subida se carga en memoria antes de validarse (máximo 10 MB por petición).
- La validación de formato revisa la firma y la cabecera de la imagen, no
  decodifica todos los píxeles.
- Las estadísticas (CU-09) aún no se muestran en la aplicación.
- El cliente es de consola; el tipo de interfaz final se define en la Fase 3.

## Autores y contribuciones

| Integrante | Responsabilidad en el proyecto |
|---|---|
| Luis Alberto Villarreal Pilco | Aplicación Go: servidor, cliente, pruebas y esta documentación (`app-go/`) |
| Byron Ariel Buitrón Valenzuela | Servicio de autenticación FastAPI y requisitos de seguridad (`auth-services/`) |
| Jonathan Alexander Mullo Palma | Infraestructura de red, documentación y estadística (`network/`, `docs/`, `research/`) |

Las contribuciones se pueden verificar en el historial de commits del repositorio.

## Créditos, licencia y uso de IA

- **Licencia:** GPL-3.0, ver `LICENSE` en la raíz del repositorio.
- **Librerías:** solo la librería estándar de Go (BSD-3-Clause). Los
  certificados se generan con OpenSSL.
- **Imágenes de prueba:** generadas para este proyecto; son ficticias y no contienen material de la FEF.
- **Declaración de uso de inteligencia artificial:** se utilizó Claude
  (Anthropic) como asistente para revisar requisitos, proponer el diseño, generar
  y corregir código, escribir pruebas y redactar esta documentación. El equipo
  revisó, ejecutó y probó todo el código entregado, tomó las decisiones de
  diseño y es capaz de explicarlo.
