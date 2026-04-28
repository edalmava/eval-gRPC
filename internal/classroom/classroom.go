package classroom

import (
	"fmt"
	"sia/pkg/models"
	"sia/pkg/utils"
	"sync"
	"time"
)

const GracePeriod = 2 * time.Minute

// closedQuestionTTL: tiempo que una pregunta cerrada sigue visible en GetActiveQuestion
// para que el polling del cliente pueda detectar el cierre y mostrar el resultado.
const closedQuestionTTL = 15 * time.Second

// Manager gestiona las salas y el estado de los estudiantes.
type Manager struct {
	mu              sync.RWMutex
	RoomCode        string
	Students        map[string]*models.Student
	ActiveQuestion  *models.ActiveQuestion
	closedAt        time.Time // momento en que se cerró la pregunta activa
	QuestionHistory map[string]*models.ActiveQuestion
	db              *DBManager
}

// NewManager crea un nuevo gestor de sala.
func NewManager(roomCode string, db *DBManager) *Manager {
	m := &Manager{
		RoomCode:        utils.NormalizeRoomCode(roomCode),
		Students:        make(map[string]*models.Student),
		QuestionHistory: make(map[string]*models.ActiveQuestion),
		db:              db,
	}

	// Cargar solo las últimas 20 preguntas al inicio para ahorrar RAM
	if db != nil {
		if history, err := db.LoadHistoryPaged(m.RoomCode, 20, 0); err == nil {
			m.QuestionHistory = history
			fmt.Printf("Caché inicial cargada: %d preguntas recuperadas de la DB\n", len(history))
		}
	}

	return m
}

// OpenQuestion registra una nueva pregunta activa. Retorna error si ya hay una abierta.
func (m *Manager) OpenQuestion(q *models.ActiveQuestion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ActiveQuestion != nil && m.ActiveQuestion.Open {
		return fmt.Errorf("ya hay una pregunta abierta")
	}

	m.ActiveQuestion = q
	m.ActiveQuestion.Open = true
	m.closedAt = time.Time{} // reset
	if m.ActiveQuestion.Answers == nil {
		m.ActiveQuestion.Answers = make(map[string]*models.Answer)
	}
	return nil
}

// SubmitAnswer registra la respuesta de un estudiante.
func (m *Manager) SubmitAnswer(questionID, clientID, studentName, answer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ActiveQuestion == nil || !m.ActiveQuestion.Open || m.ActiveQuestion.QuestionID != questionID {
		return fmt.Errorf("no hay una pregunta activa con ID %s", questionID)
	}

	if _, ok := m.ActiveQuestion.Answers[clientID]; ok {
		return fmt.Errorf("ya has respondido esta pregunta")
	}

	m.ActiveQuestion.Answers[clientID] = &models.Answer{
		ClientID:    clientID,
		StudentName: studentName,
		Answer:      answer,
		Timestamp:   time.Now(),
	}
	return nil
}

// CloseQuestion cierra la pregunta activa y retorna el total de respuestas, la opción correcta y el conteo por opción.
func (m *Manager) CloseQuestion(questionID string) (total int, correct string, counts map[string]int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ActiveQuestion == nil || m.ActiveQuestion.QuestionID != questionID {
		return 0, "", nil, fmt.Errorf("pregunta %s no encontrada o no es la activa", questionID)
	}

	m.ActiveQuestion.Open = false
	m.closedAt = time.Now() // registrar momento de cierre para el TTL
	total = len(m.ActiveQuestion.Answers)
	correct = m.ActiveQuestion.CorrectOption
	counts = make(map[string]int)

	for _, ans := range m.ActiveQuestion.Answers {
		counts[ans.Answer]++
	}

	m.QuestionHistory[questionID] = m.ActiveQuestion
	// Si el historial crece mucho en RAM, podríamos podarlo aquí (opcional)

	// Persistir en DB
	if m.db != nil {
		go func(q models.ActiveQuestion, code string) {
			if err := m.db.SaveQuestion(&q, code); err != nil {
				fmt.Printf("Error al persistir pregunta en DB: %v\n", err)
			}
		}(*m.ActiveQuestion, m.RoomCode)
	}

	return total, correct, counts, nil
}

// GetActiveQuestion retorna la pregunta activa actual.
func (m *Manager) GetActiveQuestion() *models.ActiveQuestion {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ActiveQuestion == nil {
		return nil
	}

	if m.ActiveQuestion.Open {
		return m.ActiveQuestion
	}

	if !m.closedAt.IsZero() && time.Since(m.closedAt) < closedQuestionTTL {
		return m.ActiveQuestion
	}

	m.ActiveQuestion = nil
	m.closedAt = time.Time{}
	return nil
}

// GetResults retorna todas las respuestas de una pregunta.
func (m *Manager) GetResults(questionID string) []*models.Answer {
	m.mu.RLock()
	q, ok := m.QuestionHistory[questionID]
	if !ok && (m.ActiveQuestion == nil || m.ActiveQuestion.QuestionID != questionID) {
		m.mu.RUnlock()
		// Intentar cargar de DB si no está en caché
		if m.db != nil {
			h, _ := m.db.LoadHistoryPaged(m.RoomCode, 50, 0)
			m.mu.Lock()
			for k, v := range h {
				m.QuestionHistory[k] = v
			}
			q, ok = m.QuestionHistory[questionID]
			m.mu.Unlock()
		}
		if !ok {
			return nil
		}
	} else {
		m.mu.RUnlock()
	}

	if q == nil {
		q = m.ActiveQuestion
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]*models.Answer, 0, len(q.Answers))
	for _, ans := range q.Answers {
		results = append(results, ans)
	}
	return results
}

// JoinStudent registra o reconecta a un estudiante.
func (m *Manager) JoinStudent(student *models.Student) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.Students[student.ClientID]
	if ok {
		existing.Status = "connected"
		existing.LastSeen = time.Now()
		existing.LocalIP = student.LocalIP
		existing.StudentName = student.StudentName
		return nil
	}

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

// GetStudentName devuelve el nombre de un estudiante por su ID.
func (m *Manager) GetStudentName(clientID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.Students[clientID]; ok {
		return s.StudentName
	}
	return "Desconocido"
}

// GetHistory retorna el historial (podría ser paginado en el futuro).
func (m *Manager) GetHistory() map[string]*models.ActiveQuestion {
	m.mu.RLock()
	// Si tenemos DB y poco historial en RAM, refrescamos con las últimas 50
	if m.db != nil && len(m.QuestionHistory) < 20 {
		m.mu.RUnlock()
		h, _ := m.db.LoadHistoryPaged(m.RoomCode, 50, 0)
		m.mu.Lock()
		for k, v := range h {
			m.QuestionHistory[k] = v
		}
		m.mu.Unlock()
		m.mu.RLock()
	}
	defer m.mu.RUnlock()
	
	copy := make(map[string]*models.ActiveQuestion)
	for k, v := range m.QuestionHistory {
		copy[k] = v
	}
	return copy
}
