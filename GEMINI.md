# SIA - Sistema de Evaluación en Red Local (Instrucciones de Contexto)

Este archivo proporciona contexto técnico y operativo sobre el proyecto SIA para asistir en tareas de desarrollo, mantenimiento y mejora.

## 📌 Descripción General
SIA es una plataforma de evaluación en tiempo real diseñada para redes locales (LAN). Permite a un profesor (Admin) gestionar una sala, lanzar preguntas y monitorear a los estudiantes con alta resiliencia y seguridad.

### Tecnologías Clave
- **Lenguaje:** Go (Golang).
- **Comunicación:** gRPC (Protocol Buffers) para el núcleo y WebSockets para las interfaces UI.
- **Descubrimiento:** mDNS (zeroconf) para localización automática del servidor en la LAN.
- **Seguridad:** Firmas HMAC-SHA256 (usando `RoomCode` como secreto), Rate Limiting (50 msg/s) y detección de pérdida de foco.
- **Interfaz:** HTML5/JS (Vanilla) integrados mediante `go:embed`.

---

## 🏗️ Arquitectura y Estructura de Archivos

### Directorios Principales
- `cmd/server/`: Punto de entrada del servidor y panel administrativo.
- `cmd/client/`: Punto de entrada de la aplicación del estudiante.
- `internal/api/`: Implementación de handlers gRPC y Hub de WebSockets.
- `internal/classroom/`: Lógica de negocio (gestión de estudiantes, estados de preguntas).
- `pkg/`: Modelos de datos compartidos (`models.go`) y utilidades (`utils/`).
- `proto/`: Definiciones de gRPC (`sia.proto`) y código generado.
- `bin/`: Directorio para binarios compilados.

### Flujo de Datos
1. El **Servidor** se anuncia vía mDNS con el prefijo `SIA-`.
2. El **Cliente** descubre el servidor usando el `RoomCode` y se conecta vía gRPC.
3. El **Admin** lanza preguntas que se distribuyen por **gRPC Server Streaming** a los clientes suscritos.
4. Los **Estudiantes** responden y envían eventos de seguridad (pérdida de foco) que se notifican al Admin en tiempo real vía WebSockets.

---

## ⚙️ Comandos de Desarrollo

### Construcción y Ejecución
- **Compilar Servidor:** `go build -o bin/server.exe ./cmd/server/main.go`
- **Compilar Cliente:** `go build -o bin/client.exe ./cmd/client/main.go`
- **Ejecutar Servidor:** `.\bin\server.exe MI_SALA_123`
- **Ejecutar Cliente:** `.\bin\client.exe`

### gRPC / Proto
Si modificas `proto/sia.proto`, regenera el código con:
```powershell
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/sia.proto
```

---

## 🛡️ Convenciones y Seguridad

### Seguridad HMAC
Toda acción crítica (unirse, responder, lanzar pregunta) requiere una firma HMAC-SHA256:
- **Unión:** `HMAC(ClientID + Name + LocalIP, RoomCode)`
- **Respuesta:** `HMAC(QuestionID + ClientID + Answer, RoomCode)`
- **Pregunta:** `HMAC(QuestionID + RoomCode, RoomCode)`

### Gestión de Estado y Persistencia
- El `Manager` en `internal/classroom` es el único responsable de la consistencia de la sala. Utiliza `sync.RWMutex` para concurrencia segura.
- **Persistencia:** SQLite con `journal_mode=WAL` y `busy_timeout=5000`. Se utiliza `SetMaxOpenConns(1)` para evitar errores de "database is locked" durante escrituras concurrentes.
- **Historial:** Carga paginada de 50 elementos para optimizar el consumo de memoria en sesiones largas.
- **Reconexión:** Los estudiantes tienen un **periodo de gracia de 2 minutos** para reconectarse antes de ser eliminados de la lista activa.

### Panel Administrativo (WebSockets)
- El panel se sirve en el puerto `8090`.
- Utiliza un **Hub de WebSockets** asíncrono con un buffer de 256 mensajes por cliente.
- Implementa desconexión automática para clientes lentos que saturen su buffer.

---

## 🛠️ Roadmap / Pendientes (TODO)
- [x] Implementar persistencia opcional (SQLite) para resultados históricos con modo WAL.
- [x] Implementar exportación de resultados a CSV y Excel (librería excelize).
- [x] Mejorar el dashboard administrativo con gráficos (Chart.js) y monitoreo de seguridad.
- [ ] Agregar soporte para imágenes o fórmulas en las preguntas.
- [ ] Implementar un modo de "Examen" con múltiples preguntas secuenciales.
