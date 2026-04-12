package api

import (
	"context"
	"fmt"
	"sia/internal/classroom"
	"sia/pkg/models"
	"sia/pkg/utils"
	pb "sia/proto"
	"sync"
	"time"
)

// SIAServer implementa el servicio gRPC SIAService.
type SIAServer struct {
	pb.UnimplementedSIAServiceServer
	Manager         *classroom.Manager
	Hub             *Hub
	rateLimit       sync.Map // Map[string][]time.Time para ClientID -> Timestamps
	questionStreams  sync.Map // Map[string]pb.SIAService_SubscribeToQuestionsServer
}

// BroadcastQuestion envía una pregunta a todos los estudiantes suscritos.
func (s *SIAServer) BroadcastQuestion(ctx context.Context, req *pb.BroadcastQuestionRequest) (*pb.BroadcastQuestionResponse, error) {
	adminLimitKey := "admin-" + req.RoomCode
	if !s.checkRateLimit(adminLimitKey) {
		return &pb.BroadcastQuestionResponse{Success: false}, nil
	}

	// Validar admin_signature: HMAC-SHA256 de (question_id + room_code)
	expectedMsg := req.Question.QuestionId + req.RoomCode
	if !utils.VerifyHMAC(expectedMsg, s.Manager.RoomCode, req.AdminSignature) {
		return &pb.BroadcastQuestionResponse{Success: false}, nil
	}

	// Validar correct_option si es MULTIPLE_CHOICE
	if req.Question.Type == pb.QuestionType_MULTIPLE_CHOICE && req.Question.CorrectOption != "" {
		valid := false
		for _, v := range []string{"A", "B", "C", "D"} {
			if req.Question.CorrectOption == v {
				valid = true
				break
			}
		}
		if !valid {
			return &pb.BroadcastQuestionResponse{Success: false}, fmt.Errorf("opción correcta inválida: %s", req.Question.CorrectOption)
		}
	}

	options := make([]models.QuestionOption, len(req.Question.Options))
	for i, o := range req.Question.Options {
		options[i] = models.QuestionOption{ID: o.Id, Text: o.Text}
	}

	activeQ := &models.ActiveQuestion{
		QuestionID:    req.Question.QuestionId,
		Text:          req.Question.Text,
		Type:          req.Question.Type.String(),
		Options:       options,
		CorrectOption: req.Question.CorrectOption,
		CreatedAt:     time.Unix(req.Question.CreatedAt, 0),
		Open:          true,
		Answers:       make(map[string]*models.Answer),
	}

	if err := s.Manager.OpenQuestion(activeQ); err != nil {
		return &pb.BroadcastQuestionResponse{Success: false}, err
	}

	// Broadcast vía Hub a Admins
	s.Hub.Broadcast(map[string]interface{}{
		"type":     "question_broadcast",
		"question": activeQ,
	})

	// Broadcast vía gRPC streams a Estudiantes (SIN correct_option)
	notified := 0
	studentQuestion := *req.Question
	studentQuestion.CorrectOption = "" // No revelar la respuesta correcta al estudiante

	s.questionStreams.Range(func(key, value interface{}) bool {
		stream := value.(pb.SIAService_SubscribeToQuestionsServer)
		if err := stream.Send(&studentQuestion); err != nil {
			s.questionStreams.Delete(key)
		} else {
			notified++
		}
		return true
	})

	return &pb.BroadcastQuestionResponse{Success: true, StudentsNotified: int32(notified)}, nil
}

// SubmitAnswer procesa la respuesta de un estudiante.
func (s *SIAServer) SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.SubmitAnswerResponse, error) {
	if !s.checkRateLimit(req.ClientId) {
		return &pb.SubmitAnswerResponse{Accepted: false, Message: "Rate limit excedido"}, nil
	}

	// Validar HMAC: (question_id + client_id + answer)
	expectedMsg := req.QuestionId + req.ClientId + req.Answer
	if !utils.VerifyHMAC(expectedMsg, s.Manager.RoomCode, req.Signature) {
		return &pb.SubmitAnswerResponse{Accepted: false, Message: "Firma inválida"}, nil
	}

	studentName := s.Manager.GetStudentName(req.ClientId)

	if err := s.Manager.SubmitAnswer(req.QuestionId, req.ClientId, studentName, req.Answer); err != nil {
		return &pb.SubmitAnswerResponse{Accepted: false, Message: err.Error()}, nil
	}

	// Notificar a Admin UI
	s.Hub.Broadcast(map[string]interface{}{
		"type":         "new_answer",
		"question_id":  req.QuestionId,
		"client_id":    req.ClientId,
		"student_name": studentName,
		"answer":       req.Answer,
		"timestamp":    req.Timestamp,
	})

	return &pb.SubmitAnswerResponse{Accepted: true, Message: "Respuesta recibida"}, nil
}

