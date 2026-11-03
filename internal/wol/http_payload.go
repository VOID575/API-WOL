package wol

type WolPayload struct {
	BroadcastAddress string `json:"broadcast_address"`
	MACAdress        string `json:"mac_address"`
}
