// ============================================================================
// FILE: usb/catalog/catalog.go
// PURPOSE: Generates production-ready `docker-compose.yml` files for every
//
//	self-hosted app in the catalog, including multi-container stacks
//	(Immich, Paperless-ngx, BookStack) and Minecraft Studio.
//
// HOW TO ADD CUSTOM DOCKER COMPOSE RULES FOR AN APP:
//   - Look at `BuildVerifiedAppComposeYAML()` below:
//   - If an app needs a specific internal container port (e.g. `8081:80`),
//     capabilities (`NET_ADMIN`), or extra environment variables, add a
//     `case "your_app_id":` block inside the `switch cleanID` statement.
//
// ============================================================================
package catalog

import (
	"fmt"
	"strings"
)

type AppOverride struct {
	HostPort    int               `json:"host_port,omitempty"`
	MemoryLimit string            `json:"memory_limit,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	ConfigFiles map[string]string `json:"config_files,omitempty"`
}

type AppSpec struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Category     string            `json:"category"`
	Image        string            `json:"image"`
	HostPort     int               `json:"port"`
	InternalPort int               `json:"internal_port"`
	UDPOnly      bool              `json:"udp_only,omitempty"`
	ExtraPorts   []string          `json:"extra_ports,omitempty"`
	CapAdd       []string          `json:"cap_add,omitempty"`
	Sysctls      []string          `json:"sysctls,omitempty"`
	Devices      []string          `json:"devices,omitempty"`
	ShmSize      string            `json:"shm_size,omitempty"`
	Command      string            `json:"command,omitempty"`
	Environment  map[string]string `json:"environment,omitempty"`
	Volumes      []string          `json:"volumes"`
	ConfigFiles  map[string]string `json:"config_files,omitempty"`
}

type MinecraftServerConfig struct {
	ServerType         string `json:"server_type"`         // PAPER, PURPUR, FABRIC, FORGE, NEOFORGE, VANILLA, GEYSER_FLOODGATE
	Version            string `json:"version"`             // 26.3, 26.2, 26.1, 1.21.11, 1.21.4, LATEST, etc.
	MemoryHeap         string `json:"memory_heap"`         // 2G, 4G, 6G, 8G, 12G, 16G
	UseAikarFlags      bool   `json:"use_aikar_flags"`     // -XX:+UseG1GC Aikar's flags
	Gamemode           string `json:"gamemode"`            // survival, creative, adventure
	Difficulty         string `json:"difficulty"`          // peaceful, easy, normal, hard
	OnlineMode         bool   `json:"online_mode"`         // true = Mojang auth, false = offline/LAN
	MaxPlayers         int    `json:"max_players"`         // default 20
	ViewDistance       int    `json:"view_distance"`       // default 10
	SimulationDistance int    `json:"simulation_distance"` // default 10
	LevelSeed          string `json:"level_seed"`          // optional world seed
	MOTD               string `json:"motd"`                // server list description
	OpsList            string `json:"ops_list"`            // comma-separated admin usernames
	Whitelist          string `json:"whitelist"`           // comma-separated allowed players
	ModrinthProjects   string `json:"modrinth_projects"`   // comma-separated Modrinth slugs (e.g. worldedit,lithium,bluemap)
	EnableRCON         bool   `json:"enable_rcon"`         // port 25575 inside docker / localhost
	RCONPassword       string `json:"rcon_password"`       // admin RCON password
	EnableBackups      bool   `json:"enable_backups"`      // itzg/mc-backup sidecar
}

func DefaultMinecraftConfig() MinecraftServerConfig {
	return MinecraftServerConfig{
		ServerType:         "PAPER",
		Version:            "26.3",
		MemoryHeap:         "4G",
		UseAikarFlags:      true,
		Gamemode:           "survival",
		Difficulty:         "normal",
		OnlineMode:         true,
		MaxPlayers:         20,
		ViewDistance:       10,
		SimulationDistance: 10,
		LevelSeed:          "",
		MOTD:               "Instant Server Minecraft World",
		OpsList:            "",
		Whitelist:          "",
		ModrinthProjects:   "viaversion,chunky",
		EnableRCON:         true,
		RCONPassword:       "instant-rcon-admin",
		EnableBackups:      true,
	}
}

func BuildMinecraftComposeYAML(mc MinecraftServerConfig) string {
	srvType := strings.ToUpper(strings.TrimSpace(mc.ServerType))
	if srvType == "" {
		srvType = "PAPER"
	}
	modrinth := strings.TrimSpace(mc.ModrinthProjects)
	extraPorts := ""
	if srvType == "GEYSER_FLOODGATE" {
		srvType = "PAPER"
		if modrinth == "" {
			modrinth = "geyser,floodgate,viaversion"
		} else if !strings.Contains(strings.ToLower(modrinth), "geyser") {
			modrinth += ",geyser,floodgate"
		}
		extraPorts = "\n      - \"19132:19132/udp\""
	}

	ver := strings.TrimSpace(mc.Version)
	if ver == "" {
		ver = "26.3"
	}
	heap := strings.ToUpper(strings.TrimSpace(mc.MemoryHeap))
	if heap == "" {
		heap = "4G"
	}
	mode := strings.ToLower(strings.TrimSpace(mc.Gamemode))
	if mode == "" {
		mode = "survival"
	}
	diff := strings.ToLower(strings.TrimSpace(mc.Difficulty))
	if diff == "" {
		diff = "normal"
	}
	maxP := mc.MaxPlayers
	if maxP <= 0 {
		maxP = 20
	}
	viewD := mc.ViewDistance
	if viewD < 4 || viewD > 32 {
		viewD = 10
	}
	simD := mc.SimulationDistance
	if simD < 4 || simD > 32 {
		simD = 10
	}
	rconPass := strings.TrimSpace(mc.RCONPassword)
	if rconPass == "" {
		rconPass = "instant-rcon-admin"
	}

	var envLines []string
	envLines = append(envLines,
		`      EULA: "TRUE"`,
		fmt.Sprintf(`      TYPE: "%s"`, srvType),
		fmt.Sprintf(`      VERSION: "%s"`, ver),
		fmt.Sprintf(`      MEMORY: "%s"`, heap),
		fmt.Sprintf(`      USE_AIKAR_FLAGS: "%t"`, mc.UseAikarFlags),
		fmt.Sprintf(`      MODE: "%s"`, mode),
		fmt.Sprintf(`      DIFFICULTY: "%s"`, diff),
		fmt.Sprintf(`      ONLINE_MODE: "%t"`, mc.OnlineMode),
		fmt.Sprintf(`      MAX_PLAYERS: "%d"`, maxP),
		fmt.Sprintf(`      VIEW_DISTANCE: "%d"`, viewD),
		fmt.Sprintf(`      SIMULATION_DISTANCE: "%d"`, simD),
		fmt.Sprintf(`      ENABLE_RCON: "%t"`, mc.EnableRCON),
		fmt.Sprintf(`      RCON_PASSWORD: "%s"`, rconPass),
		`      snooper_enabled: "false"`,
	)
	if strings.TrimSpace(mc.MOTD) != "" {
		envLines = append(envLines, fmt.Sprintf(`      MOTD: "%s"`, strings.ReplaceAll(mc.MOTD, `"`, `\"`)))
	}
	if strings.TrimSpace(mc.LevelSeed) != "" {
		envLines = append(envLines, fmt.Sprintf(`      SEED: "%s"`, strings.ReplaceAll(mc.LevelSeed, `"`, "")))
	}
	if strings.TrimSpace(mc.OpsList) != "" {
		envLines = append(envLines, fmt.Sprintf(`      OPS: "%s"`, strings.TrimSpace(mc.OpsList)))
	}
	if strings.TrimSpace(mc.Whitelist) != "" {
		envLines = append(envLines,
			`      ENABLE_WHITELIST: "true"`,
			`      ENFORCE_WHITELIST: "true"`,
			fmt.Sprintf(`      WHITELIST: "%s"`, strings.TrimSpace(mc.Whitelist)),
		)
	} else {
		// Minecraft 26.3 ("Wilderness Bound") enables white-list=true by default in new server.properties;
		// explicitly set WHITE_LIST=false unless the user specified a whitelist so LAN/friends aren't blocked.
		envLines = append(envLines,
			`      WHITE_LIST: "false"`,
			`      ENFORCE_WHITELIST: "false"`,
		)
	}
	if modrinth != "" {
		envLines = append(envLines, fmt.Sprintf(`      MODRINTH_PROJECTS: "%s"`, modrinth))
	}

	backupService := ""
	if mc.EnableBackups {
		backupService = fmt.Sprintf(`
  minecraft_backup:
    image: itzg/mc-backup:latest
    container_name: minecraft_backup
    restart: unless-stopped
    depends_on:
      - minecraft
    environment:
      RCON_HOST: "minecraft"
      RCON_PORT: "25575"
      RCON_PASSWORD: "%s"
      BACKUP_INTERVAL: "12h"
      PRUNE_BACKUPS_DAYS: "7"
      INITIAL_DELAY: "5m"
    volumes:
      - /DATA/AppData/minecraft/data:/data:ro
      - /DATA/Backups/minecraft:/backups
`, rconPass)
	}

	return fmt.Sprintf(`name: minecraft
services:
  minecraft:
    image: itzg/minecraft-server:latest
    container_name: minecraft
    restart: unless-stopped
    tty: true
    stdin_open: true
    security_opt:
      - no-new-privileges:true
    environment:
%s
    ports:
      - "25565:25565/tcp"
      - "127.0.0.1:25575:25575/tcp"%s
    volumes:
      - /DATA/AppData/minecraft/data:/data
%s`, strings.Join(envLines, "\n"), extraPorts, backupService)
}

