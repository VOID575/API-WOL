# API-WOL — Remote Wake-on-LAN & Shutdown API

A lightweight and fast REST API written in **Go** (using the [Gin](https://github.com/gin-gonic/gin) framework) designed to manage the power state of remote machines:
1. **Remote Wake-up (Wake-on-LAN)** via UDP Magic Packets broadcasted over the local area network.
2. **Clean Remote Shutdown** securely executed via **Tailscale SSH** for both **Linux** and **Windows** hosts.

---

## Table of Contents

- [Architecture & How It Works](#-architecture--how-it-works)
- [Prerequisites](#-prerequisites)
- [Tailscale Configuration (Tags, ACLs & SSH)](#-tailscale-configuration-tags-acls--ssh)
  - [1. Why Use Tailscale Tags?](#1-why-use-tailscale-tags)
  - [2. Defining Tags and Tailscale ACLs](#2-defining-tags-and-tailscale-acls)
  - [3. Enabling Tailscale SSH on Target Machines](#3-enabling-tailscale-ssh-on-target-machines)
  - [4. Shutdown Permissions on Targets](#4-shutdown-permissions-on-targets)
- [Installation & Quick Start](#-installation--quick-start)
  - [Build and Run](#build-and-run)
  - [Network Binding Note (`localhost` vs Tailscale)](#network-binding-note-localhost-vs-tailscale)
  - [Deployment with `systemd`](#deployment-with-systemd)
- [API Reference](#-api-reference)
  - [1. Wake a Machine (`POST /wakeonlan`)](#1-wake-a-machine-post-wakeonlan)
  - [2. Shut Down a Machine (`POST /shutdown`)](#2-shut-down-a-machine-post-shutdown)
- [Project Structure](#-project-structure)
- [Troubleshooting & FAQ](#-troubleshooting--faq)

---

## Architecture & How It Works

```
                                      +---------------------------------------------+
                                      |            Local Area Network (LAN)         |
                                      |                                             |
                         [Magic Packet]     +--------------------------------+      |
                     +--------------------> | Target Machine (Sleeping/Off)  |      |
                     |   (UDP Broadcast)    | MAC: AA:BB:CC:DD:EE:FF         |      |
                     |                      +--------------------------------+      |
+-------------+      |                                                              |
| HTTP Client | ---> | [ API-WOL (Go / Gin) ]                                       |
+-------------+      | (Tailscale Host)                                             |
                     |                                                              |
                     +--------------------------------------------------------------+
                     |                     Tailnet (Tailscale)                      |
                     |                                                              |
                     | [tailscale ssh]      +--------------------------------+      |
                     +--------------------> | Target Machine (Powered on)    |      |
                                            | IP: 100.x.y.z (Linux / Windows)|      |
                                            +--------------------------------+      |
                                      +---------------------------------------------+
```

### How Does It Work?
- **Wake-on-LAN (`/wakeonlan`)**: The API builds a magic packet consisting of 6 `0xFF` bytes followed by 16 repetitions of the target MAC address, and broadcasts it over UDP to the specified network subnet using `github.com/mdlayher/wol`. The API host machine must be physically or logically connected to the same local network (broadcast domain) as the target machine.
- **Shutdown (`/shutdown`)**: The API executes a remote SSH command via the Tailscale CLI (`tailscale ssh <user>@<ip> <command>`). Depending on the specified target OS:
  - **Linux**: `sudo shutdown -h now`
  - **Windows**: `shutdown /s /t 0 /f`
  The API explicitly handles SSH exit code `255` (which occurs when the target machine cuts the SSH connection abruptly during immediate shutdown) and treats it as a successful completion.

---

## Prerequisites

1. **API Host Machine**:
   - [Go](https://go.dev/) 1.22+ (tested with Go 1.26).
   - [Tailscale](https://tailscale.com/) installed, authenticated, and running.
   - Connected to the local Ethernet/Wi-Fi subnet of the target machines for Wake-on-LAN.
2. **Target Machines**:
   - **For Wake-on-LAN**: Motherboard and Network Interface Card (NIC) with Wake-on-LAN enabled in BIOS/UEFI and OS power management settings.
   - **For Shutdown**: Tailscale installed and running with Tailscale SSH enabled (`--ssh`).

---

## Tailscale Configuration (Tags, ACLs & SSH)

Tailscale SSH enables secure authentication and authorization through Tailnet identities without manually managing SSH key pairs.

### 1. Why Use Tailscale Tags?

By default, Tailscale nodes are tied to individual user accounts and their session keys expire periodically. By assigning **Tags** (e.g. `tag:api-wol` and `tag:targets`):
- The API host becomes an impersonal system node (*tagged node*).
- Authentication keys associated with tags do not expire.
- You can establish granular, role-based access rules in the ACL policy.

### 2. Defining Tags and Tailscale ACLs

Open your Tailscale admin console (**Admin Console > Access Controls**) and update your HuJSON policy:

```jsonc
{
  // 1. Define tag ownership
  "tagOwners": {
    "tag:api-wol": ["autogroup:admin"],
    "tag:targets": ["autogroup:admin"]
  },

  // 2. Standard network ACLs (allow API host to reach target machines)
  "acls": [
    {
      "action": "accept",
      "src": ["tag:api-wol"],
      "dst": ["tag:targets:*"]
    }
  ],

  // 3. Tailscale SSH Access Rules
  "ssh": [
    {
      "action": "accept",
      "src": ["tag:api-wol"],
      "dst": ["tag:targets"],
      "users": ["root", "valentin", "autogroup:nonroot"],
      "checkPeriod": "0s" // Disables interactive check prompt (essential for headless API calls)
    }
  ]
}
```

> [!IMPORTANT]
> The setting `"checkPeriod": "0s"` (or direct `"accept"` action) is crucial: it prevents Tailscale from asking for interactive browser verification URLs, which would block automated HTTP requests.

#### Assigning Tags to Machines

- **On the API Host Machine**:
  ```bash
  sudo tailscale up --advertise-tags=tag:api-wol
  ```
- **On Target Machines (to be shut down)**:
  ```bash
  sudo tailscale up --advertise-tags=tag:targets --ssh
  ```

---

### 3. Enabling Tailscale SSH on Target Machines

For targets to accept incoming connections handled by Tailscale SSH, they must start with the `--ssh` flag:

```bash
sudo tailscale up --ssh --accept-routes
```

Verify the status on the target machine:
```bash
tailscale status
```

---

### 4. Shutdown Permissions on Targets

#### For Linux Targets
The shutdown command executed is `sudo shutdown -h now`. The specified remote user must have permissions to run `shutdown` without password prompts.

Create a sudoers drop-in file on the target Linux system (e.g., `/etc/sudoers.d/tailscale-shutdown`):
```bash
echo "valentin ALL=(ALL) NOPASSWD: /sbin/shutdown, /usr/sbin/shutdown" | sudo tee /etc/sudoers.d/tailscale-shutdown
sudo chmod 0440 /etc/sudoers.d/tailscale-shutdown
```
*(Replace `valentin` with the username you provide in your API request).*

#### For Windows Targets
The command executed is `shutdown /s /t 0 /f`.
- The user specified in the API payload must be a member of the local **Administrators** group or have the `SeShutdownPrivilege` policy assigned.
- Tailscale SSH on Windows will run under this local user account.

---

## Installation & Quick Start

### Build and Run

1. **Clone the repository and download dependencies**:
   ```bash
   git clone <repo-url>
   cd API-WOL
   go mod download
   ```

2. **Run in development mode**:
   ```bash
   go run cmd/api/main.go
   ```

3. **Compile a production binary**:
   ```bash
   go build -o api-wol cmd/api/main.go
   ./api-wol
   ```

### Network Binding Note (`localhost` vs Tailscale)

By default, `cmd/api/main.go` binds to `localhost:8080`:
```go
router.Run("localhost:8080")
```
If you wish to access this API from other devices in your Tailnet or local network, update the bind address to listen on all interfaces:
```go
router.Run("0.0.0.0:8080")
```
or bind specifically to your machine's Tailscale IP address (e.g., `100.x.y.z:8080`).

---

### Deployment with `systemd`

To run the API as a background daemon that automatically starts on system boot:

1. Create a service file `/etc/systemd/system/api-wol.service`:
   ```ini
   [Unit]
   Description=API-WOL Service
   After=network.target tailscaled.service
   Wants=tailscaled.service

   [Service]
   Type=simple
   User=valentin
   WorkingDirectory=/home/valentin/linux_extension/API-WOL
   ExecStart=/home/valentin/linux_extension/API-WOL/api-wol
   Restart=always
   RestartSec=5
   Environment=GIN_MODE=release

   [Install]
   WantedBy=multi-user.target
   ```

2. Reload `systemd` and enable the service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now api-wol.service
   sudo systemctl status api-wol.service
   ```

---

## API Reference

### 1. Wake a Machine (`POST /wakeonlan`)

Broadcasts a Wake-on-LAN Magic Packet to wake up a sleeping or powered-off machine.

- **Method**: `POST`
- **Path**: `/wakeonlan`
- **Headers**: `Content-Type: application/json`

#### Request Body Parameters (JSON)

| Field | Type | Required | Description | Example |
| :--- | :--- | :---: | :--- | :--- |
| `broadcast_address` | String | Yes | Subnet broadcast IP address including UDP port (usually port 9 or 7) | `"192.168.1.255:9"` |
| `mac_address` | String | Yes | Target machine MAC address (separated by `:` or `-`) | `"AA:BB:CC:DD:EE:FF"` |

> [!NOTE]
> Make sure to specify the port in `broadcast_address` (e.g. `:9`). If your network uses a `255.255.255.0` netmask on `192.168.1.0/24`, the broadcast address is `192.168.1.255:9`. For global broadcast (if supported by your router), use `255.255.255.255:9`.

#### cURL Example

```bash
curl -X POST http://localhost:8080/wakeonlan \
  -H "Content-Type: application/json" \
  -d '{
    "broadcast_address": "192.168.1.255:9",
    "mac_address": "00:11:22:33:44:55"
  }'
```

#### PowerShell Example

```powershell
$body = @{
    broadcast_address = "192.168.1.255:9"
    mac_address = "00:11:22:33:44:55"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/wakeonlan" -Method POST -ContentType "application/json" -Body $body
```

#### Responses

- **`200 OK`**: Magic packet successfully sent.
  ```json
  {
    "message": "Magic packet sent successfully",
    "target": "00:11:22:33:44:55"
  }
  ```
- **`400 Bad Request`**: Invalid MAC address or malformed JSON payload.
  ```json
  {
    "message": "Invalid request body"
  }
  ```
- **`500 Internal Server Error`**: Socket initialization error or network failure.
  ```json
  {
    "message": "Error initializing the network socket"
  }
  ```

---

### 2. Shut Down a Machine (`POST /shutdown`)

Executes a remote power-off command on a target host via Tailscale SSH.

- **Method**: `POST`
- **Path**: `/shutdown`
- **Headers**: `Content-Type: application/json`

#### Request Body Parameters (JSON)

| Field | Type | Required | Description | Examples |
| :--- | :--- | :---: | :--- | :--- |
| `ip` | String | Yes | Target Tailscale IP (100.x.y.z) or MagicDNS hostname | `"100.85.120.45"` or `"my-pc.tailnet-xyz.ts.net"` |
| `port` | String | Yes | SSH port (e.g. `"22"`) | `"22"` |
| `user` | String | Yes | Remote target user account name | `"valentin"`, `"root"`, `"admin"` |
| `os` | String | Yes | Target operating system (`"linux"` or `"windows"`) | `"linux"`, `"windows"` |

#### cURL Example (Linux Target)

```bash
curl -X POST http://localhost:8080/shutdown \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "100.85.120.45",
    "port": "22",
    "user": "valentin",
    "os": "linux"
  }'
```

#### cURL Example (Windows Target)

```bash
curl -X POST http://localhost:8080/shutdown \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "100.85.120.90",
    "port": "22",
    "user": "Administrator",
    "os": "windows"
  }'
```

#### Responses

- **`200 OK`**: Shutdown signal successfully sent.
  ```json
  {
    "message": "Signal d'extinction transmis avec succès vers 100.85.120.45"
  }
  ```
- **`400 Bad Request`**: Missing required fields or malformed JSON payload.
  ```json
  {
    "message": "Corps de requête invalide"
  }
  ```
- **`500 Internal Server Error`**: Execution failure when calling Tailscale SSH.
  ```json
  {
    "message": "Échec de l'exécution: Erreur SSH (Exit Code): exit status 1"
  }
  ```

---

## Project Structure

```
API-WOL/
├── cmd/
│   └── api/
│       └── main.go                 # Entry point, Gin router setup & HTTP server
├── internal/
│   ├── system/
│   │   ├── commands.go             # Tailscale SSH command execution logic
│   │   ├── constants.go            # Linux and Windows shutdown command strings
│   │   ├── handler.go              # HTTP handler for POST /shutdown
│   │   └── http_payload.go         # ShutdownPayload struct definition
│   └── wol/
│       ├── handler.go              # HTTP handler for POST /wakeonlan
│       ├── http_payload.go         # WolPayload struct definition
│       └── sender.go               # UDP Magic Packet sender wrapper
├── go.mod                          # Go module dependencies
├── go.sum                          # Module checksums
└── README.md                       # Project documentation
```

---

## 🛠 Troubleshooting & FAQ

### 1. The target machine doesn't wake up after calling `/wakeonlan`
- **BIOS/UEFI Configuration**: Verify that *Wake on LAN* (or *Power On By PCI-E Device*) is enabled in the BIOS/UEFI settings.
- **Windows Power Settings**: Disable *Fast Startup* in Windows Control Panel. Fast Startup puts network adapters into an ultra-low power hibernation state where they might not listen for Magic Packets.
- **Broadcast Subnet**: The API server and the target device must share the same local broadcast domain. Standard routers discard directed UDP broadcast packets crossing subnets unless explicitly configured.
- **Address Format**: Ensure you included the port in `broadcast_address` (e.g. `192.168.1.255:9`).

### 2. Shutdown fails with an SSH error
- **Tailscale Connectivity**: From the API server terminal, test manual connectivity:
  ```bash
  tailscale ssh <user>@<ip>
  ```
- **Password Prompt (Linux)**: If `sudo shutdown` requests a password, ensure the `NOPASSWD` rule is properly configured in `/etc/sudoers.d/tailscale-shutdown`.
- **Tailscale ACL Policy**: Confirm that the Tailscale SSH rule specifies `"action": "accept"` with `"checkPeriod": "0s"` to prevent interactive validation prompts.

### 3. API is unreachable from other Tailnet devices
If the server starts with `localhost:8080`, it only listens on local loopback. Change the bind string in `cmd/api/main.go` to `0.0.0.0:8080` to accept connections across all network interfaces, including Tailscale.
