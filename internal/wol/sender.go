package wol

import (
	"net"
	"strings"

	extwol "github.com/mdlayher/wol"
)

const defaultWOLPort = "9"

func normalizeBroadcastAddress(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}

	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		cleanHost := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
		return net.JoinHostPort(cleanHost, defaultWOLPort)
	}
	if port == "" && host != "" {
		return net.JoinHostPort(host, defaultWOLPort)
	}

	return trimmed
}

func SendMagicPacket(broadcastAddress string, macAddress net.HardwareAddr) error {
	client, err := extwol.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	targetAddr := normalizeBroadcastAddress(broadcastAddress)
	if err := client.Wake(targetAddr, macAddress); err != nil {
		return err
	}

	return nil
}
