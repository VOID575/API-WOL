package wol

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func WakeOnLan(context *gin.Context) {
	var wolPayload WolPayload

	if err := context.ShouldBindJSON(&wolPayload); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	if strings.TrimSpace(wolPayload.BroadcastAddress) == "" {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	parsedMACAdress, err := net.ParseMAC(strings.TrimSpace(wolPayload.MACAdress))
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	if err := SendMagicPacket(wolPayload.BroadcastAddress, parsedMACAdress); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	context.IndentedJSON(http.StatusOK,
		gin.H{
			"message": "Magic packet sent successfully",
			"target":  parsedMACAdress.String(),
		})
}
