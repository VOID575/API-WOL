package system

type ShutdownPayload struct {
	IP   string `json:"ip" binding:"required"`
	Port string `json:"port"`
	User string `json:"user" binding:"required"`
	OS   string `json:"os" binding:"required"`
}
