package system

const (
	Windows = "windows"
	Linux   = "linux"
)

const (
	windowsShutdownCommand = "shutdown /s /t 0 /f"
	linuxShutdownCommand   = "sudo shutdown -h now"
)
