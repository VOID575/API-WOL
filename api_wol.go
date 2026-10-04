package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mdlayher/wol"
)

func main() {
	router := gin.Default()
	router.GET("/wakeonlan", wakeOnLan)

	router.Run("localhost:8080")
}

func wakeOnLan(c *gin.Context) {

	var MACAddress string
	if err := c.ParseMAC(&MACAddress); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	if err := sendMagicPacket(MACAddress); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "Magic packet sent successfully"})
}

func sendMagicPacket(macAddress string) error {

	client, err := wol.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Wake(macAddress); err != nil {
		return err
	}

	return nil
}
