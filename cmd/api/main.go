package main

import (
	"api-wol/internal/wol"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.POST("/wakeonlan", wol.WakeOnLan)

	router.Run("localhost:8080")
}
