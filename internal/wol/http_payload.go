package wol

type WolPayload struct {
	BroadcastAddress string `json:"broadcast_address" binding:"required"`
	MACAdress        string `json:"mac_address" binding:"required"`
}
