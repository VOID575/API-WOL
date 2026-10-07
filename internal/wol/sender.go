package wol

import (
	"net"

	extwol "github.com/mdlayher/wol"
)

func SendMagicPacket(broadcastAddress string, macAddress net.HardwareAddr) error {

	client, err := extwol.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Wake(broadcastAddress, macAddress); err != nil {
		return err
	}

	return nil
}
