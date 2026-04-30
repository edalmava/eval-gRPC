# SIA - Sistema de Evaluación en Red Local

SIA es un sistema de evaluación de aula diseñado para funcionar en redes locales (LAN) con un enfoque en la **alta resiliencia**, **seguridad** y **facilidad de uso**. El sistema permite a un profesor gestionar una sala de evaluación y monitorear la actividad de los estudiantes en tiempo real.

## 🚀 Características Principales

- **Binarios Autónomos:** Gracias a `go:embed`, todos los archivos de interfaz (HTML/CSS/JS) están integrados dentro de los ejecutables.
- **Descubrimiento Automático (mDNS):** Los clientes encuentran al servidor en la red local automáticamente usando el `RoomCode`.
- **Comunicación de Alto Rendimiento:** Utiliza **gRPC** con Protocol Buffers para una comunicación eficiente y escalable.
- **Seguridad LAN Avanzada:**
  - **HMAC-SHA256:** Toda la comunicación crítica está firmada digitalmente usando el `RoomCode` como secreto.
  - **Rate Limiting:** Protección integrada contra ráfagas de mensajes (50 msg/s por cliente).
  - **Monitoreo de Foco:** Detección en tiempo real si un estudiante sale de la pestaña del navegador.
- **Persistencia Robusta:** Uso de **SQLite con modo WAL** (Write-Ahead Logging) y `busy_timeout` de 5 segundos para garantizar integridad y rendimiento incluso bajo carga.
- **Exportación Profesional:** Generación de reportes detallados en **Excel (.xlsx)** y CSV con resúmenes de efectividad por estudiante y desglose por pregunta.
- **Panel Administrativo Moderno:** Dashboard con **Chart.js** para visualizar respuestas en vivo y alertas de seguridad integradas.
- **Infraestructura Optimizada:** Carga de historial paginada (Lazy Loading) para soportar sesiones largas sin degradar el rendimiento.

## 🛠️ Arquitectura Técnica

- **Lenguaje:** Go (Golang) con estructura de módulos limpia.
- **Base de Datos:** SQLite (con `SetMaxOpenConns(1)` para evitar bloqueos).
- **Transporte:** gRPC (Puerto 50051 TCP) con streams para difusión masiva.
- **WebSockets:** Hub asíncrono para el Panel Admin.
- **Web UI:** HTML5/JavaScript (Vanilla) de alto rendimiento.
- **Reportes:** Motor de generación Excel integrado (`excelize`).

## 📋 Requisitos de Red (Servidor)

Para un correcto funcionamiento, asegúrate de permitir los siguientes puertos en el firewall:
- `5353/UDP` (mDNS para descubrimiento)
- `50051/TCP` (gRPC para comunicación central)
- `8090/TCP` (Panel Administrativo - Acceso desde el navegador del profesor)

## 🏃 Cómo empezar

### Servidor (Admin)
1. Ejecuta el binario definiendo un código de sala único:
   ```bash
   .\server.exe MI_SALA_123
   ```
2. Accede al panel en `http://localhost:8090`.
3. Credenciales: Usuario: `admin` | Contraseña: `MI_SALA_123` (El código de tu sala).

### Cliente (Estudiante)
1. Ejecuta el binario:
   ```bash
   .\client.exe
   ```
2. El navegador se abrirá automáticamente en `http://localhost:8080`.
3. Ingresa el código de la sala y tu nombre para unirte.

## 🛠️ Desarrollo y Compilación

Si deseas compilar el proyecto desde el código fuente:

1. **Generar Proto:**
   ```bash
   protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/sia.proto
   ```
2. **Compilar:**
   ```bash
   go build -o bin/server.exe ./cmd/server/main.go
   go build -o bin/client.exe ./cmd/client/main.go
   ```

## 📅 Hoja de Ruta (Roadmap)
- [ ] Soporte para imágenes y fórmulas en las preguntas.
- [ ] Modo "Examen" para lanzamientos secuenciales automáticos.
- [ ] Integración con sistemas de gestión de aprendizaje (LTI).

---
Desarrollado con ❤️ para entornos educativos resilientes.
