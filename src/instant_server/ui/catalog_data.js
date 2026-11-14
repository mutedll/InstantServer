const APP_STORE_CATALOG = [
  {
    "id": "kiwix",
    "title": "Offline Wikipedia (Kiwix)",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files"
    ],
    "recommended": true,
    "tag": "Port 8086",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/kiwix/kiwix-serve:latest",
    "port": 8086,
    "desc": "Host full offline Wikipedia, Wiktionary, medical guides, and Project Gutenberg (.zim files) on any old..."
  },
  {
    "id": "jellyfin",
    "title": "Jellyfin",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": true,
    "tag": "Port 8096",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "jellyfin/jellyfin:latest",
    "port": 8096,
    "desc": "Stream your movies, TV shows, and music to phones, smart TVs, and browsers with zero subscriptions."
  },
  {
    "id": "nextcloud",
    "title": "Nextcloud",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": true,
    "tag": "Port 8081",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "nextcloud:stable",
    "port": 8081,
    "desc": "Private Google Drive & Google Workspace replacement for your files, calendars, contacts, and phone bac..."
  },
  {
    "id": "pihole",
    "title": "Pi-hole",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Media & Photos",
      "Security & Tools"
    ],
    "recommended": true,
    "tag": "Port 8085",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "pihole/pihole:latest",
    "port": 8085,
    "desc": "Network-wide DNS ad-blocker and tracker shield that blocks ads on every phone, TV, and laptop on your..."
  },
  {
    "id": "immich",
    "title": "Immich",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Cloud & Files",
      "AI & Machine Learning"
    ],
    "recommended": true,
    "tag": "Port 2283",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/immich-app/immich-server:release",
    "port": 2283,
    "desc": "Fast self-hosted Google Photos alternative with automatic iOS/Android camera backup and album sharing."
  },
  {
    "id": "homeassistant",
    "title": "Home Assistant",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Media & Photos"
    ],
    "recommended": true,
    "tag": "Port 8123",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "homeassistant/home-assistant:stable",
    "port": 8123,
    "desc": "Automate smart lights, plugs, cameras, and Zigbee/Matter devices locally inside your home."
  },
  {
    "id": "vaultwarden",
    "title": "Vaultwarden",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": true,
    "tag": "Port 8082",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "vaultwarden/server:latest",
    "port": 8082,
    "desc": "Ultra-lightweight Bitwarden-compatible password manager server for your browser extensions and mobile..."
  },
  {
    "id": "minecraft",
    "title": "Minecraft Dedicated Server (Java)",
    "category": "Gaming Servers",
    "categories": [
      "Gaming Servers",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 25565",
    "ram": "2 GB+",
    "hardware_tier": "high_end",
    "image": "itzg/minecraft-server:latest",
    "port": 25565,
    "desc": "24/7 dedicated Minecraft Java server (Vanilla, Paper, Spigot, Fabric, or Forge) with automatic world b...",
    "hardware_note": "4GB+ RAM"
  },
  {
    "id": "minecraft_bedrock",
    "title": "Minecraft Bedrock Server (Mobile / Console)",
    "category": "Gaming Servers",
    "categories": [
      "Gaming Servers"
    ],
    "recommended": false,
    "tag": "Port 19132",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "itzg/minecraft-bedrock-server:latest",
    "port": 19132,
    "desc": "Lightweight C++ Minecraft Bedrock server for Phones (iOS/Android), Xbox, PlayStation, Switch, and Wind..."
  },
  {
    "id": "ollama_webui",
    "title": "Local AI Server (Ollama + Open WebUI)",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 3005",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "ghcr.io/open-webui/open-webui:ollama",
    "port": 3005,
    "desc": "Run private local AI models (Llama 3, DeepSeek, Mistral, Qwen) with a ChatGPT web UI. Not for old low-...",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "localai",
    "title": "LocalAI (Private AI Voice, Vision & Text)",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8088",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "localai/localai:latest",
    "port": 8088,
    "desc": "Self-hosted OpenAI-compatible API for local text generation, Whisper speech-to-text, and voice synthesis.",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "bookstack",
    "title": "BookStack (Personal Wiki & Notes)",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 6875",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/bookstack:latest",
    "port": 6875,
    "desc": "Build your own Wikipedia-style knowledge base, family handbook, and study notes organized into books a..."
  },
  {
    "id": "calibreweb",
    "title": "Calibre-Web (E-Book Library)",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8083",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/calibre-web:latest",
    "port": 8083,
    "desc": "Browse, read in browser, and wireless-send EPUB/PDF e-books to your Kindle, Kobo, or phone."
  },
  {
    "id": "wg_easy",
    "title": "WireGuard VPN (wg-easy)",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools"
    ],
    "recommended": true,
    "tag": "Port 51821",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/wg-easy/wg-easy:15",
    "port": 51821,
    "desc": "Fast WireGuard VPN server with a web dashboard and instant QR-code setup for phones and laptops."
  },
  {
    "id": "adguardhome",
    "title": "AdGuard Home",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3000",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "adguard/adguardhome:latest",
    "port": 3000,
    "desc": "All-in-one DNS ad-blocker, parental control filter, and encrypted DNS-over-HTTPS server."
  },
  {
    "id": "plex",
    "title": "Plex Media Server",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 32400",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/plex:latest",
    "port": 32400,
    "desc": "Organize and stream personal movies, shows, and music to Plex apps on any smart TV, console, or phone."
  },
  {
    "id": "syncthing",
    "title": "Syncthing",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": true,
    "tag": "Port 8384",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/syncthing:latest",
    "port": 8384,
    "desc": "Real-time peer-to-peer folder sync across your laptops, desktops, and Android devices."
  },
  {
    "id": "filebrowser",
    "title": "FileBrowser",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8087",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "filebrowser/filebrowser:latest",
    "port": 8087,
    "desc": "Simple web interface to browse, upload, download, and share files stored on your server."
  },
  {
    "id": "qbittorrent",
    "title": "qBittorrent",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8080",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/qbittorrent:latest",
    "port": 8080,
    "desc": "Web-based torrent downloader with magnet link, sequential download, and RSS auto-download support."
  },
  {
    "id": "uptimekuma",
    "title": "Uptime Kuma",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": true,
    "tag": "Port 3001",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "louislam/uptime-kuma:1",
    "port": 3001,
    "desc": "Self-hosted status page and uptime monitor that sends alerts if your home apps or internet go offline."
  },
  {
    "id": "portainer",
    "title": "Portainer CE",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Network & Ad-Block",
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": true,
    "tag": "Port 9443",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "portainer/portainer-ce:latest",
    "port": 9443,
    "desc": "Advanced web dashboard for managing Docker containers, images, networks, and Compose stacks."
  },
  {
    "id": "nginxproxymanager",
    "title": "Nginx Proxy Manager",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 81",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "jc21/nginx-proxy-manager:latest",
    "port": 81,
    "desc": "Route custom domain names to your self-hosted apps with automatic Let's Encrypt SSL certificates."
  },
  {
    "id": "paperless",
    "title": "Paperless-ngx",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Security & Tools",
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8000",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/paperless-ngx/paperless-ngx:latest",
    "port": 8000,
    "desc": "Organize scanned receipts, PDFs, and invoices with automatic OCR full-text search."
  },
  {
    "id": "stirlingpdf",
    "title": "Stirling-PDF",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Cloud & Files",
      "Knowledge & Wiki"
    ],
    "recommended": true,
    "tag": "Port 8084",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "stirlingtools/stirling-pdf:latest",
    "port": 8084,
    "desc": "Private local web toolkit to merge, split, rotate, compress, and convert PDF documents."
  },
  {
    "id": "navidrome",
    "title": "Navidrome",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 4533",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "deluan/navidrome:latest",
    "port": 4533,
    "desc": "Fast, lightweight personal music streaming server compatible with Subsonic mobile apps."
  },
  {
    "id": "gitea",
    "title": "Gitea",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3002",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "gitea/gitea:latest",
    "port": 3002,
    "desc": "Lightweight self-hosted Git service for hosting your private code repositories, issues, and actions."
  },
  {
    "id": "audiobookshelf",
    "title": "Audiobookshelf",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Cloud & Files",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 13378",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/advplyr/audiobookshelf:latest",
    "port": 13378,
    "desc": "Stream audiobooks and podcasts with progress sync across iOS and Android."
  },
  {
    "id": "kavita",
    "title": "Kavita",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 5000",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "jvmilazz0/kavita:latest",
    "port": 5000,
    "desc": "Fast digital reader for manga, comics, EPUBs, and PDFs."
  },
  {
    "id": "komga",
    "title": "Komga",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 25600",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "gotson/komga:latest",
    "port": 25600,
    "desc": "Media server for comics, mangas, BDs, and magazines with OPDS support."
  },
  {
    "id": "photoprism",
    "title": "PhotoPrism",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Cloud & Files",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 2342",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "photoprism/photoprism:latest",
    "port": 2342,
    "desc": "Browse, tag, and organize family photos with map view and face recognition."
  },
  {
    "id": "photoview",
    "title": "Photoview",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8089",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "viktorstrate/photoview:2",
    "port": 8089,
    "desc": "Fast photo gallery built for browsing existing folder hierarchies on disk."
  },
  {
    "id": "swingmusic",
    "title": "Swing Music",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 1970",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/swingmx/swingmusic:latest",
    "port": 1970,
    "desc": "Modern self-hosted music player with gapless playback and lyrics."
  },
  {
    "id": "airsonic",
    "title": "Airsonic-Advanced",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 4040",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "airsonicadvanced/airsonic-advanced:latest",
    "port": 4040,
    "desc": "Full-featured Subsonic music and podcast server with transcoding."
  },
  {
    "id": "jellyseerr",
    "title": "Jellyseerr",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 5055",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "fallenbagel/jellyseerr:latest",
    "port": 5055,
    "desc": "Request and discover movies and TV shows for Jellyfin and Plex."
  },
  {
    "id": "overseerr",
    "title": "Overseerr",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 5056",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/overseerr:latest",
    "port": 5056,
    "desc": "Request management and media discovery portal for Plex libraries."
  },
  {
    "id": "tautulli",
    "title": "Tautulli",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8181",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/tautulli:latest",
    "port": 8181,
    "desc": "Monitor stream history, bandwidth, and playback stats on your media server."
  },
  {
    "id": "ersatztv",
    "title": "ErsatzTV",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8409",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "jasongdove/ersatztv:latest",
    "port": 8409,
    "desc": "Turn your personal movie and show files into live IPTV channels with EPG."
  },
  {
    "id": "owncast",
    "title": "Owncast",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 8097",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "owncast/owncast:latest",
    "port": 8097,
    "desc": "Self-hosted live video streaming and chat server compatible with OBS."
  },
  {
    "id": "transmission",
    "title": "Transmission",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 9091",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/transmission:latest",
    "port": 9091,
    "desc": "Ultra-lightweight torrent client with a clean web UI for low-RAM laptops."
  },
  {
    "id": "deluge",
    "title": "Deluge",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 8112",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/deluge:latest",
    "port": 8112,
    "desc": "Plugin-extensible BitTorrent client with web and thin-client support."
  },
  {
    "id": "sabnzbd",
    "title": "SABnzbd",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8090",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/sabnzbd:latest",
    "port": 8090,
    "desc": "Automated Usenet binary newsreader with PAR2 repair and unpacking."
  },
  {
    "id": "nzbget",
    "title": "NZBGet",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 6789",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/nzbget:latest",
    "port": 6789,
    "desc": "Low-memory C++ Usenet downloader ideal for older laptops and mini-PCs."
  },
  {
    "id": "sonarr",
    "title": "Sonarr",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 8989",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/sonarr:latest",
    "port": 8989,
    "desc": "Automatically track, organize, and rename TV series episodes."
  },
  {
    "id": "radarr",
    "title": "Radarr",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 7878",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/radarr:latest",
    "port": 7878,
    "desc": "Automated movie collection manager and organizer for your home cinema."
  },
  {
    "id": "prowlarr",
    "title": "Prowlarr",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 9696",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/prowlarr:latest",
    "port": 9696,
    "desc": "Centralized indexer manager that syncs across Sonarr, Radarr, and Lidarr."
  },
  {
    "id": "lidarr",
    "title": "Lidarr",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 8686",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/lidarr:latest",
    "port": 8686,
    "desc": "Music collection manager that organizes albums, tracks, and metadata."
  },
  {
    "id": "readarr",
    "title": "Readarr",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 8787",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/readarr:develop",
    "port": 8787,
    "desc": "E-book and audiobook library organizer with automatic metadata tagging."
  },
  {
    "id": "bazarr",
    "title": "Bazarr",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 6767",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/bazarr:latest",
    "port": 6767,
    "desc": "Automatically downloads and syncs multi-language subtitles for movies and TV."
  },
  {
    "id": "metube",
    "title": "MeTube (yt-dlp Web UI)",
    "category": "Media & Photos",
    "categories": [
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 8091",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/alexta69/metube:latest",
    "port": 8091,
    "desc": "Save lectures, tutorials, and playlists for offline viewing via yt-dlp."
  },
  {
    "id": "owncloud",
    "title": "ownCloud Infinite Scale (oCIS)",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 9200",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "owncloud/ocis:latest",
    "port": 9200,
    "desc": "Single-binary Go file sync and share cloud with low memory footprint."
  },
  {
    "id": "seafile",
    "title": "Seafile",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8092",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "seafileltd/seafile-mc:latest",
    "port": 8092,
    "desc": "High-speed C-based cloud drive with block-level deduplication and sync."
  },
  {
    "id": "pairdrop",
    "title": "PairDrop (Local AirDrop)",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 3003",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/pairdrop:latest",
    "port": 3003,
    "desc": "Instant AirDrop-style file sharing between iPhones, Androids, Macs, and PCs."
  },
  {
    "id": "pingvinshare",
    "title": "Pingvin Share",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3004",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "stonith404/pingvin-share:latest",
    "port": 3004,
    "desc": "Self-hosted file sharing portal with expiring links and password protection."
  },
  {
    "id": "sftpgo",
    "title": "SFTPGo",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8093",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "drakkan/sftpgo:latest",
    "port": 8093,
    "desc": "Full-featured SFTP, HTTP, and WebDAV file server with web administration."
  },
  {
    "id": "bentopdf",
    "title": "BentoPDF",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8094",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/alam00000/bentopdf:latest",
    "port": 8094,
    "desc": "Clean browser-based PDF editor, merger, compressor, and converter."
  },
  {
    "id": "onlyoffice",
    "title": "ONLYOFFICE Document Server",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8095",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "onlyoffice/documentserver:latest",
    "port": 8095,
    "desc": "Collaborative online editor for Word, Excel, and PowerPoint files."
  },
  {
    "id": "collabora",
    "title": "Collabora Online (LibreOffice Web)",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 9980",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "collabora/code:latest",
    "port": 9980,
    "desc": "LibreOffice-powered collaborative document editing for Nextcloud and web."
  },
  {
    "id": "duplicati",
    "title": "Duplicati Backup",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8200",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/duplicati:latest",
    "port": 8200,
    "desc": "Scheduled AES-256 encrypted backups to USB drives, S3, SFTP, or NAS."
  },
  {
    "id": "kopia",
    "title": "Kopia Backup Server",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 51515",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "kopia/kopia:latest",
    "port": 51515,
    "desc": "Fast, encrypted, deduplicated snapshot backup tool with web UI."
  },
  {
    "id": "outline",
    "title": "Outline Wiki",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 3006",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "docker.getoutline.com/outlinewiki/outline:latest",
    "port": 3006,
    "desc": "Fast, markdown-first knowledge base and documentation workspace."
  },
  {
    "id": "affine",
    "title": "AFFiNE",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 3010",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/toeverything/affine:stable",
    "port": 3010,
    "desc": "Privacy-focused workspace blending Notion-style docs and Miro whiteboards."
  },
  {
    "id": "blinko",
    "title": "Blinko",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 1111",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "blinkospace/blinko:latest",
    "port": 1111,
    "desc": "Self-hosted fleeting notes and knowledge base with local AI search."
  },
  {
    "id": "memos",
    "title": "Memos",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 5230",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "neosmemo/memos:stable",
    "port": 5230,
    "desc": "Ultra-fast, lightweight markdown note-taking hub that runs in 40 MB RAM."
  },
  {
    "id": "trilium",
    "title": "TriliumNext Notes",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8098",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "triliumnext/notes:latest",
    "port": 8098,
    "desc": "Build deep personal knowledge graphs, journals, and encrypted note trees."
  },
  {
    "id": "silverbullet",
    "title": "SilverBullet",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 3007",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "zefhemel/silverbullet:latest",
    "port": 3007,
    "desc": "Offline-capable plain-markdown note app with live queries and backlinks."
  },
  {
    "id": "docmost",
    "title": "Docmost",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 3008",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "docmost/docmost:latest",
    "port": 3008,
    "desc": "Clean open-source Confluence and Notion alternative for documentation."
  },
  {
    "id": "karakeep",
    "title": "Karakeep (Hoarder Bookmarks)",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 3009",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/karakeep-app/karakeep:release",
    "port": 3009,
    "desc": "Save links, articles, notes, and full-page snapshots with auto-tagging."
  },
  {
    "id": "linkwarden",
    "title": "Linkwarden",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3011",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/linkwarden/linkwarden:latest",
    "port": 3011,
    "desc": "Preserve web pages asoffline HTML/PDFs so saved links never succumb to link rot."
  },
  {
    "id": "freshrss",
    "title": "FreshRSS",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8099",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/freshrss:latest",
    "port": 8099,
    "desc": "Fast, self-hosted RSS and Atom news aggregator capable of 100,000+ articles."
  },
  {
    "id": "miniflux",
    "title": "Miniflux",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8101",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "miniflux/miniflux:latest",
    "port": 8101,
    "desc": "Opinionated, keyboard-driven minimalist feed reader written in Go."
  },
  {
    "id": "wallabag",
    "title": "Wallabag (Read-It-Later)",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8102",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "wallabag/wallabag:latest",
    "port": 8102,
    "desc": "Save web articles cleanly without ads for offline reading on phone or e-reader."
  },
  {
    "id": "technitium",
    "title": "Technitium DNS Server",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 5380",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "technitium/dns-server:latest",
    "port": 5380,
    "desc": "Full authoritative and recursive DNS server with blocklists and DNS-over-HTTPS."
  },
  {
    "id": "unbound",
    "title": "Unbound Recursive DNS",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 5335",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "mvance/unbound:latest",
    "port": 5335,
    "desc": "Validating, recursive, caching DNS resolver that queries root servers directly."
  },
  {
    "id": "headscale",
    "title": "Headscale (Self-Hosted Tailscale Control)",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8103",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "headscale/headscale:latest",
    "port": 8103,
    "desc": "Open-source implementation of the Tailscale mesh VPN control server."
  },
  {
    "id": "netbird",
    "title": "NetBird Mesh VPN",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8104",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "netbirdio/dashboard:latest",
    "port": 8104,
    "desc": "Peer-to-peer WireGuard overlay network connecting all your devices securely."
  },
  {
    "id": "traefik",
    "title": "Traefik Proxy",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Cloud & Files",
      "Security & Tools",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8105",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "traefik:v3.1",
    "port": 8105,
    "desc": "Cloud-native edge router that automatically discovers running Docker apps."
  },
  {
    "id": "caddy",
    "title": "Caddy Web & Reverse Proxy",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8106",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "caddy:latest",
    "port": 8106,
    "desc": "Fast, automatic-HTTPS reverse proxy and static file server."
  },
  {
    "id": "cloudflared",
    "title": "Cloudflare Tunnel (cloudflared)",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Cloud & Files",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8107",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "cloudflare/cloudflared:latest",
    "port": 8107,
    "desc": "Expose self-hosted web apps securely to your domain without opening router ports."
  },
  {
    "id": "librespeed",
    "title": "LibreSpeed (LAN Speedtest)",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block"
    ],
    "recommended": false,
    "tag": "Port 8108",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/librespeed:latest",
    "port": 8108,
    "desc": "Test real Wi-Fi and Ethernet throughput between your phones/laptops and server."
  },
  {
    "id": "openspeedtest",
    "title": "OpenSpeedTest",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block"
    ],
    "recommended": false,
    "tag": "Port 3012",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "openspeedtest/latest:latest",
    "port": 3012,
    "desc": "Zero-plugin HTML5 network speed test for diagnosing home Wi-Fi dead zones."
  },
  {
    "id": "speedtest_tracker",
    "title": "Speedtest Tracker",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8109",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/speedtest-tracker:latest",
    "port": 8109,
    "desc": "Continuously logs your ISP download, upload, and ping history with charts."
  },
  {
    "id": "smokeping",
    "title": "SmokePing",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block"
    ],
    "recommended": false,
    "tag": "Port 8110",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/smokeping:latest",
    "port": 8110,
    "desc": "Track network latency jitter and packet loss over time."
  },
  {
    "id": "frigate",
    "title": "Frigate NVR (AI Security Cameras)",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Media & Photos",
      "Security & Tools",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 5001",
    "ram": "2 GB+",
    "hardware_tier": "high_end",
    "image": "ghcr.io/blakeblackshear/frigate:stable",
    "port": 5001,
    "desc": "24/7 IP camera NVR with real-time local person, car, and pet detection.",
    "hardware_note": "4GB+ RAM"
  },
  {
    "id": "scrypted",
    "title": "Scrypted (HomeKit Secure Video Bridge)",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 10443",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "koush/scrypted:latest",
    "port": 10443,
    "desc": "Bridge any RTSP/ONVIF camera into Apple HomeKit Secure Video and Google Home."
  },
  {
    "id": "homebridge",
    "title": "Homebridge",
    "category": "Smart Home",
    "categories": [
      "Smart Home"
    ],
    "recommended": false,
    "tag": "Port 8581",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "homebridge/homebridge:latest",
    "port": 8581,
    "desc": "Connect thousands of non-HomeKit smart devices to Apple Home and Siri."
  },
  {
    "id": "zigbee2mqtt",
    "title": "Zigbee2MQTT",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8111",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "koenkk/zigbee2mqtt:latest",
    "port": 8111,
    "desc": "Pair Aqara, Philips Hue, IKEA, and Sonoff Zigbee sensors without proprietary hubs."
  },
  {
    "id": "mosquitto",
    "title": "Eclipse Mosquitto MQTT Broker",
    "category": "Smart Home",
    "categories": [
      "Smart Home"
    ],
    "recommended": false,
    "tag": "Port 1883",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "eclipse-mosquitto:2",
    "port": 1883,
    "desc": "Ultra-fast MQTT message broker for Home Assistant, Zigbee2MQTT, and Tasmota."
  },
  {
    "id": "nodered",
    "title": "Node-RED",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 1880",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "nodered/node-red:latest",
    "port": 1880,
    "desc": "Drag-and-drop visual wiring tool for smart home automations and APIs."
  },
  {
    "id": "esphome",
    "title": "ESPHome Dashboard",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 6052",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/esphome/esphome:stable",
    "port": 6052,
    "desc": "Build and wireless-flash custom ESP32/ESP8266 sensors from simple YAML files."
  },
  {
    "id": "music_assistant",
    "title": "Music Assistant",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Media & Photos",
      "Cloud & Files",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8095",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/music-assistant/server:latest",
    "port": 8095,
    "desc": "Stream local files and music services across Sonos, Chromecast, and AirPlay speakers."
  },
  {
    "id": "anythingllm",
    "title": "AnythingLLM (Chat with Your PDFs & Docs)",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning",
      "Cloud & Files",
      "Security & Tools",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 3013",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "mintplexlabs/anythingllm:latest",
    "port": 3013,
    "desc": "All-in-one private AI desktop & server app to chat with your local documents.",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "librechat",
    "title": "LibreChat",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 3080",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "ghcr.io/danny-avila/librechat:latest",
    "port": 3080,
    "desc": "Unified ChatGPT-style interface supporting Ollama, OpenAI, Claude, and Gemini.",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "whisper_asr",
    "title": "Whisper ASR (Offline Speech-to-Text)",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning",
      "Media & Photos"
    ],
    "recommended": false,
    "tag": "Port 9000",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "onerahmet/openai-whisper-asr-webservice:latest",
    "port": 9000,
    "desc": "Local OpenAI Whisper speech-to-text API and web UI for transcribing audio/video.",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "flowise",
    "title": "Flowise AI (Visual LLM Builder)",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 3014",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "flowiseai/flowise:latest",
    "port": 3014,
    "desc": "Build custom AI agents, RAG pipelines, and chatbots visually.",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "sd_forge",
    "title": "Stable Diffusion WebUI Forge",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 7860",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "nykk3/stable-diffusion-webui-forge:latest",
    "port": 7860,
    "desc": "Generate images locally with Stable Diffusion and Flux on GPU workstations.",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "romm",
    "title": "RomM (Retro Gaming & Emulator Library)",
    "category": "Gaming Servers",
    "categories": [
      "Gaming Servers"
    ],
    "recommended": false,
    "tag": "Port 8113",
    "ram": "2 GB+",
    "hardware_tier": "high_end",
    "image": "rommapp/romm:latest",
    "port": 8113,
    "desc": "Manage and play retro console games directly in your browser via EmulatorJS.",
    "hardware_note": "4GB+ RAM"
  },
  {
    "id": "palworld",
    "title": "Palworld Dedicated Server",
    "category": "Gaming Servers",
    "categories": [
      "Gaming Servers",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8211",
    "ram": "2 GB+",
    "hardware_tier": "high_end",
    "image": "thijsvanloef/palworld-server-docker:latest",
    "port": 8211,
    "desc": "Dedicated multiplayer Palworld server with automatic world backups.",
    "hardware_note": "4GB+ RAM"
  },
  {
    "id": "valheim",
    "title": "Valheim Dedicated Server",
    "category": "Gaming Servers",
    "categories": [
      "Gaming Servers",
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 2456",
    "ram": "2 GB+",
    "hardware_tier": "high_end",
    "image": "lloesche/valheim-server:latest",
    "port": 2456,
    "desc": "24/7 Valheim multiplayer world server with automatic update & backup hooks.",
    "hardware_note": "4GB+ RAM"
  },
  {
    "id": "terraria",
    "title": "Terraria Server (TShock)",
    "category": "Gaming Servers",
    "categories": [
      "Gaming Servers"
    ],
    "recommended": false,
    "tag": "Port 7777",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ryshe/terraria:latest",
    "port": 7777,
    "desc": "Lightweight dedicated Terraria multiplayer server that runs great on old laptops."
  },
  {
    "id": "factorio",
    "title": "Factorio Headless Server",
    "category": "Gaming Servers",
    "categories": [
      "Gaming Servers"
    ],
    "recommended": false,
    "tag": "Port 34197",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "factoriotools/factorio:stable",
    "port": 34197,
    "desc": "Dedicated Factorio multiplayer server for building megafactories with friends."
  },
  {
    "id": "actualbudget",
    "title": "Actual Budget",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 5006",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "actualbudget/actual-server:latest",
    "port": 5006,
    "desc": "Fast, local-first envelope budgeting and personal finance tracker."
  },
  {
    "id": "firefly",
    "title": "Firefly III",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8114",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "fireflyiii/core:latest",
    "port": 8114,
    "desc": "Comprehensive personal finance manager with double-entry accounting and reports."
  },
  {
    "id": "wallos",
    "title": "Wallos (Subscription Tracker)",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 8282",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "bellamy/wallos:latest",
    "port": 8282,
    "desc": "Track recurring subscriptions, bills, and renewal reminders in multiple currencies."
  },
  {
    "id": "mealie",
    "title": "Mealie (Recipe & Meal Planner)",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 9925",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/mealie-recipes/mealie:latest",
    "port": 9925,
    "desc": "Import recipes from any website in 1 click, plan weekly meals, and build grocery lists."
  },
  {
    "id": "tandoor",
    "title": "Tandoor Recipes",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Cloud & Files",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8115",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "vabene1111/recipes:latest",
    "port": 8115,
    "desc": "Manage recipes, scale ingredients, and sync shared household shopping lists."
  },
  {
    "id": "grocy",
    "title": "Grocy (Pantry & Household ERP)",
    "category": "Smart Home",
    "categories": [
      "Smart Home"
    ],
    "recommended": false,
    "tag": "Port 9283",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/grocy:latest",
    "port": 9283,
    "desc": "Track kitchen inventory, expiration dates, household chores, and battery replacements."
  },
  {
    "id": "vikunja",
    "title": "Vikunja (To-Do & Kanban)",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files"
    ],
    "recommended": false,
    "tag": "Port 3456",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "vikunja/vikunja:latest",
    "port": 3456,
    "desc": "Organize tasks, lists, Gantt charts, and Kanban boards with CalDAV sync."
  },
  {
    "id": "nocodb",
    "title": "NocoDB (Airtable Alternative)",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8116",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "nocodb/nocodb:latest",
    "port": 8116,
    "desc": "Turn any database into a smart collaborative spreadsheet, gallery, and form builder."
  },
  {
    "id": "baserow",
    "title": "Baserow",
    "category": "Cloud & Files",
    "categories": [
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8117",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "baserow/baserow:latest",
    "port": 8117,
    "desc": "Create custom relational tables, views, and forms without writing code."
  },
  {
    "id": "homebox",
    "title": "Homebox (Home Inventory)",
    "category": "Smart Home",
    "categories": [
      "Smart Home",
      "Cloud & Files",
      "Security & Tools",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 3100",
    "ram": "512 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/sysadminsmedia/homebox:latest",
    "port": 3100,
    "desc": "Catalog home electronics, serial numbers, receipts, warranties, and storage boxes."
  },
  {
    "id": "2fauth",
    "title": "2FAuth (Web 2FA Authenticator)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8118",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "2fauth/2fauth:latest",
    "port": 8118,
    "desc": "Self-hosted two-factor authentication (TOTP) code generator with QR scanner."
  },
  {
    "id": "authentik",
    "title": "Authentik SSO",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 9001",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/goauthentik/server:latest",
    "port": 9001,
    "desc": "Protect all your self-hosted apps with single sign-on, passkeys, and 2FA."
  },
  {
    "id": "pocketid",
    "title": "Pocket ID (Passkey OIDC Provider)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3015",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/pocket-id/pocket-id:latest",
    "port": 3015,
    "desc": "Lightweight OIDC provider that lets you sign in to apps using fingerprint/FaceID passkeys."
  },
  {
    "id": "dockge",
    "title": "Dockge (Compose Stack Manager)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 5001",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "louislam/dockge:1",
    "port": 5001,
    "desc": "Reactive, easy-to-use `docker-compose.yaml` manager by the creator of Uptime Kuma."
  },
  {
    "id": "dozzle",
    "title": "Dozzle (Real-Time Container Logs)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8888",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "amir20/dozzle:latest",
    "port": 8888,
    "desc": "Real-time web viewer for Docker container logs with instant search and filtering."
  },
  {
    "id": "forgejo",
    "title": "Forgejo (Self-Hosted Git)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3016",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "codeberg.org/forgejo/forgejo:9",
    "port": 3016,
    "desc": "Community-governed lightweight Git hosting with pull requests and CI actions."
  },
  {
    "id": "codeserver",
    "title": "code-server (VS Code in Browser)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8443",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lscr.io/linuxserver/code-server:latest",
    "port": 8443,
    "desc": "Run full Visual Studio Code in your browser from any laptop or iPad."
  },
  {
    "id": "n8n",
    "title": "n8n Workflow Automation",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Smart Home",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 5678",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "docker.n8n.io/n8nio/n8n:latest",
    "port": 5678,
    "desc": "Automate workflows across 400+ apps, webhooks, emails, and local AI models."
  },
  {
    "id": "it_tools",
    "title": "IT-Tools (Developer Swiss Army Knife)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Network & Ad-Block"
    ],
    "recommended": false,
    "tag": "Port 8119",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "corentinth/it-tools:latest",
    "port": 8119,
    "desc": "80+ handy browser tools for hashes, QR codes, JSON, regex, and networking."
  },
  {
    "id": "cyberchef",
    "title": "CyberChef",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8120",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/gchq/cyberchef:latest",
    "port": 8120,
    "desc": "The Cyber Swiss Army Knife for encoding, encryption, compression, and data analysis."
  },
  {
    "id": "homarr",
    "title": "Homarr Dashboard",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 7575",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/ajnart/homarr:latest",
    "port": 7575,
    "desc": "Customizable homelab startpage with live widgets for Sonarr, Radarr, and qBittorrent."
  },
  {
    "id": "homepage",
    "title": "Homepage Dashboard",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3017",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/gethomepage/homepage:latest",
    "port": 3017,
    "desc": "Fast, static-rendered application dashboard with 100+ service integrations."
  },
  {
    "id": "dashy",
    "title": "Dashy",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 4000",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "lissy93/dashy:latest",
    "port": 4000,
    "desc": "Self-hosted dashboard with status checks, widgets, and instant theme switching."
  },
  {
    "id": "glance",
    "title": "Glance (Personal Feed & Status Hub)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8121",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "glanceapp/glance:latest",
    "port": 8121,
    "desc": "All-in-one personal startpage showing server stats, RSS feeds, weather, and YouTube."
  },
  {
    "id": "changedetection",
    "title": "changedetection.io",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 5005",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/dgtlmoon/changedetection.io:latest",
    "port": 5005,
    "desc": "Monitor websites for price drops, product restocks, and text changes."
  },
  {
    "id": "ntfy",
    "title": "ntfy Push Notification Server",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8122",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "binwiederhier/ntfy:latest",
    "port": 8122,
    "desc": "Send instant push notifications to your phone or desktop from any script via PUT/POST."
  },
  {
    "id": "gotify",
    "title": "Gotify Notification Server",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8124",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "gotify/server:latest",
    "port": 8124,
    "desc": "Simple real-time WebSocket server for sending and receiving messages."
  },
  {
    "id": "scrutiny",
    "title": "Scrutiny (Hard Drive S.M.A.R.T. Web UI)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Cloud & Files",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8125",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "ghcr.io/analogj/scrutiny:master-omnibus",
    "port": 8125,
    "desc": "Monitor hard drive and NVMe S.M.A.R.T. health metrics with Backblaze failure rates."
  },
  {
    "id": "glances",
    "title": "Glances System Monitor",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Network & Ad-Block"
    ],
    "recommended": false,
    "tag": "Port 61208",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "nicolargo/glances:latest-full",
    "port": 61208,
    "desc": "Cross-platform real-time CPU, memory, disk I/O, network, and sensor monitor."
  },
  {
    "id": "beszel",
    "title": "Beszel (Lightweight Server Monitoring Hub)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8090",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "henrygd/beszel:latest",
    "port": 8090,
    "desc": "Featherweight server resource and Docker container history monitor with alerts."
  },
  {
    "id": "grafana",
    "title": "Grafana",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 3018",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "grafana/grafana-oss:latest",
    "port": 3018,
    "desc": "Visualize time-series metrics, server temperatures, power draw, and logs."
  },
  {
    "id": "prometheus",
    "title": "Prometheus",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 9092",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "prom/prometheus:latest",
    "port": 9092,
    "desc": "Collect and query real-time metrics from your server nodes and containers."
  },
  {
    "id": "watchtower",
    "title": "Watchtower (Auto Container Updater)",
    "category": "Security & Tools",
    "categories": [
      "Security & Tools",
      "Knowledge & Wiki",
      "AI & Machine Learning"
    ],
    "recommended": false,
    "tag": "Port 8126",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "containrrr/watchtower:latest",
    "port": 8126,
    "desc": "Automatically keeps running Docker containers updated to the latest stable image."
  },
  {
    "id": "excalidraw",
    "title": "Excalidraw Whiteboard",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 3019",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "excalidraw/excalidraw:latest",
    "port": 3019,
    "desc": "Virtual collaborative whiteboard for sketching architecture diagrams and notes."
  },
  {
    "id": "drawio",
    "title": "draw.io (diagrams.net)",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Network & Ad-Block",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8127",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "jgraph/drawio:latest",
    "port": 8127,
    "desc": "Full offline flowchart, UML, and home network topology diagram editor."
  },
  {
    "id": "searxng",
    "title": "SearXNG (Private Metasearch Engine)",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block"
    ],
    "recommended": false,
    "tag": "Port 8128",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "searxng/searxng:latest",
    "port": 8128,
    "desc": "Aggregate results from 70+ search engines with zero tracking, profiling, or ads."
  },
  {
    "id": "whoogle",
    "title": "Whoogle Search",
    "category": "Network & Ad-Block",
    "categories": [
      "Network & Ad-Block"
    ],
    "recommended": false,
    "tag": "Port 5002",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "benbusby/whoogle-search:latest",
    "port": 5002,
    "desc": "Get Google search results without ads, JavaScript tracking, AMP links, or cookies."
  },
  {
    "id": "privategpt_lite",
    "title": "Open-Notebook (Self-Hosted NotebookLM)",
    "category": "AI & Machine Learning",
    "categories": [
      "AI & Machine Learning",
      "Cloud & Files",
      "Security & Tools",
      "Knowledge & Wiki"
    ],
    "recommended": false,
    "tag": "Port 8502",
    "ram": "8 GB+",
    "hardware_tier": "high_end",
    "image": "lfnovo/open_notebook:latest",
    "port": 8502,
    "desc": "Privacy-focused alternative to NotebookLM for researching PDFs, notes, and links.",
    "hardware_note": "8GB+ RAM"
  },
  {
    "id": "archivebox",
    "title": "ArchiveBox (Personal Wayback Machine)",
    "category": "Knowledge & Wiki",
    "categories": [
      "Knowledge & Wiki",
      "Cloud & Files",
      "Security & Tools"
    ],
    "recommended": false,
    "tag": "Port 8129",
    "ram": "128 MB",
    "hardware_tier": "old_friendly",
    "image": "archivebox/archivebox:latest",
    "port": 8129,
    "desc": "Save complete offline copies of websites in HTML, PDF, screenshot, and WARC formats."
  }
];

    const APP_CATEGORIES = [
      "All",
      "Recommended",
      "Old Laptop Friendly",
      "Higher-End / Gaming & AI",
      "Knowledge & Wiki",
      "Gaming Servers",
      "AI & Machine Learning",
      "Media & Photos",
      "Cloud & Files",
      "Network & Ad-Block",
      "Smart Home",
      "Security & Tools"
    ];

    const SERVER_ROLE_PRESETS = [
      {
        id: "media_box",
        title: "MED Home Media & Movies Box",
        badge: "Most Popular",
        ram_gb: 4,
        desc: "Jellyfin, Immich Photos, Navidrome Music, qBittorrent & FileBrowser with Intel GPU transcoding.",
        apps: ["jellyfin", "immich", "navidrome", "qbittorrent", "filebrowser"],
        vaapi: true,
        samba: true
      },
      {
        id: "family_cloud",
        title: "CLD Private Family Cloud & Backup",
        badge: "Google Drive Replacement",
        ram_gb: 4,
        desc: "Nextcloud, Immich Photos, Vaultwarden Passwords, Syncthing & Paperless-ngx Document OCR.",
        apps: ["nextcloud", "immich", "vaultwarden", "syncthing", "paperless", "filebrowser"],
        vaapi: false,
        samba: true
      },
      {
        id: "adblock_vpn",
        title: " Whole-Home Ad-Blocker & VPN",
        badge: "Runs on 2GB RAM",
        ram_gb: 2,
        desc: "Pi-hole, AdGuard Home, WireGuard VPN (wg-easy), Uptime Kuma & Vaultwarden.",
        apps: ["pihole", "adguardhome", "wg_easy", "uptimekuma", "vaultwarden"],
        pihole_dns: true
      },
      {
        id: "offline_wiki",
        title: "ZIM Offline Wikipedia & Survival Box",
        badge: "100% Air-Gapped Ready",
        ram_gb: 2,
        desc: "Kiwix Offline Wikipedia, WikiMed Medical Guide, iFixit Manuals, Calibre-Web eBooks & BookStack.",
        apps: ["kiwix", "calibreweb", "bookstack", "stirlingpdf", "filebrowser"],
        zims: ["wikipedia_en_simple", "wikipedia_en_medicine", "ifixit_en_all"]
      },
      {
        id: "old_laptop_starter",
        title: "FAST Lightweight Old Laptop Starter",
        badge: "2GB–4GB RAM Friendly",
        ram_gb: 2,
        desc: "Tuned for older dual-core laptops: ZRAM compression, CPU powersave governor, Kiwix, Pi-hole & FileBrowser.",
        apps: ["kiwix", "pihole", "vaultwarden", "filebrowser"],
        zram: true,
        governor: "powersave"
      },
      {
        id: "ai_gaming_pro",
        title: "AI Local AI & Gaming Server",
        badge: "8GB–16GB+ RAM",
        ram_gb: 8,
        desc: "Ollama + Open WebUI (Llama/DeepSeek), Minecraft Java & Bedrock Servers, Jellyfin & Portainer.",
        apps: ["ollama_webui", "minecraft", "minecraft_bedrock", "jellyfin", "portainer", "uptimekuma"],
        governor: "schedutil"
      }
    ];

    const KIWIX_ZIM_PACKS = [
      { id: "wikipedia_en_simple", label: "Wikipedia Simple English Illustrated (2.4 GB — Fast & Complete)" },
      { id: "wikipedia_en_medicine", label: "WikiMed Clinical & Emergency Medical Encyclopedia (1.8 GB)" },
      { id: "ifixit_en_all", label: "iFixit Complete Laptop, Phone & Appliance Repair Guides (3.6 GB)" },
      { id: "wikipedia_en_nopic", label: "Wikipedia Full English — All 6.8M Articles, No Pictures (56 GB)" },
      { id: "wikipedia_en_full_maxi", label: "Wikipedia Full English — Complete Edition with All HD Images (102 GB)" },
      { id: "gutenberg_en_all", label: "Project Gutenberg 70,000+ Classic Books Library (72 GB)" }
    ];

    const QUICK_HELP_ITEMS = [
      { title: "USB drive is open in File Explorer or locked", desc: "Close any open folder windows on the USB drive, pause antivirus scans on that drive letter, and click Refresh drives.", action: "scan-usb", btn: "Refresh USB" },
      { title: "Administrator permission needed before flashing", desc: "Formatting and writing a bootable USB drive requires Administrator permission on Windows or sudo on macOS and Linux.", action: "elevate-admin", btn: "Run as Admin" },
      { title: "Unplugging Ethernet cable drops connection", desc: "Turn on 'Seamless Ethernet-to-Wi-Fi failover' in Network advanced options so Wi-Fi stays awake and takes over right away.", action: "jump-setup-tab", tab: "network", btn: "Open Network" },
      { title: "Combining two servers that already have files", desc: "Choose 'Keep all files on both computers' in Multi-computer options so both drives merge into one shared folder without deleting anything.", action: "jump-setup-tab", tab: "network", btn: "Multi-Server Options" }
    ];

    const SETUP_TABS = [
      { id: "network", label: "Network" },
      { id: "apps", label: "Apps & Store" },
      { id: "storage", label: "Laptop & USB" },
      { id: "power", label: "Power" },
      { id: "finish", label: "Finish" }
    ];

    const SETTINGS_TABS = [
      { id: "system", label: "System", icon: "" },
      { id: "network", label: "Network & internet", icon: "" },
      { id: "apps", label: "Apps & store", icon: "" },
      { id: "storage", label: "Laptop & USB flash", icon: "" },
      { id: "power", label: "Power & battery", icon: "" },
      { id: "config", label: "Configuration & LAN", icon: "" }
    ];

    let shouldAnimateTab = true;


    const DEFAULT_APP_CONFIG_TEMPLATES = {
      minecraft: {
        "server.properties": "motd=Instant Server Minecraft World\ngamemode=survival\ndifficulty=normal\nmax-players=20\nview-distance=10\nsimulation-distance=10\nonline-mode=true\nenable-rcon=true\nrcon.port=25575\npvp=true\nspawn-protection=0\nallow-flight=true\n",
        "ops.json": "[\n  {\n    \"uuid\": \"00000000-0000-0000-0000-000000000000\",\n    \"name\": \"admin\",\n    \"level\": 4,\n    \"bypassesPlayerLimit\": true\n  }\n]",
        "whitelist.json": "[]",
        "paper-global.yml": "chunk-loading-advanced:\n  auto-config-send-distance: true\nproxies:\n  velocity:\n    enabled: false\n"
      },
      minecraft_bedrock: {
        "server.properties": "server-name=Instant Server Bedrock\ngamemode=survival\ndifficulty=normal\nmax-players=16\nonline-mode=true\nview-distance=12\ntick-distance=4\n"
      },
      nextcloud: {
        "config.php": "<?php\n$CONFIG = array (\n  'trusted_domains' => array (0 => 'localhost', 1 => 'instant-server.local', 2 => '*.local', 3 => '192.168.*.*'),\n  'memcache.local' => '\\\\OC\\\\Memcache\\\\APCu',\n  'maintenance_window_start' => 3,\n);\n"
      },
      pihole: {
        "pihole-FTL.conf": "PRIVACYLEVEL=0\nRATE_LIMIT=1000/60\nMAXDBDAYS=90\n",
        "adlists.list": "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts\nhttps://big.oisd.nl/domainswild\n"
      },
      adguardhome: {
        "AdGuardHome.yaml": "http:\n  address: 0.0.0.0:3000\ndns:\n  bind_hosts:\n    - 0.0.0.0\n  port: 53\n  upstream_dns:\n    - https://dns.cloudflare.com/dns-query\n"
      },
      wg_easy: {
        "wg0.env": "WG_HOST=instant-server.local\nWG_PORT=51820\nWG_DEFAULT_DNS=1.1.1.1,9.9.9.9\n"
      },
      homeassistant: {
        "configuration.yaml": "default_config:\nhttp:\n  use_x_forwarded_for: true\n  trusted_proxies:\n    - 127.0.0.1\n    - 192.168.0.0/16\n"
      },
      frigate: {
        "config.yml": "mqtt:\n  enabled: false\ndetectors:\n  ov:\n    type: openvino\n    device: AUTO\n"
      },
      jellyfin: {
        "encoding.xml": "<EncodingOptions>\n  <HardwareAccelerationType>vaapi</HardwareAccelerationType>\n  <VaapiDevice>/dev/dri/renderD128</VaapiDevice>\n</EncodingOptions>\n"
      },
      ollama_webui: {
        "models.txt": "llama3.2:3b\ndeepseek-r1:7b\nqwen2.5-coder:7b\n"
      }
    };