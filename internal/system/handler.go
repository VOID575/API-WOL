package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ShutdownMachine(context *gin.Context) {
	var shutdownPayload ShutdownPayload

	if err := context.BindJSON(&shutdownPayload); err != nil {
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
