# Gestión Segura de Imágenes

Plataforma cliente-servidor para gestionar de forma segura imágenes
confidenciales entre las dos sedes de una agencia de marketing digital.

**Proyecto Integrador, Etapa 1** · Universidad Internacional del Ecuador
Programación orientada a objetos · Docente: Iván Galo Reyes Chacón

## Integrantes
| Integrante | 
| Luis Alberto Villarreal Pilco | 
| Byron Ariel Buitrón Valenzuela | 
| Jonathan Alexander Mullo Palma | 

## Problema
La agencia maneja las redes de la Federación Ecuatoriana de Fútbol, incluyendo
material aún no público. Las dos sedes lo comparten por WhatsApp, Drive personal
y correo, sin control de acceso ni trazabilidad: riesgo crítico de filtración.

## Solución
Aplicación en **Go** (cliente y servidor) integrada con autenticación en
**FastAPI** mediante tokens **JWT**.

## Alcance
- Subir, consultar, clasificar, descargar y eliminar imágenes con metadatos
- Registro, login y gestión de usuarios y roles (Empleado y Administrador)
- Comunicación cifrada (HTTPS) entre ambas sedes
- Registro de actividad y estadísticas (subidas, descargas, tiempos, accesos denegados)

## Estado

- *Etapa 1, Planeación del software:* entregada. Video: https://www.youtube.com/watch?v=rPyS6hrYkdo
- *Etapa 2, Prototipo en Go (cliente y servidor). Video demostrativo: https://www.youtube.com/watch?v=T8IZ7AxTqlw
- *Siguiente:* integración con FastAPI (JWT), estadísticas y pruebas en la red de las dos sedes.

## Estructura del repositorio

| Carpeta | Contenido |
| app-go/ | Aplicación cliente-servidor en Go (ver su README para ejecutarla) |
| auth-service/ | Servicio de autenticación en FastAPI |
| network/ | Plan y configuración de la red de las dos sedes |
| research/ | Investigación |
| docs/ | Informes y entregables |
markdown
## Red de las dos sedes

| Elemento | Valor |
| Sede A | 10.10.0.0/24 |
| Sede B | 10.10.1.0/24 |
| Enlace entre routers | 192.168.100.0/24 |
| VLAN | 10, 20, 30 y 99 |
| Servidor Go | 10.10.0.50:8443 (VLAN 20, Sede A) |
| Servidor FastAPI | 10.10.0.51:9443 (VLAN 20, Sede A) |

Direccionamiento VLSM dentro de 10.10.0.0/16, con enrutamiento OSPF entre sedes.
La red se simula con OpenWrt en máquinas virtuales. El detalle está en la carpeta network/.

## Documentación

Los informes de cada fase y el entregable en PDF están en la carpeta docs/.
