package system

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strings"
	"time"
)

const reachabilityProbeTimeout = 2500 * time.Millisecond

var tailscaleCGNATNet = &net.IPNet{
	IP:   net.IPv4(100, 64, 0, 0).To4(),
	Mask: net.CIDRMask(10, 32),
}

func isTailscaleIP(ip net.IP) bool {
	return ip != nil && tailscaleCGNATNet.Contains(ip)
}

func runReachabilityProbe(ctx context.Context, name string, args ...string) bool {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run() == nil
}

func isMachineReachable(ctx context.Context, ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	probeCtx, cancel := context.WithTimeout(ctx, reachabilityProbeTimeout)
	defer cancel()

	targetIP := ip.String()
	if !isTailscaleIP(ip) {
		return runReachabilityProbe(probeCtx, "ping", "-c", "1", "-W", "1", targetIP)
	}

	probes := [][]string{
		{"tailscale", "ping", "--c=1", "--timeout=2s", targetIP},
		{"ping", "-c", "1", "-W", "1", targetIP},
	}

	results := make(chan bool, len(probes))
	for _, probeArgs := range probes {
		go func(args []string) {
			results <- runReachabilityProbe(probeCtx, args[0], args[1:]...)
		}(probeArgs)
	}

	for range probes {
		if <-results {
			cancel()
			return true
		}
	}

	return false
}

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
