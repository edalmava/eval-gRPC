package classroom

import (
	"fmt"
	"sia/pkg/models"
	"sia/pkg/utils"
	"sync"
	"time"
)

const GracePeriod = 2 * time.Minute

// Manager gestiona las salas y el estado de los estudiantes.
type Manager struct {
	mu       sync.RWMutex
	RoomCode string
	Students map[string]*models.Student
}

// NewManager crea un nuevo gestor de sala.
func NewManager(roomCode string) *Manager {
	return &Manager{
		RoomCode: utils.NormalizeRoomCode(roomCode),
		Students: make(map[string]*models.Student),
	}
}

// JoinStudent registra o reconecta a un estudiante.
func (m *Manager) JoinStudent(student *models.Student) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.Students[student.ClientID]
	if ok {
		// Reconexión
		existing.Status = "connected"
		existing.LastSeen = time.Now()
		existing.LocalIP = student.LocalIP
		existing.StudentName = student.StudentName
		return nil
	}

	// Nuevo ingreso
	student.Status = "connected"
	student.LastSeen = time.Now()
	m.Students[student.ClientID] = student
	return nil
}

// Heartbeat actualiza el estado 'last_seen' de un estudiante.
func (m *Manager) Heartbeat(clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	student, ok := m.Students[clientID]
	if !ok {
		return fmt.Errorf("estudiante %s no encontrado", clientID)
	}

	student.Status = "connected"
	student.LastSeen = time.Now()
	return nil
}

// SetStatus permite cambiar manualmente el estado (ej. "goodbye").
func (m *Manager) SetStatus(clientID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if student, ok := m.Students[clientID]; ok {
		student.Status = status
		student.LastSeen = time.Now()
	}
}

// CleanupDisconnected monitorea y limpia estudiantes desconectados tras el periodo de gracia.
func (m *Manager) CleanupDisconnected() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		m.mu.Lock()
		for id, student := range m.Students {
			if student.Status == "goodbye" {
				delete(m.Students, id)
				continue
			}

			if time.Since(student.LastSeen) > GracePeriod {
				delete(m.Students, id)
				continue
			}

			if time.Since(student.LastSeen) > 15*time.Second && student.Status == "connected" {
				student.Status = "disconnected"
			}
		}
		m.mu.Unlock()
	}
}

// GetStudents devuelve una copia de la lista de estudiantes.
func (m *Manager) GetStudents() []*models.Student {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*models.Student, 0, len(m.Students))
	for _, s := range m.Students {
		copy := *s
		list = append(list, &copy)
	}
	return list
}
