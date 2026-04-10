package models

import (
	"time"
)

// Student representa el estado de un estudiante en la sala.
type Student struct {
	ClientID    string    `json:"client_id"`
	StudentName string    `json:"student_name"`
	LocalIP     string    `json:"local_ip"`
	Status      string    `json:"status"` // "connected", "disconnected", "goodbye"
	LastSeen    time.Time `json:"last_seen"`
}

// RoomState contiene el estado actual de una sala de evaluación.
type RoomState struct {
	RoomCode string             `json:"room_code"`
	Students map[string]*Student `json:"students"`
}
