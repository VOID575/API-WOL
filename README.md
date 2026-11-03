# API-WOL — Remote Wake-on-LAN & Shutdown API

API REST légère et performante développée en **Go** (avec le framework [Gin](https://github.com/gin-gonic/gin)), permettant d'administrer l'alimentation de machines distantes :
1. **Réveil à distance (Wake-on-LAN)** via l'envoi de Magic Packets UDP sur le réseau local.
2. **Extinction propre à distance (Shutdown)** sécurisée à travers **Tailscale SSH** pour les environnements **Linux** et **Windows**.

---

## Sommaire

- [Architecture et Fonctionnement](#-architecture-et-fonctionnement)
- [Prérequis](#-prérequis)
- [Configuration Tailscale (Tags, ACLs et SSH)](#-configuration-tailscale-tags-acls-et-ssh)
  - [1. Pourquoi utiliser des Tags Tailscale ?](#1-pourquoi-utiliser-des-tags-tailscale-)
  - [2. Déclaration des Tags et ACLs Tailscale](#2-déclaration-des-tags-et-acls-tailscale)
  - [3. Activation de Tailscale SSH sur les machines cibles](#3-activation-de-tailscale-ssh-sur-les-machines-cibles)
  - [4. Permissions d'extinction sur les cibles](#4-permissions-dextinction-sur-les-cibles)
- [Installation et Démarrage](#-installation-et-démarrage)
  - [Compilation et exécution](#compilation-et-exécution)
  - [Note sur l'adresse d'écoute (`localhost` vs Tailscale)](#note-sur-ladresse-découte-localhost-vs-tailscale)
  - [Déploiement en service `systemd`](#déploiement-en-service-systemd)
- [Référence des Appels API](#-référence-des-appels-api)
  - [1. Réveiller une machine (`POST /wakeonlan`)](#1-réveiller-une-machine-post-wakeonlan)
  - [2. Éteindre une machine (`POST /shutdown`)](#2-éteindre-une-machine-post-shutdown)
- [Structure du Projet](#-structure-du-projet)
- [Dépannage & FAQ](#-dépannage--faq)

---

## Architecture et Fonctionnement

```
                                      +---------------------------------------------+
                                      |            Réseau Local (LAN)               |
                                      |                                             |
                         [Magic Packet]     +--------------------------------+      |
                     +--------------------> | Machine Cible (Éteinte en veille)     |
                     |   (UDP Broadcast)    | MAC: AA:BB:CC:DD:EE:FF         |      |
                     |                      +--------------------------------+      |
+-------------+      |                                                              |
| Client HTTP | ---> | [ API-WOL (Go / Gin) ]                                       |
+-------------+      | (Hôte Tailscale)                                             |
                     |                                                              |
                     +--------------------------------------------------------------+
                     |                     Tailnet (Tailscale)                      |
                     |                                                              |
                     | [tailscale ssh]      +--------------------------------+      |
                     +--------------------> | Machine Cible (Allumée)        |      |
                                            | IP: 100.x.y.z (Linux / Windows)|      |
                                            +--------------------------------+      |
                                      +---------------------------------------------+
```

### Comment ça marche ?
- **Wake-on-LAN (`/wakeonlan`)** : L'API génère un paquet magique constitué de 6 octets `0xFF` suivis de 16 répétitions de l'adresse MAC cible, puis l'émet en UDP broadcast sur le sous-réseau local via la bibliothèque `github.com/mdlayher/wol`. La machine hôte de l'API doit être connectée physiquement ou logiquement au même réseau local (broadcast domain) que la machine à réveiller.
- **Extinction (`/shutdown`)** : L'API déclenche une commande SSH via la CLI Tailscale (`tailscale ssh <user>@<ip> <command>`). Selon l'OS spécifié dans la requête :
  - **Linux** : `sudo shutdown -h now`
  - **Windows** : `shutdown /s /t 0 /f`
  L'API gère le code de retour SSH `255` (qui se produit naturellement lorsque la machine cible coupe la connexion réseau lors de son extinction immédiate) et le traite comme un succès.

---

## Prérequis

1. **Serveur hébergeant l'API** :
   - [Go](https://go.dev/) 1.22 ou supérieur (le projet utilise Go 1.26).
   - [Tailscale](https://tailscale.com/) installé, connecté au tailnet et fonctionnel en ligne de commande.
   - Présence sur le réseau local Ethernet/Wi-Fi des cibles pour le Wake-on-LAN.
2. **Machines cibles** :
   - **Pour Wake-on-LAN** : Carte mère et carte réseau configurées avec le WoL activé dans le BIOS/UEFI et dans l'OS (paramètres d'économie d'énergie).
   - **Pour Shutdown** : Client Tailscale actif avec Tailscale SSH activé (`--ssh`).

---

## Configuration Tailscale (Tags, ACLs et SSH)

Tailscale SSH permet d'authentifier et d'autoriser les connexions SSH en utilisant les identités de votre Tailnet, sans avoir à gérer manuellement de clés SSH publiques/privées.

### 1. Pourquoi utiliser des Tags Tailscale ?

Par défaut, les machines Tailscale sont associées à un utilisateur individuel et leurs clés de session expirent périodiquement. En utilisant des **Tags** (ex. `tag:api-wol` et `tag:targets`) :
- La machine hébergeant l'API devient un nœud système impersonnel (*tagged node*).
- Les clés d'authentification associées aux tags n'expirent pas.
- Vous définissez des règles d'accès strictes et granulaires dans la politique ACL.

### 2. Déclaration des Tags et ACLs Tailscale

Rendez-vous dans la console d'administration Tailscale (**Admin Console > Access Controls**) et configurez votre fichier de configuration de politique (HuJSON) :

```jsonc
{
  // 1. Définition des propriétaires des tags
  "tagOwners": {
    "tag:api-wol": ["autogroup:admin"],
    "tag:targets": ["autogroup:admin"]
  },

  // 2. Règles réseau standard (autoriser l'API à joindre les cibles)
  "acls": [
    {
      "action": "accept",
      "src": ["tag:api-wol"],
      "dst": ["tag:targets:*"]
    }
  ],

  // 3. Règles Tailscale SSH
  "ssh": [
    {
      "action": "accept",
      "src": ["tag:api-wol"],
      "dst": ["tag:targets"],
      "users": ["root", "user", "autogroup:nonroot"],
      "checkPeriod": "0s" // Pas de ré-authentification interactive (requis pour un appel d'API)
    }
  ]
}
```

> [!IMPORTANT]
> L'instruction `"checkPeriod": "0s"` (ou l'action `"accept"` directe) est cruciale : elle garantit que la connexion SSH ne demandera pas de confirmation via navigateur ou URL de validation, ce qui bloquerait l'appel HTTP de l'API.

#### Assigner les tags aux machines

- **Sur la machine hôte de l'API** :
  ```bash
  sudo tailscale up --advertise-tags=tag:api-wol
  ```
- **Sur les machines cibles (à éteindre)** :
  ```bash
  sudo tailscale up --advertise-tags=tag:targets --ssh
  ```

---

### 3. Activation de Tailscale SSH sur les machines cibles

Pour que la machine accepte les connexions gérées par Tailscale SSH, elle doit être démarrée avec le flag `--ssh` :

```bash
sudo tailscale up --ssh --accept-routes
```

Vérifiez l'état du démon sur la machine cible :
```bash
tailscale status
```

---

### 4. Permissions d'extinction sur les cibles

#### Pour une cible Linux
La commande exécutée est `sudo shutdown -h now`. L'utilisateur configuré dans la requête doit pouvoir lancer `shutdown` sans mot de passe.

Créez une règle sudoers dédiée sur la cible Linux (ex. `/etc/sudoers.d/tailscale-shutdown`) :
```bash
echo "user ALL=(ALL) NOPASSWD: /sbin/shutdown, /usr/sbin/shutdown" | sudo tee /etc/sudoers.d/tailscale-shutdown
sudo chmod 0440 /etc/sudoers.d/tailscale-shutdown
```
*(Remplacez `user` par l'utilisateur que vous spécifierez dans le JSON de l'API).*

#### Pour une cible Windows
La commande exécutée est `shutdown /s /t 0 /f`.
- L'utilisateur spécifié dans la requête doit être un compte membre du groupe **Administrateurs** local ou disposer du privilège `SeShutdownPrivilege`.
- Tailscale SSH sous Windows se connectera sous ce compte local.

---

## Installation et Démarrage

### Compilation et exécution

1. **Cloner le projet et installer les dépendances** :
   ```bash
   git clone <url-du-repo>
   cd API-WOL
   go mod download
   ```

2. **Lancement en mode développement** :
   ```bash
   go run cmd/api/main.go
   ```

3. **Compilation d'un binaire de production** :
   ```bash
   go build -o api-wol cmd/api/main.go
   ./api-wol
   ```

### Note sur l'adresse d'écoute (`localhost` vs Tailscale)

Par défaut, `cmd/api/main.go` écoute sur `localhost:8080` :
```go
router.Run("localhost:8080")
```
Si vous souhaitez appeler cette API depuis d'autres appareils connectés à votre Tailnet ou votre réseau local, modifiez cette ligne pour écouter sur toutes les interfaces :
```go
router.Run("0.0.0.0:8080")
```
ou spécifiquement sur l'adresse IP Tailscale du serveur (ex. `100.x.y.z:8080`).

---

### Déploiement en service `systemd`

Pour maintenir l'API active en arrière-plan et la relancer automatiquement au démarrage du système :

1. Créez le fichier de service `/etc/systemd/system/api-wol.service` :
   ```ini
   [Unit]
   Description=API-WOL Service
   After=network.target tailscaled.service
   Wants=tailscaled.service

   [Service]
   Type=simple
   User=user
   WorkingDirectory=/home/user/linux_extension/API-WOL
   ExecStart=/home/user/linux_extension/API-WOL/api-wol
   Restart=always
   RestartSec=5
   Environment=GIN_MODE=release

   [Install]
   WantedBy=multi-user.target
   ```

2. Activez et démarrez le service :
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now api-wol.service
   sudo systemctl status api-wol.service
   ```

---

## Référence des Appels API

### 1. Réveiller une machine (`POST /wakeonlan`)

Émet un paquet magique Wake-on-LAN sur le réseau broadcast vers l'adresse MAC cible.

- **Méthode** : `POST`
- **Route** : `/wakeonlan`
- **Headers** : `Content-Type: application/json`

#### Paramètres du corps de requête (JSON)

| Champ | Type | Obligatoire | Description | Exemple |
| :--- | :--- | :---: | :--- | :--- |
| `broadcast_address` | String | Oui | Adresse IP de broadcast du sous-réseau avec le port UDP (généralement port 9 ou 7) | `"192.168.1.255:9"` |
| `mac_address` | String | Oui | Adresse MAC de la carte réseau de la machine cible (séparée par des `:` ou `-`) | `"AA:BB:CC:DD:EE:FF"` |

> [!NOTE]
> Veillez à bien spécifier le port dans `broadcast_address` (ex: `:9`). Si votre réseau utilise le masque `255.255.255.0` sur `192.168.1.0/24`, l'adresse de broadcast est `192.168.1.255:9`. Pour du broadcast global (si supporté par votre routeur), vous pouvez utiliser `255.255.255.255:9`.

#### Exemple d'appel cURL

```bash
curl -X POST http://localhost:8080/wakeonlan \
  -H "Content-Type: application/json" \
  -d '{
    "broadcast_address": "192.168.1.255:9",
    "mac_address": "00:11:22:33:44:55"
  }'
```

#### Exemple PowerShell

```powershell
$body = @{
    broadcast_address = "192.168.1.255:9"
    mac_address = "00:11:22:33:44:55"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/wakeonlan" -Method POST -ContentType "application/json" -Body $body
```

#### Réponses possibles

- **`200 OK`** : Paquet envoyé avec succès.
  ```json
  {
    "message": "Magic packet sent successfully",
    "target": "00:11:22:33:44:55"
  }
  ```
- **`400 Bad Request`** : Adresse MAC invalide ou payload JSON incorrect.
  ```json
  {
    "message": "Invalid request body"
  }
  ```
- **`500 Internal Server Error`** : Erreur réseau lors de l'ouverture du socket ou de l'envoi du paquet.
  ```json
  {
    "message": "Error initializing the network socket"
  }
  ```

---

### 2. Éteindre une machine (`POST /shutdown`)

Ordonne l'extinction immédiate d'une machine cible via Tailscale SSH.

- **Méthode** : `POST`
- **Route** : `/shutdown`
- **Headers** : `Content-Type: application/json`

#### Paramètres du corps de requête (JSON)

| Champ | Type | Obligatoire | Description | Exemples |
| :--- | :--- | :---: | :--- | :--- |
| `ip` | String | Oui | Adresse IP Tailscale (100.x.y.z) ou MagicDNS de la machine cible | `"100.85.120.45"` ou `"mon-pc.tailnet-xyz.ts.net"` |
| `port` | String | Oui | Port SSH (ex. `"22"`) | `"22"` |
| `user` | String | Oui | Nom d'utilisateur sur la machine distante | `"user"`, `"root"`, `"admin"` |
| `os` | String | Oui | Système d'exploitation cible (`"linux"` ou `"windows"`) | `"linux"`, `"windows"` |

#### Exemple d'appel cURL (Machine Linux)

```bash
curl -X POST http://localhost:8080/shutdown \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "100.85.120.45",
    "port": "22",
    "user": "user",
    "os": "linux"
  }'
```

#### Exemple d'appel cURL (Machine Windows)

```bash
curl -X POST http://localhost:8080/shutdown \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "100.85.120.90",
    "port": "22",
    "user": "Administrateur",
    "os": "windows"
  }'
```

#### Réponses possibles

- **`200 OK`** : Ordre d'extinction transmis avec succès.
  ```json
  {
    "message": "Signal d'extinction transmis avec succès vers 100.85.120.45"
  }
  ```
- **`400 Bad Request`** : Corps de requête manquant ou invalide.
  ```json
  {
    "message": "Corps de requête invalide"
  }
  ```
- **`500 Internal Server Error`** : Échec lors de l'exécution de la commande SSH.
  ```json
  {
    "message": "Échec de l'exécution: Erreur SSH (Exit Code): exit status 1"
  }
  ```

---

## Structure du Projet

```
API-WOL/
├── cmd/
│   └── api/
│       └── main.go                 # Point d'entrée, initialisation du routeur Gin & serveur HTTP
├── internal/
│   ├── system/
│   │   ├── commands.go             # Logique d'exécution de la commande tailscale ssh
│   │   ├── constants.go            # Commandes système Linux/Windows
│   │   ├── handler.go              # Contrôleur HTTP pour POST /shutdown
│   │   └── http_payload.go         # Structure du payload ShutdownPayload
│   └── wol/
│       ├── handler.go              # Contrôleur HTTP pour POST /wakeonlan
│       ├── http_payload.go         # Structure du payload WolPayload
│       └── sender.go               # Wrapper d'envoi du paquet magique UDP
├── go.mod                          # Dépendances du module Go
├── go.sum                          # Checksums des dépendances
└── README.md                       # Documentation du projet
```

---

## Dépannage & FAQ

### 1. La machine ne se réveille pas après un appel `/wakeonlan`
- **Vérification du BIOS/UEFI** : Assurez-vous que l'option *Wake on LAN* (ou *Power On By PCI-E Device*) est activée.
- **Paramètres OS (Windows)** : Désactivez le démarrage rapide (*Fast Startup*), car il place la carte réseau dans un état d'hibernation profonde où elle n'écoute plus les Magic Packets.
- **Sous-réseau de broadcast** : L'API et la machine cible doivent être sur le même réseau local. Le broadcast UDP ne traverse généralement pas les routeurs sans configuration spécifique (ip directed-broadcast).
- **Format de l'adresse** : Assurez-vous d'avoir fourni l'IP de broadcast terminée par le port (ex. `192.168.1.255:9`).

### 2. Le shutdown échoue avec une erreur SSH
- **Vérification de la connectivité Tailscale** : Depuis le serveur de l'API, testez manuellement la commande :
  ```bash
  tailscale ssh <user>@<ip>
  ```
- **Demande de mot de passe (Linux)** : Si `sudo shutdown` demande un mot de passe, configurez `sudoers` avec `NOPASSWD` comme indiqué dans la section de configuration.
- **ACL Tailscale** : Vérifiez que l'action SSH dans la console d'administration est bien sur `"action": "accept"` sans check interactif (`checkPeriod: 0s`).

### 3. L'API est inaccessible depuis un autre appareil du Tailnet
Si l'API est démarrée avec `localhost:8080`, elle n'écoute que les requêtes venant de la même machine. Changez le bind dans `cmd/api/main.go` en `0.0.0.0:8080` pour la rendre accessible sur toutes les interfaces (y compris Tailscale).