var VerifiedCoreAppSpecs = map[string]AppSpec{
	"jellyfin": {
		ID: "jellyfin", Title: "Jellyfin Media Server", Category: "Media & Photos",
		Image: "jellyfin/jellyfin:latest", HostPort: 8096, InternalPort: 8096,
		Devices: []string{"/dev/dri:/dev/dri"},
		Volumes: []string{"/DATA/AppData/jellyfin/config:/config", "/DATA/Media:/media"},
	},
	"plex": {
		ID: "plex", Title: "Plex Media Server", Category: "Media & Photos",
		Image: "lscr.io/linuxserver/plex:latest", HostPort: 32400, InternalPort: 32400,
		Devices:     []string{"/dev/dri:/dev/dri"},
		Environment: map[string]string{"VERSION": "docker"},
		Volumes:     []string{"/DATA/AppData/plex/config:/config", "/DATA/Media:/media"},
	},
	"emby": {
		ID: "emby", Title: "Emby Media Server", Category: "Media & Photos",
		Image: "lscr.io/linuxserver/emby:latest", HostPort: 8097, InternalPort: 8096,
		Devices: []string{"/dev/dri:/dev/dri"},
		Volumes: []string{"/DATA/AppData/emby/config:/config", "/DATA/Media:/media"},
	},
	"nextcloud": {
		ID: "nextcloud", Title: "Nextcloud Personal Cloud", Category: "Cloud & Files",
		Image: "nextcloud:stable", HostPort: 8081, InternalPort: 80,
		Environment: map[string]string{
			"SQLITE_DATABASE":           "nextcloud",
			"NEXTCLOUD_ADMIN_USER":      "admin",
			"NEXTCLOUD_ADMIN_PASSWORD":  "admin",
			"NEXTCLOUD_TRUSTED_DOMAINS": "localhost instant-server.local *.local 192.168.*.* 10.*.*.* 100.*.*.*",
		},
		Volumes: []string{"/DATA/AppData/nextcloud/html:/var/www/html", "/DATA/AppData/nextcloud/data:/var/www/html/data"},
	},
	"pihole": {
		ID: "pihole", Title: "Pi-hole v6 Network Ad-Blocker", Category: "Network & Ad-Block",
		Image: "pihole/pihole:latest", HostPort: 8085, InternalPort: 80,
		ExtraPorts: []string{"53:53/tcp", "53:53/udp"},
		CapAdd:     []string{"NET_ADMIN"},
		Environment: map[string]string{
			"TZ":                             "UTC",
			"FTLCONF_dns_listeningMode":      "all",
			"FTLCONF_webserver_api_password": "admin",
			"FTLCONF_misc_etc_dnsmasq_d":     "true",
		},
		Volumes: []string{"/DATA/AppData/pihole/etc-pihole:/etc/pihole", "/DATA/AppData/pihole/etc-dnsmasq.d:/etc/dnsmasq.d"},
	},
	"adguardhome": {
		ID: "adguardhome", Title: "AdGuard Home DNS", Category: "Network & Ad-Block",
		Image: "adguard/adguardhome:latest", HostPort: 3000, InternalPort: 3000,
		ExtraPorts: []string{"53:53/tcp", "53:53/udp"},
		CapAdd:     []string{"NET_ADMIN"},
		Volumes:    []string{"/DATA/AppData/adguardhome/work:/opt/adguardhome/work", "/DATA/AppData/adguardhome/conf:/opt/adguardhome/conf"},
	},
	"wg_easy": {
		ID: "wg_easy", Title: "WireGuard VPN (wg-easy v15)", Category: "Network & Ad-Block",
		Image: "ghcr.io/wg-easy/wg-easy:15", HostPort: 51821, InternalPort: 51821,
		ExtraPorts: []string{"51820:51820/udp"},
		CapAdd:     []string{"NET_ADMIN", "SYS_MODULE"},
		Sysctls:    []string{"net.ipv4.ip_forward=1", "net.ipv4.conf.all.src_valid_mark=1", "net.ipv6.conf.all.disable_ipv6=0", "net.ipv6.conf.all.forwarding=1"},
		Volumes:    []string{"/DATA/AppData/wg_easy:/etc/wireguard"},
	},
	"qbittorrent": {
		ID: "qbittorrent", Title: "qBittorrent Web UI", Category: "Media & Photos",
		Image: "lscr.io/linuxserver/qbittorrent:latest", HostPort: 8090, InternalPort: 8090,
		ExtraPorts:  []string{"6881:6881/tcp", "6881:6881/udp"},
		Environment: map[string]string{"WEBUI_PORT": "8090", "TORRENTING_PORT": "6881"},
		Volumes:     []string{"/DATA/AppData/qbittorrent/config:/config", "/DATA/Downloads:/downloads"},
	},
	"jellyseerr": {
		ID: "jellyseerr", Title: "Jellyseerr Media Requests", Category: "Media & Photos",
		Image: "fallenbagel/jellyseerr:latest", HostPort: 5055, InternalPort: 5055,
		Volumes: []string{"/DATA/AppData/jellyseerr/config:/app/config"},
	},
	"kavita": {
		ID: "kavita", Title: "Kavita Manga & eBook Reader", Category: "Knowledge & Wiki",
		Image: "jvmilazz0/kavita:latest", HostPort: 5000, InternalPort: 5000,
		Volumes: []string{"/DATA/AppData/kavita/config:/kavita/config", "/DATA/Media/Books:/library:ro"},
	},
	"actualbudget": {
		ID: "actualbudget", Title: "Actual Budget Finance Server", Category: "Knowledge & Wiki",
		Image: "actualbudget/actual-server:latest", HostPort: 5006, InternalPort: 5006,
		Volumes: []string{"/DATA/AppData/actualbudget/data:/data"},
	},
	"nginxproxymanager": {
		ID: "nginxproxymanager", Title: "Nginx Proxy Manager", Category: "Network & Ad-Block",
		Image: "jc21/nginx-proxy-manager:latest", HostPort: 81, InternalPort: 81,
		ExtraPorts: []string{"80:80/tcp", "443:443/tcp"},
		Volumes:    []string{"/DATA/AppData/nginxproxymanager/data:/data", "/DATA/AppData/nginxproxymanager/letsencrypt:/etc/letsencrypt"},
	},
	"kiwix": {
		ID: "kiwix", Title: "Offline Wikipedia (Kiwix)", Category: "Knowledge & Wiki",
		Image: "ghcr.io/kiwix/kiwix-serve:latest", HostPort: 8086, InternalPort: 8080,
		Command: "*.zim",
		Volumes: []string{"/DATA/AppData/kiwix/zim:/data"},
	},
	"immich": {
		ID: "immich", Title: "Immich Photo Backup", Category: "Media & Photos",
		Image: "ghcr.io/immich-app/immich-server:release", HostPort: 2283, InternalPort: 2283,
		Devices: []string{"/dev/dri:/dev/dri"},
		Volumes: []string{"/DATA/AppData/immich/upload:/usr/src/app/upload"},
	},
	"homeassistant": {
		ID: "homeassistant", Title: "Home Assistant", Category: "Smart Home",
		Image: "ghcr.io/home-assistant/home-assistant:stable", HostPort: 8123, InternalPort: 8123,
		Volumes: []string{"/DATA/AppData/homeassistant/config:/config", "/etc/localtime:/etc/localtime:ro"},
	},
	"frigate": {
		ID: "frigate", Title: "Frigate AI NVR Camera System", Category: "Smart Home",
		Image: "ghcr.io/blakeblackshear/frigate:stable", HostPort: 8971, InternalPort: 8971,
		ShmSize:    "256mb",
		ExtraPorts: []string{"5001:5000/tcp", "8554:8554/tcp", "8555:8555/tcp", "8555:8555/udp"},
		Devices:    []string{"/dev/dri:/dev/dri"},
		Volumes:    []string{"/DATA/AppData/frigate/config:/config", "/DATA/Media/Frigate:/media/frigate", "/etc/localtime:/etc/localtime:ro"},
	},
	"vaultwarden": {
		ID: "vaultwarden", Title: "Vaultwarden Passwords", Category: "Security & Tools",
		Image: "vaultwarden/server:latest", HostPort: 8082, InternalPort: 80,
		Volumes: []string{"/DATA/AppData/vaultwarden/data:/data"},
	},
	"stirlingpdf": {
		ID: "stirlingpdf", Title: "Stirling-PDF Studio", Category: "Security & Tools",
		Image: "stirlingtools/stirling-pdf:latest", HostPort: 8084, InternalPort: 8080,
		Volumes: []string{"/DATA/AppData/stirlingpdf/configs:/configs"},
	},
	"filebrowser": {
		ID: "filebrowser", Title: "FileBrowser Web NAS", Category: "Cloud & Files",
		Image: "filebrowser/filebrowser:latest", HostPort: 8095, InternalPort: 80,
		Volumes: []string{"/DATA:/srv", "/DATA/AppData/filebrowser:/database"},
	},
	"gitea": {
		ID: "gitea", Title: "Gitea Private Git", Category: "Security & Tools",
		Image: "gitea/gitea:latest", HostPort: 3002, InternalPort: 3000,
		ExtraPorts: []string{"2222:22/tcp"},
		Volumes:    []string{"/DATA/AppData/gitea/data:/data"},
	},
	"bookstack": {
		ID: "bookstack", Title: "BookStack Family Wiki", Category: "Knowledge & Wiki",
		Image: "lscr.io/linuxserver/bookstack:latest", HostPort: 6875, InternalPort: 80,
		Volumes: []string{"/DATA/AppData/bookstack/config:/config"},
	},
	"audiobookshelf": {
		ID: "audiobookshelf", Title: "Audiobookshelf", Category: "Media & Photos",
		Image: "ghcr.io/advplyr/audiobookshelf:latest", HostPort: 13378, InternalPort: 80,
		Volumes: []string{"/DATA/Media/Audiobooks:/audiobooks", "/DATA/Media/Podcasts:/podcasts", "/DATA/AppData/audiobookshelf/config:/config", "/DATA/AppData/audiobookshelf/metadata:/metadata"},
	},
	"mealie": {
		ID: "mealie", Title: "Mealie Recipe Manager", Category: "Knowledge & Wiki",
		Image: "ghcr.io/mealie-recipes/mealie:latest", HostPort: 9925, InternalPort: 9000,
		Volumes: []string{"/DATA/AppData/mealie/data:/app/data"},
	},
	"dozzle": {
		ID: "dozzle", Title: "Dozzle Live Container Logs", Category: "Security & Tools",
		Image: "amir20/dozzle:latest", HostPort: 8888, InternalPort: 8080,
		Volumes: []string{"/var/run/docker.sock:/var/run/docker.sock:ro"},
	},
	"ollama_webui": {
		ID: "ollama_webui", Title: "Ollama + Open WebUI (Local AI)", Category: "AI & Machine Learning",
		Image: "ghcr.io/open-webui/open-webui:ollama", HostPort: 3005, InternalPort: 8080,
		Volumes: []string{"/DATA/AppData/ollama_webui/ollama:/root/.ollama", "/DATA/AppData/ollama_webui/data:/app/backend/data"},
	},
	"open_webui": {
		ID: "open_webui", Title: "Open WebUI", Category: "AI & Machine Learning",
		Image: "ghcr.io/open-webui/open-webui:main", HostPort: 3006, InternalPort: 8080,
		Volumes: []string{"/DATA/AppData/open_webui/data:/app/backend/data"},
	},
	"minecraft_bedrock": {
		ID: "minecraft_bedrock", Title: "Minecraft Bedrock Server", Category: "Gaming Servers",
		Image: "itzg/minecraft-bedrock-server:latest", HostPort: 19132, InternalPort: 19132,
		UDPOnly:     true,
		Environment: map[string]string{"EULA": "TRUE", "GAMEMODE": "survival", "DIFFICULTY": "normal"},
		Volumes:     []string{"/DATA/AppData/minecraft_bedrock/data:/data"},
	},
	"valheim": {
		ID: "valheim", Title: "Valheim Dedicated Server", Category: "Gaming Servers",
		Image: "lloesche/valheim-server:latest", HostPort: 2456, InternalPort: 2456,
		UDPOnly:     true,
		ExtraPorts:  []string{"2457:2457/udp"},
		Environment: map[string]string{"SERVER_NAME": "InstantServerValheim", "WORLD_NAME": "Dedicated", "SERVER_PASS": "secret123"},
		Volumes:     []string{"/DATA/AppData/valheim/config:/config", "/DATA/AppData/valheim/data:/opt/valheim"},
	},
	"palworld": {
		ID: "palworld", Title: "Palworld Dedicated Server", Category: "Gaming Servers",
		Image: "thijsvanloef/palworld-server-docker:latest", HostPort: 8211, InternalPort: 8211,
		UDPOnly:     true,
		ExtraPorts:  []string{"27015:27015/udp"},
		Environment: map[string]string{"PLAYERS": "16", "MULTITHREADING": "true", "RCON_ENABLED": "true"},
		Volumes:     []string{"/DATA/AppData/palworld/palworld:/palworld"},
	},
	"factorio": {
		ID: "factorio", Title: "Factorio Dedicated Server", Category: "Gaming Servers",
		Image: "factoriotools/factorio:stable", HostPort: 34197, InternalPort: 34197,
		UDPOnly:    true,
		ExtraPorts: []string{"27015:27015/tcp"},
		Volumes:    []string{"/DATA/AppData/factorio:/factorio"},
	},
	"terraria": {
		ID: "terraria", Title: "Terraria (TShock) Server", Category: "Gaming Servers",
		Image: "ryshe/terraria:latest", HostPort: 7777, InternalPort: 7777,
		Volumes: []string{"/DATA/AppData/terraria/world:/root/.local/share/Terraria/Worlds"},
	},
	"satisfactory": {
		ID: "satisfactory", Title: "Satisfactory Dedicated Server", Category: "Gaming Servers",
		Image: "wolveix/satisfactory-server:latest", HostPort: 7777, InternalPort: 7777,
		ExtraPorts: []string{"7777:7777/udp", "8888:8888/tcp"},
		Volumes:    []string{"/DATA/AppData/satisfactory:/config"},
	},
	"cs2_server": {
		ID: "cs2_server", Title: "Counter-Strike 2 Dedicated Server", Category: "Gaming Servers",
		Image: "joedwards32/cs2:latest", HostPort: 27015, InternalPort: 27015,
		ExtraPorts: []string{"27015:27015/udp"},
		Volumes:    []string{"/DATA/AppData/cs2_server:/home/steam/cs2-dedicated"},
	},
}
