package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// NormalizeRoomCode convierte el código de la sala a MAYÚSCULAS.
func NormalizeRoomCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// GenerateHMAC genera una firma HMAC-SHA256 usando el RoomCode como clave.
func GenerateHMAC(message, roomCode string) string {
	secret := []byte(NormalizeRoomCode(roomCode))
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMAC verifica si una firma HMAC-SHA256 es válida.
func VerifyHMAC(message, roomCode, signature string) bool {
	expected := GenerateHMAC(message, roomCode)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// GetLocalIP intenta encontrar la dirección IP local del dispositivo en la LAN.
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		// Verificar si es una dirección IP y no es loopback
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no se encontró una dirección IP local")
}

// MapIntToInt32 convierte un mapa de string:int a string:int32 (común en proto).
func MapIntToInt32(in map[string]int) map[string]int32 {
	out := make(map[string]int32)
	for k, v := range in {
		out[k] = int32(v)
	}
	return out
}
