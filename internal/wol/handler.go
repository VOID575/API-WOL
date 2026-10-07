package wol

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	extwol "github.com/mdlayher/wol"
)

func WakeOnLan(context *gin.Context) {

	var wolPayload WolPayload

	if err := context.BindJSON(&wolPayload); err != nil {
		return
	}

	client, err := extwol.NewClient()
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"message": "Error initializing the network socket"})
		return
	}
	defer client.Close()

	parsedMACAdress, err := net.ParseMAC(wolPayload.MACAdress)
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
