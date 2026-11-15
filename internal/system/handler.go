package system

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func HealthCheck(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func GetMachineStatus(context *gin.Context) {
	ipParam := strings.TrimSpace(context.Param("ip"))
	parsedIP := net.ParseIP(ipParam)
	if parsedIP == nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "Invalid IP address"})
		return
	}

	status := "offline"
	if isMachineReachable(context.Request.Context(), parsedIP) {
		status = "online"
	}

	context.JSON(http.StatusOK, gin.H{"status": status})
}

func ShutdownMachine(context *gin.Context) {
	var shutdownPayload ShutdownPayload

	if err := context.ShouldBindJSON(&shutdownPayload); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Corps de requête invalide"})
		return
	}

	err := execShutdownCommand(shutdownPayload)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"message": "Échec de l'exécution: " + err.Error()})
		return
	}

	context.JSON(http.StatusOK, gin.H{"message": "Signal d'extinction transmis avec succès vers " + shutdownPayload.IP})
}
