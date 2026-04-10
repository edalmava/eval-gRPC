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
	Manager   *classroom.Manager
	Hub       *Hub
	rateLimit sync.Map // Map[string][]time.Time para ClientID -> Timestamps
}

// checkRateLimit devuelve true si el cliente está dentro del límite (50 msg/s).
func (s *SIAServer) checkRateLimit(clientID string) bool {
	now := time.Now()
	val, _ := s.rateLimit.LoadOrStore(clientID, []time.Time{})
	times := val.([]time.Time)

	// Filtrar tiempos antiguos (> 1 seg)
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

	// Verificar HMAC... (resto igual)
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

	err := s.Manager.JoinStudent(student)
	if err != nil {
		return &pb.JoinResponse{Success: false, Message: fmt.Sprintf("Error: %v", err)}, nil
	}

	// Notificar al Admin UI
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
		s.Hub.Broadcast(map[string]string{"client_id": req.ClientId, "status": "connected", "event": "heartbeat"})
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
		"client_id":  req.ClientId,
		"event":      "security",
		"type":       req.EventType,
		"timestamp":  req.Timestamp,
	})
	return &pb.SecurityEventResponse{Received: true}, nil
}
