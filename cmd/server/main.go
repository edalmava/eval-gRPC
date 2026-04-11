package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sia/internal/api"
	"sia/internal/classroom"
	"sia/pkg/utils"
	pb "sia/proto"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grandcat/zeroconf"
	"google.golang.org/grpc"
)

var sessionToken = "" // Token temporal para la sesión actual

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Uso: go run cmd/server/main.go <ROOM_CODE>")
	}

	roomCode := utils.NormalizeRoomCode(os.Args[1])
	sessionToken = "sia-session-" + roomCode // Simplificado para red local
	fmt.Printf("Iniciando Servidor de Sala [%s]...\n", roomCode)

	manager := classroom.NewManager(roomCode)
	go manager.CleanupDisconnected()

	hub := api.NewHub()
	go hub.Run()

	// Instancia única del servidor gRPC para compartir con los handlers HTTP
	siaServer := &api.SIAServer{
		Manager: manager,
		Hub:     hub,
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Error al escuchar en puerto 50051: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterSIAServiceServer(grpcServer, siaServer)

	adminMux := http.NewServeMux()

	// Middleware de autenticación simple
	isAuthenticated := func(r *http.Request) bool {
		cookie, err := r.Cookie("session")
		return err == nil && cookie.Value == sessionToken
	}

	// Página de Login
	adminMux.HandleFunc("/login-page", func(w http.ResponseWriter, r *http.Request) {
		_, err := os.Stat("cmd/server/login.html")
		if err == nil {
			http.ServeFile(w, r, "cmd/server/login.html")
			return
		}
		http.ServeFile(w, r, "login.html")
	})

	// Endpoint de Login POST
	adminMux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var creds struct{ User, Pass string }
		json.NewDecoder(r.Body).Decode(&creds)

		// Credenciales: admin / RoomCode
		if creds.User == "admin" && utils.NormalizeRoomCode(creds.Pass) == roomCode {
			http.SetCookie(w, &http.Cookie{
				Name:    "session",
				Value:   sessionToken,
				Path:    "/",
				Expires: time.Now().Add(12 * time.Hour),
			})
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})

	// Página principal (Protegida)
	adminMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}
		_, err := os.Stat("cmd/server/admin.html")
		if err == nil {
			http.ServeFile(w, r, "cmd/server/admin.html")
			return
		}
		http.ServeFile(w, r, "admin.html")
	})

	// WebSocket de Admin (Protegido)
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// API de Preguntas (Proxy a gRPC/Manager)
	adminMux.HandleFunc("/api/question/broadcast", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) || r.Method != http.MethodPost {
			http.Error(w, "No autorizado", http.StatusUnauthorized)
			return
		}
		var req pb.BroadcastQuestionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Usamos la instancia compartida
		res, err := siaServer.BroadcastQuestion(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(res)
	})

	adminMux.HandleFunc("/api/question/close", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) || r.Method != http.MethodPost {
			http.Error(w, "No autorizado", http.StatusUnauthorized)
			return
		}
		var req pb.CloseQuestionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := siaServer.CloseQuestion(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(res)
	})

	adminMux.HandleFunc("/api/question/active", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "No autorizado", http.StatusUnauthorized)
			return
		}
		active := manager.GetActiveQuestion()
		json.NewEncoder(w).Encode(active)
	})

	adminMux.HandleFunc("/api/question/results/", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "No autorizado", http.StatusUnauthorized)
			return
		}
		id := r.URL.Path[len("/api/question/results/"):]
		results := manager.GetResults(id)
		json.NewEncoder(w).Encode(results)
	})

	adminMux.HandleFunc("/ws-admin", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "No autorizado", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Error upgrade WS Admin: %v", err)
			return
		}
		hub.ServeHTTP_Manual(conn)
		initialState := map[string]interface{}{
			"type":     "init",
			"room":     roomCode,
			"students": manager.GetStudents(),
		}
		data, _ := json.Marshal(initialState)
		conn.WriteMessage(websocket.TextMessage, data)
	})

	go func() {
		fmt.Println("Panel Administrativo protegido en http://localhost:8081")
		if err := http.ListenAndServe(":8081", adminMux); err != nil {
			log.Fatalf("Fallo de servidor HTTP Admin: %v", err)
		}
	}()

	server, err := zeroconf.Register("SIA-"+roomCode, "_sia._tcp", "local.", 50051, []string{"room=" + roomCode}, nil)
	if err != nil {
		log.Fatalf("Error al registrar servicio mDNS: %v", err)
	}
	defer server.Shutdown()

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Fallo al servir gRPC: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("\nApagando servidor...")
	grpcServer.GracefulStop()
}