// CloseQuestion cierra una pregunta, notifica a admins y envía el resultado a estudiantes
// mediante una señal especial en el stream gRPC.
func (s *SIAServer) CloseQuestion(ctx context.Context, req *pb.CloseQuestionRequest) (*pb.CloseQuestionResponse, error) {
	total, correct, counts, err := s.Manager.CloseQuestion(req.QuestionId)
	if err != nil {
		return &pb.CloseQuestionResponse{Success: false}, err
	}

	// Notificar al panel de Admin
	s.Hub.Broadcast(map[string]interface{}{
		"type":           "question_closed",
		"question_id":    req.QuestionId,
		"total_answers":  int32(total),
		"correct_option": correct,
		"counts":         counts,
	})

	// --- NUEVO: Notificar a estudiantes vía stream gRPC ---
	// Enviamos un mensaje especial con question_id = "__CLOSED__" y correct_option.
	// El cliente Go lo detecta y reenvía el resultado a la UI del navegador.
	closedSignal := &pb.Question{
		QuestionId:    "__CLOSED__",
		RoomCode:      req.RoomCode,
		Text:          req.QuestionId, // Transportar el ID original en el campo Text
		CorrectOption: correct,
	}

	s.questionStreams.Range(func(key, value interface{}) bool {
		stream := value.(pb.SIAService_SubscribeToQuestionsServer)
		if err := stream.Send(closedSignal); err != nil {
			s.questionStreams.Delete(key)
		}
		return true
	})

	return &pb.CloseQuestionResponse{
		Success:       true,
		TotalAnswers:  int32(total),
		CorrectOption: correct,
		Counts:        utils.MapIntToInt32(counts),
	}, nil
}

// SubscribeToQuestions permite a los estudiantes recibir preguntas en tiempo real.
func (s *SIAServer) SubscribeToQuestions(req *pb.SubscribeRequest, stream pb.SIAService_SubscribeToQuestionsServer) error {
	s.questionStreams.Store(req.ClientId, stream)
	defer s.questionStreams.Delete(req.ClientId)

	<-stream.Context().Done()
	return stream.Context().Err()
}

// checkRateLimit devuelve true si el cliente está dentro del límite (50 msg/s).
func (s *SIAServer) checkRateLimit(clientID string) bool {
	now := time.Now()
	val, _ := s.rateLimit.LoadOrStore(clientID, []time.Time{})
	times := val.([]time.Time)

	var newTimes []time.Time
	for _, t := range times {
		if now.Sub(t) < time.Second {
			newTimes = append(newTimes, t)
		}
	}

	if len(newTimes) >= 50 {
		return false
	}

	newTimes = append(newTimes, now)
	s.rateLimit.Store(clientID, newTimes)
	return true
}

// Join maneja las solicitudes de unión de estudiantes con verificación HMAC.
func (s *SIAServer) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	if !s.checkRateLimit(req.ClientId) {
		return &pb.JoinResponse{Success: false, Message: "Rate limit excedido (50 msg/s)"}, nil
	}

	message := fmt.Sprintf("%s%s%s", req.ClientId, req.StudentName, req.LocalIp)
	if !utils.VerifyHMAC(message, s.Manager.RoomCode, req.Signature) {
		return &pb.JoinResponse{Success: false, Message: "Firma HMAC inválida."}, nil
	}

	student := &models.Student{
		ClientID:    req.ClientId,
		StudentName: req.StudentName,
		LocalIP:     req.LocalIp,
		LastSeen:    time.Now(),
		Status:      "connected",
	}

	if err := s.Manager.JoinStudent(student); err != nil {
		return &pb.JoinResponse{Success: false, Message: fmt.Sprintf("Error: %v", err)}, nil
	}

	s.Hub.Broadcast(student)

	return &pb.JoinResponse{Success: true, Message: "Unión exitosa."}, nil
}

// Heartbeat actualiza el estado de conexión del estudiante.
func (s *SIAServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if !s.checkRateLimit(req.ClientId) {
		return &pb.HeartbeatResponse{Acknowledged: false}, nil
	}
	err := s.Manager.Heartbeat(req.ClientId)
	if err == nil {
		s.Hub.Broadcast(map[string]interface{}{
			"client_id": req.ClientId,
			"status":    "connected",
			"event":     "heartbeat",
			"last_seen": time.Now().Format(time.RFC3339),
		})
	}
	return &pb.HeartbeatResponse{Acknowledged: err == nil}, nil
}

// ReportSecurityEvent registra eventos de seguridad.
func (s *SIAServer) ReportSecurityEvent(ctx context.Context, req *pb.SecurityEventRequest) (*pb.SecurityEventResponse, error) {
	if !s.checkRateLimit(req.ClientId) {
		return &pb.SecurityEventResponse{Received: false}, nil
	}
	fmt.Printf("EVENTO DE SEGURIDAD [%s]: %s para el cliente %s\n",
		time.Unix(req.Timestamp, 0).Format(time.RFC822), req.EventType, req.ClientId)

	s.Hub.Broadcast(map[string]interface{}{
		"client_id": req.ClientId,
		"event":     "security",
		"type":      req.EventType,
		"timestamp": req.Timestamp,
		"last_seen": time.Now().Format(time.RFC3339),
	})
	return &pb.SecurityEventResponse{Received: true}, nil
}
