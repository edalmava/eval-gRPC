# SIA - Sistema de Evaluación en Red Local

SIA es un sistema de evaluación de aula diseñado para funcionar en redes locales (LAN) con un enfoque en la **alta resiliencia**, **seguridad** y **facilidad de uso**. El sistema permite a un profesor gestionar una sala de evaluación y monitorear la actividad de los estudiantes en tiempo real.

## 🚀 Características Principales

- **Binarios Autónomos:** Gracias a `go:embed`, todos los archivos de interfaz (HTML/CSS/JS) están integrados dentro de los ejecutables.
- **Descubrimiento Automático (mDNS):** Los clientes encuentran al servidor en la red local automáticamente usando el código de la sala.
- **Comunicación de Alto Rendimiento:** Utiliza **gRPC** con Protocol Buffers para una comunicación eficiente.
- **Exportación Profesional:** Generación de reportes detallados en **Excel (.xlsx)** y CSV con resúmenes de efectividad por estudiante y desglose por pregunta.
- **Monitoreo y Estadísticas en Vivo:** Interfaz administrativa optimizada (60 FPS) que muestra alertas de seguridad y gráficos de respuestas en tiempo real.
- **Infraestructura Escalable:** Servidor optimizado con carga de historial bajo demanda (Lazy Loading) y manejo asíncrono de WebSockets para soportar grandes grupos de estudiantes.
- **Seguridad HMAC-SHA256:** Toda la comunicación crítica está firmada digitalmente para evitar suplantaciones.
- **Persistencia Robusta:** Uso de SQLite con modo WAL (Write-Ahead Logging) para una persistencia de datos rápida y segura.

## 🛠️ Arquitectura Técnica

- **Lenguaje:** Go (Golang) con Workspace.
- **Base de Datos:** SQLite con índices optimizados y carga paginada.
- **Transporte:** gRPC (Puerto 50051 TCP) con streams para difusión.
- **WebSockets:** Hub asíncrono con buffers por cliente (Admin Panel).
- **Web UI:** HTML5/JavaScript (Vanilla) optimizado para bajo consumo de CPU/RAM.
- **Reportes:** Motor de generación Excel integrado (`excelize`).

## 📋 Requisitos de Red (Servidor)

Para un correcto funcionamiento, asegúrate de permitir los siguientes puertos en el firewall del servidor:
- `5353/UDP` (mDNS)
- `50051/TCP` (gRPC)
- `8081/TCP` (Panel Admin - Opcional para acceso remoto)

## 🏃 Cómo empezar

### Servidor (Admin)
1. Ejecuta el binario definiendo un código de sala:
   ```bash
   .\server.exe MI_SALA_2024
   ```
2. Accede al panel en `http://localhost:8081` (Usuario: `admin`, Clave: `MI_SALA_2024`).

### Cliente (Estudiante)
1. Ejecuta el binario:
   ```bash
   .\client.exe
   ```
2. El navegador se abrirá automáticamente. Ingresa el código de la sala y tu nombre.

## 🛠️ Desarrollo y Compilación

Si deseas compilar el proyecto desde el código fuente:

1. **Regenerar Proto (opcional):**
   ```bash
   protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/sia.proto
   ```
2. **Compilar binarios:**
   ```bash
   go build -o bin/server.exe ./cmd/server/main.go
   go build -o bin/client.exe ./cmd/client/main.go
   ```

---
Desarrollado con ❤️ para entornos educativos resilientes.
