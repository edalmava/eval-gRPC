package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client representa una conexión WebSocket de administrador con su propio buffer de mensajes.
type Client struct {
	conn *websocket.Conn
	send chan interface{}
	hub  *Hub
}

// Hub gestiona las conexiones WebSocket de los administradores.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan interface{}
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan interface{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Si el buffer del cliente está lleno, desconectarlo (cliente muy lento)
					log.Printf("Buffer de cliente lleno, desconectando...")
					close(client.send)
					delete(h.clients, client)
					client.conn.Close()
				}
			}
			h.mu.Unlock()
		}
	}
}

func (c *Client) writePump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		data, _ := json.Marshal(message)
		if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}

func (h *Hub) ServeHTTP_Manual(conn *websocket.Conn) {
	client := &Client{
		conn: conn,
		send: make(chan interface{}, 256), // Buffer de 256 mensajes
		hub:  h,
	}
	h.register <- client
	go client.writePump()
}

func (h *Hub) Broadcast(msg interface{}) {
	h.broadcast <- msg
}
