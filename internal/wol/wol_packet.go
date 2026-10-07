package wol

type WolPacket struct {
	BroadcastAddress string `json:"broadcast_address"`
	MACAdress        string `json:"mac_address"`
}
