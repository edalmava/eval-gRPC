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

// ActiveQuestion representa la pregunta actualmente abierta en la sala.
type ActiveQuestion struct {
    QuestionID      string            `json:"question_id"`
    Text            string            `json:"text"`
    Type            string            `json:"type"` // "text" o "multiple_choice"
    Options         []QuestionOption  `json:"options,omitempty"`
    CorrectOption   string            `json:"correct_option,omitempty"` // NUEVO
    DurationSeconds int32             `json:"duration_seconds"`         // NUEVO
    CreatedAt       time.Time         `json:"created_at"`
    Answers         map[string]*Answer `json:"answers"` // key = ClientID
    Open            bool              `json:"open"`
}

type QuestionOption struct {
    ID   string `json:"id"`
    Text string `json:"text"`
}

type Answer struct {
    ClientID    string    `json:"client_id"`
    StudentName string    `json:"student_name"`
    Answer      string    `json:"answer"`
    Timestamp   time.Time `json:"timestamp"`
}

// RoomState contiene el estado actual de una sala de evaluación.
type RoomState struct {
	RoomCode string             `json:"room_code"`
	Students map[string]*Student `json:"students"`
}
