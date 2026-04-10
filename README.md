# SIA - Sistema de Evaluación en Red Local

SIA es un sistema de evaluación de aula diseñado para funcionar en redes locales (LAN) con un enfoque en la **alta resiliencia**, **seguridad** y **facilidad de uso**. El sistema permite a un profesor gestionar una sala de evaluación y monitorear la actividad de los estudiantes en tiempo real.

## 🚀 Características Principales

- **Descubrimiento Automático (mDNS):** Los clientes encuentran al servidor en la red local automáticamente usando el código de la sala. Cero configuración manual de IPs.
- **Comunicación de Alto Rendimiento:** Utiliza **gRPC** con Protocol Buffers para una comunicación eficiente y tipada entre estudiantes y el servidor.
- **Seguridad HMAC-SHA256:** Las solicitudes de unión están firmadas digitalmente usando el `RoomCode` como clave secreta, evitando suplantaciones.
- **Monitoreo en Tiempo Real:** Interfaz administrativa vía WebSockets que muestra el estado de conexión y **alertas de pérdida de foco** del navegador en los estudiantes.
- **Resiliencia Extrema:** Periodo de gracia de **2 minutos** para reconexión automática de estudiantes en caso de micro-cortes de red.
- **Identidad Persistente:** Generación de `ClientID` único por dispositivo almacenado localmente.
- **Apertura Automática:** El cliente detecta el navegador predeterminado y abre la interfaz de usuario automáticamente al iniciar.

## 🛠️ Arquitectura Técnica

- **Lenguaje:** Go (Golang) con Workspace.
- **Transporte:** gRPC (Puerto 50051 TCP).
- **Descubrimiento:** mDNS / DNS-SD (Puerto 5353 UDP).
- **Web UI:** HTML5/JavaScript (Vanilla) con WebSockets.
- **Seguridad:** Rate Limiting (50 msg/s por cliente) y validación HMAC.

## 📋 Requisitos de Red (Servidor)

Para un correcto funcionamiento, asegúrate de permitir los siguientes puertos en el firewall del servidor:
- `5353/UDP` (mDNS)
- `50051/TCP` (gRPC)
- `8081/TCP` (Panel Admin - Opcional para acceso remoto)

## 🏃 Cómo empezar

### Servidor (Admin)
1. Ejecuta el servidor definiendo un código de sala:
   ```bash
   .\server.exe MI_SALA_2024
   ```
2. Accede al panel en `http://localhost:8081` (Usuario: `admin`, Clave: `MI_SALA_2024`).

### Cliente (Estudiante)
1. Ejecuta el cliente:
   ```bash
   .\client.exe
   ```
2. El navegador se abrirá automáticamente. Ingresa el código de la sala y tu nombre.

---
Desarrollado con ❤️ para entornos educativos resilientes.
