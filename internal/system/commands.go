package system

import (
	"errors"
	"os/exec"
	"strings"
)

func execShutdownCommand(shutdownPayload ShutdownPayload) error {
	var cmdString string
	if strings.ToLower(shutdownPayload.OS) == "windows" {
		cmdString = windowsShutdownCommand
	} else {
		cmdString = linuxShutdownCommand
	}

	target := shutdownPayload.User + "@" + shutdownPayload.IP
	cmd := exec.Command("tailscale", "ssh", target, cmdString)

	err := cmd.Run()

	if err == nil {
		return nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 255 {
			return nil
		}

		return errors.New("Erreur SSH (Exit Code): " + exitErr.Error())
	}

	return errors.New("Erreur système locale: " + err.Error())
}
