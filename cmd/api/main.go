package main

import (
	"log"
	"os"
	"strings"

	"api-wol/internal/system"
	"api-wol/internal/wol"

	"github.com/gin-gonic/gin"
)

const defaultListenAddr = "0.0.0.0:8080"

func main() {
	addr := strings.TrimSpace(os.Getenv("API_WOL_ADDR"))
	if addr == "" {
		addr = defaultListenAddr
	}

	router := gin.Default()

	router.POST("/wakeonlan", wol.WakeOnLan)
	router.POST("/shutdown", system.ShutdownMachine)
	router.GET("/status", system.HealthCheck)
	router.GET("/status/:ip", system.GetMachineStatus)

	if err := router.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
