package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sia/pkg/utils"
	pb "sia/proto"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/grandcat/zeroconf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:embed index.html
var indexHTML []byte

const ClientIDFile = "client_id.txt"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		// 'start' abre el navegador predeterminado en Windows
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("plataforma no soportada")
	}
	if err != nil {
		log.Printf("No se pudo abrir el navegador: %v", err)
	}
}

func getClientID() string {
	data, err := os.ReadFile(ClientIDFile)
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	id := uuid.New().String()
	_ = os.WriteFile(ClientIDFile, []byte(id), 0644)
	return id
}

type ClientApp struct {
	ClientID   string
	RoomCode   string
	ServerAddr string
	GRPCClient pb.SIAServiceClient
}

func (c *ClientApp) DiscoverServer(ctx context.Context, targetRoom string) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	found := make(chan bool)

	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			for _, txt := range entry.Text {
				if strings.Contains(strings.ToUpper(txt), "ROOM="+targetRoom) {
					addr := ""
					if len(entry.AddrIPv4) > 0 {
						addr = entry.AddrIPv4[0].String()
					} else if len(entry.AddrIPv6) > 0 {
						addr = "[" + entry.AddrIPv6[0].String() + "]"
					}

					if addr != "" {
						c.ServerAddr = fmt.Sprintf("%s:%d", addr, entry.Port)
						fmt.Printf("Servidor encontrado en: %s\n", c.ServerAddr)
						found <- true
						return
					}
				}
			}
		}
	}(entries)

	err = resolver.Browse(ctx, "_sia._tcp", "local.", entries)
	if err != nil {
		return err
	}

	select {
	case <-found:
		return nil
	case <-time.After(7 * time.Second):
		return fmt.Errorf("no se encontró servidor para la sala %s", targetRoom)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (app *ClientApp) handleLocalWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error local WS upgrade: %v", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			continue
		}

		switch data["type"] {
		case "join_request":
			room := utils.NormalizeRoomCode(data["room"].(string))
			name := data["name"].(string)
			fmt.Printf("Solicitud de unión: Sala %s, Estudiante %s\n", room, name)
			
			app.RoomCode = room
			go app.processJoin(conn, room, name)

		case "submit_answer":
			if app.GRPCClient == nil { return }
			questionID := data["question_id"].(string)
			answer := data["answer"].(string)
			message := questionID + app.ClientID + answer
			signature := utils.GenerateHMAC(message, app.RoomCode)

			res, err := app.GRPCClient.SubmitAnswer(context.Background(), &pb.SubmitAnswerRequest{
				QuestionId: questionID,
				ClientId:   app.ClientID,
				RoomCode:   app.RoomCode,
				Answer:     answer,
				Timestamp:  time.Now().Unix(),
				Signature:  signature,
			})
			
			accepted := (err == nil && res != nil && res.Accepted)
			resultMsg, _ := json.Marshal(map[string]interface{}{
				"type":     "answer_result",
				"accepted": accepted,
			})
			conn.WriteMessage(websocket.TextMessage, resultMsg)

		case "focus_lost":
			if app.GRPCClient != nil {
				fmt.Println("¡PÉRDIDA DE FOCO DETECTADA!")
				_, _ = app.GRPCClient.ReportSecurityEvent(context.Background(), &pb.SecurityEventRequest{
					ClientId:  app.ClientID,
					RoomCode:  app.RoomCode,
					EventType: "focus_lost",
					Timestamp: time.Now().Unix(),
				})
			}
		}
	}
}

func (app *ClientApp) processJoin(ws *websocket.Conn, room, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.DiscoverServer(ctx, room); err != nil {
		errMsg, _ := json.Marshal(map[string]string{"type": "error", "message": err.Error()})
		ws.WriteMessage(websocket.TextMessage, errMsg)
		return
	}

	conn, err := grpc.Dial(app.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		errMsg, _ := json.Marshal(map[string]string{"type": "error", "message": "Fallo conexión gRPC"})
		ws.WriteMessage(websocket.TextMessage, errMsg)
		return
	}
	app.GRPCClient = pb.NewSIAServiceClient(conn)

	localIP, _ := utils.GetLocalIP()
	message := fmt.Sprintf("%s%s%s", app.ClientID, name, localIP)
	signature := utils.GenerateHMAC(message, room)

	res, err := app.GRPCClient.Join(context.Background(), &pb.JoinRequest{
		ClientId:    app.ClientID,
		StudentName: name,
		LocalIp:     localIP,
		Signature:   signature,
	})

	if err != nil || !res.Success {
		msg := "Error al unirse"
		if res != nil { msg = res.Message }
		errMsg, _ := json.Marshal(map[string]string{"type": "error", "message": msg})
		ws.WriteMessage(websocket.TextMessage, errMsg)
		return
	}

	// Éxito
	initMsg, _ := json.Marshal(map[string]string{
		"type": "init",
		"id":   app.ClientID,
		"name": name,
		"room": room,
	})
	ws.WriteMessage(websocket.TextMessage, initMsg)

	// Iniciar suscripción a preguntas
	go app.subscribeToQuestions(ws, room)

	// Heartbeat Loop
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			_, err := app.GRPCClient.Heartbeat(context.Background(), &pb.HeartbeatRequest{
				ClientId: app.ClientID,
				RoomCode: room,
			})
			if err != nil { return }
		}
	}()
}

func (app *ClientApp) subscribeToQuestions(ws *websocket.Conn, room string) {
	stream, err := app.GRPCClient.SubscribeToQuestions(context.Background(), &pb.SubscribeRequest{
		ClientId: app.ClientID,
		RoomCode: room,
	})
	if err != nil {
		log.Printf("Error al suscribirse a preguntas: %v", err)
		return
	}

	for {
		q, err := stream.Recv()
		if err != nil {
			log.Printf("Stream de preguntas cerrado: %v", err)
			break
		}

		options := make([]map[string]string, 0)
		for _, o := range q.Options {
			options = append(options, map[string]string{"id": o.Id, "text": o.Text})
		}

		msg, _ := json.Marshal(map[string]interface{}{
			"type":          "question_incoming",
			"question_id":   q.QuestionId,
			"text":          q.Text,
			"question_type": q.Type.String(),
			"options":       options,
		})
		ws.WriteMessage(websocket.TextMessage, msg)
	}
}

func main() {
	app := &ClientApp{
		ClientID: getClientID(),
	}

	// Servidor web local
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	
	http.HandleFunc("/ws-local", app.handleLocalWS)

	go func() {
		fmt.Println("Interfaz de Estudiante en http://localhost:8080")
		// Abrir navegador automáticamente tras iniciar el servidor
		go func() {
			time.Sleep(500 * time.Millisecond) // Dar tiempo al servidor para iniciar
			openBrowser("http://localhost:8080")
		}()
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Fallo del servidor web local: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("Cerrando cliente...")
}
