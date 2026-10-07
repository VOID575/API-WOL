package system

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

func ShutdownMachine(context *gin.Context) {
	var shutdownPayload ShutdownPayload

	if err := context.BindJSON(&shutdownPayload); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Corps de requête invalide"})
		return
	}

	var cmdString string
	if strings.ToLower(shutdownPayload.OS) == "windows" {
		cmdString = windowsShutdownCommand
	} else {
		cmdString = linuxShutdownCommand
	}

	target := shutdownPayload.User + "@" + shutdownPayload.IP
	cmd := exec.Command("tailscale", "ssh", target, cmdString)

	// Exécution bloquante
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if (err != nil) && ok {
		// While connected in ssh to a Windows machine, the command "shutdown /s /t 0 /f" will return an exit code of 255,
		// which is expected and indicates that the shutdown signal was sent successfully.
		if exitErr.ExitCode() == 255 {
			context.JSON(http.StatusOK, gin.H{
				"message": "Signal envoyé avec succès (déconnexion attendue confirmée)",
				"target":  target,
			})
		}
	} else if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"message": "Échec de l'exécution: " + err.Error()})
	} else {
		context.JSON(http.StatusOK, gin.H{"message": "Signal d'extinction transmis avec succès vers " + shutdownPayload.IP})
	}
}
