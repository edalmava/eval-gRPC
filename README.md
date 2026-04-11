# SIA - Sistema de Evaluación en Red Local

SIA es un sistema de evaluación de aula diseñado para funcionar en redes locales (LAN) con un enfoque en la **alta resiliencia**, **seguridad** y **facilidad de uso**. El sistema permite a un profesor gestionar una sala de evaluación y monitorear la actividad de los estudiantes en tiempo real.

## 🚀 Características Principales

- **Binarios Autónomos:** Gracias a `go:embed`, todos los archivos de interfaz (HTML/CSS/JS) están integrados dentro de los ejecutables. No necesitas archivos externos para correr el sistema.
- **Descubrimiento Automático (mDNS):** Los clientes encuentran al servidor en la red local automáticamente usando el código de la sala. Cero configuración manual de IPs.
- **Comunicación de Alto Rendimiento:** Utiliza **gRPC** con Protocol Buffers para una comunicación eficiente y tipada entre estudiantes y el servidor.
- **Módulo de Preguntas en Clase:** El profesor puede lanzar preguntas (texto o opción múltiple) que aparecen instantáneamente en los dispositivos de los alumnos mediante **gRPC Server Streaming**.
- **Monitoreo y Estadísticas en Vivo:** Interfaz administrativa que muestra el estado de conexión, alertas de pérdida de foco y gráficos de respuestas en tiempo real.
- **Seguridad HMAC-SHA256:** Las solicitudes de unión y respuestas están firmadas digitalmente usando el `RoomCode` como clave secreta, evitando suplantaciones.
- **Resiliencia Extrema:** Periodo de gracia de **2 minutos** para reconexión automática de estudiantes en caso de micro-cortes de red.
- **Identidad Persistente:** Generación de `ClientID` único por dispositivo almacenado localmente.
- **Apertura Automática:** El cliente detecta el navegador predeterminado y abre la interfaz de usuario automáticamente al iniciar.

## 🛠️ Arquitectura Técnica

- **Lenguaje:** Go (Golang) con Workspace.
- **Empaquetado:** `go:embed` para recursos estáticos autónomos.
- **Transporte:** gRPC (Puerto 50051 TCP) con streams para difusión de preguntas.
- **Descubrimiento:** mDNS / DNS-SD (Puerto 5353 UDP).
- **Web UI:** HTML5/JavaScript (Vanilla) con WebSockets y CSS moderno.
- **Seguridad:** Rate Limiting (50 msg/s por cliente) y validación HMAC-SHA256.

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
