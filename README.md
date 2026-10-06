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
Etapa 1, Planeación del software (en curso). Video: https://www.youtube.com/watch?v=rPyS6hrYkdo 

