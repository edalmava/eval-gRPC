package classroom

import (
	"database/sql"
	"sia/pkg/models"
	"time"

	_ "modernc.org/sqlite"
)

type DBManager struct {
	db *sql.DB
}

func NewDBManager(path string) (*DBManager, error) {
	// Optimizar SQLite para concurrencia y velocidad:
	// - journal_mode=WAL: Permite que lectores y escritores no se bloqueen entre sí.
	// - synchronous=NORMAL: Mejora el rendimiento de escritura.
	// - busy_timeout=5000: Espera hasta 5s si la DB está ocupada antes de fallar.
	// - _pragma=foreign_keys(ON): Habilita la integridad referencial.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Limitar a una sola conexión abierta para evitar conflictos de escritura (Database is locked),
	// ya que SQLite maneja mejor las colas de espera internas con una sola conexión.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	dm := &DBManager{db: db}
	if err := dm.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return dm, nil
}

func (dm *DBManager) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS questions (
			id TEXT PRIMARY KEY,
			room_code TEXT,
			text TEXT,
			type TEXT,
			correct_option TEXT,
			duration_seconds INTEGER,
			created_at DATETIME,
			open BOOLEAN
		)`,
		`CREATE TABLE IF NOT EXISTS options (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_id TEXT,
			option_label TEXT,
			option_text TEXT,
			FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS answers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_id TEXT,
			client_id TEXT,
			student_name TEXT,
			answer_text TEXT,
			timestamp DATETIME,
			FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_room ON questions(room_code)`,
		`CREATE INDEX IF NOT EXISTS idx_options_question ON options(question_id)`,
		`CREATE INDEX IF NOT EXISTS idx_answers_question ON answers(question_id)`,
	}

	for _, q := range queries {
		if _, err := dm.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (dm *DBManager) SaveQuestion(q *models.ActiveQuestion, roomCode string) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT OR REPLACE INTO questions (id, room_code, text, type, correct_option, duration_seconds, created_at, open)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		q.QuestionID, roomCode, q.Text, q.Type, q.CorrectOption, q.DurationSeconds, q.CreatedAt, q.Open,
	)
	if err != nil {
		return err
	}

	// Guardar opciones
	for _, opt := range q.Options {
		_, err = tx.Exec(
			`INSERT INTO options (question_id, option_label, option_text) VALUES (?, ?, ?)`,
			q.QuestionID, opt.ID, opt.Text,
		)
		if err != nil {
			return err
		}
	}

	// Guardar respuestas
	for _, ans := range q.Answers {
		_, err = tx.Exec(
			`INSERT INTO answers (question_id, client_id, student_name, answer_text, timestamp)
			 VALUES (?, ?, ?, ?, ?)`,
			q.QuestionID, ans.ClientID, ans.StudentName, ans.Answer, ans.Timestamp,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (dm *DBManager) LoadHistory(roomCode string) (map[string]*models.ActiveQuestion, error) {
	history := make(map[string]*models.ActiveQuestion)

	// 1. Cargar todas las preguntas
	rows, err := dm.db.Query(`SELECT id, text, type, correct_option, duration_seconds, created_at, open FROM questions WHERE room_code = ?`, roomCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questionIDs []interface{}
	for rows.Next() {
		q := &models.ActiveQuestion{
			Answers: make(map[string]*models.Answer),
		}
		var createdAt time.Time
		err := rows.Scan(&q.QuestionID, &q.Text, &q.Type, &q.CorrectOption, &q.DurationSeconds, &createdAt, &q.Open)
		if err != nil {
			return nil, err
		}
		q.CreatedAt = createdAt
		history[q.QuestionID] = q
		questionIDs = append(questionIDs, q.QuestionID)
	}

	if len(questionIDs) == 0 {
		return history, nil
	}

	// Helper para crear placeholders (?, ?, ?)
	placeholders := "?"
	for i := 1; i < len(questionIDs); i++ {
		placeholders += ", ?"
	}

	// 2. Cargar todas las opciones de una vez
	optRows, err := dm.db.Query(`SELECT question_id, option_label, option_text FROM options WHERE question_id IN (`+placeholders+`)`, questionIDs...)
	if err != nil {
		return nil, err
	}
	defer optRows.Close()
	for optRows.Next() {
		var qid string
		var opt models.QuestionOption
		if err := optRows.Scan(&qid, &opt.ID, &opt.Text); err == nil {
			if q, ok := history[qid]; ok {
				q.Options = append(q.Options, opt)
			}
		}
	}

	// 3. Cargar todas las respuestas de una vez
	ansRows, err := dm.db.Query(`SELECT question_id, client_id, student_name, answer_text, timestamp FROM answers WHERE question_id IN (`+placeholders+`)`, questionIDs...)
	if err != nil {
		return nil, err
	}
	defer ansRows.Close()
	for ansRows.Next() {
		var qid string
		ans := &models.Answer{}
		var ts time.Time
		if err := ansRows.Scan(&qid, &ans.ClientID, &ans.StudentName, &ans.Answer, &ts); err == nil {
			ans.Timestamp = ts
			if q, ok := history[qid]; ok {
				q.Answers[ans.ClientID] = ans
			}
		}
	}

	return history, nil
}

func (dm *DBManager) Close() error {
	return dm.db.Close()
}

func (dm *DBManager) LoadHistoryPaged(roomCode string, limit, offset int) (map[string]*models.ActiveQuestion, error) {
	history := make(map[string]*models.ActiveQuestion)

	rows, err := dm.db.Query(`SELECT id, text, type, correct_option, duration_seconds, created_at, open 
		FROM questions WHERE room_code = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, roomCode, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questionIDs []interface{}
	for rows.Next() {
		q := &models.ActiveQuestion{Answers: make(map[string]*models.Answer)}
		var createdAt time.Time
		err := rows.Scan(&q.QuestionID, &q.Text, &q.Type, &q.CorrectOption, &q.DurationSeconds, &createdAt, &q.Open)
		if err != nil {
			return nil, err
		}
		q.CreatedAt = createdAt
		history[q.QuestionID] = q
		questionIDs = append(questionIDs, q.QuestionID)
	}

	if len(questionIDs) == 0 {
		return history, nil
	}

	placeholders := "?"
	for i := 1; i < len(questionIDs); i++ {
		placeholders += ", ?"
	}

	// Opciones
	optRows, err := dm.db.Query(`SELECT question_id, option_label, option_text FROM options WHERE question_id IN (`+placeholders+`)`, questionIDs...)
	if err != nil {
		return nil, err
	}
	defer optRows.Close()
	for optRows.Next() {
		var qid string
		var opt models.QuestionOption
		if err := optRows.Scan(&qid, &opt.ID, &opt.Text); err == nil {
			if q, ok := history[qid]; ok {
				q.Options = append(q.Options, opt)
			}
		}
	}

	// Respuestas
	ansRows, err := dm.db.Query(`SELECT question_id, client_id, student_name, answer_text, timestamp FROM answers WHERE question_id IN (`+placeholders+`)`, questionIDs...)
	if err != nil {
		return nil, err
	}
	defer ansRows.Close()
	for ansRows.Next() {
		var qid string
		ans := &models.Answer{}
		var ts time.Time
		if err := ansRows.Scan(&qid, &ans.ClientID, &ans.StudentName, &ans.Answer, &ts); err == nil {
			ans.Timestamp = ts
			if q, ok := history[qid]; ok {
				q.Answers[ans.ClientID] = ans
			}
		}
	}

	return history, nil
}
