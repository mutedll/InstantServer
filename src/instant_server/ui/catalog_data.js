// src/instant_server/ui/catalog_data.js
(function (root) {
  "use strict";

  const VERIFIED_CONTAINER_SPECS = {
    jellyfin: { intPort: 8096, devices: ["/dev/dri:/dev/dri"], configTemplates: {
      "system.xml": `<ServerConfiguration>\n  <EnableMetrics>false</EnableMetrics>\n  <EnableUPnP>false</EnableUPnP>\n  <PublicPort>8096</PublicPort>\n</ServerConfiguration>`,
      "encoding.xml": `<EncodingOptions>\n  <HardwareAccelerationType>vaapi</HardwareAccelerationType>\n  <VaapiDevice>/dev/dri/renderD128</VaapiDevice>\n  <EnableHardwareEncoding>true</EnableHardwareEncoding>\n</EncodingOptions>`
    }},
    plex: { intPort: 32400, devices: ["/dev/dri:/dev/dri"], env: ["VERSION=docker"], configTemplates: {
      "Preferences.xml": `<?xml version="1.0" encoding="utf-8"?>\n<Preferences HardwareAcceleratedCodecs="1"HardwareAcceleratedEncoders="1" />`
    }},
    emby: { intPort: 8096, devices: ["/dev/dri:/dev/dri"] },
    nextcloud: { intPort: 80, configTemplates: {
      "config.php": `<?php\n$CONFIG = array (\n  'trusted_domains' => array (\n    0 => 'localhost',\n    1 => 'instant-server.local',\n    2 => '*.local',\n    3 => '192.168.*.*',\n    4 => '100.*.*.*',\n  ),\n  'default_phone_region' => 'US',\n  'memcache.local' => '\\\\OC\\\\Memcache\\\\APCu',\n  'maintenance_window_start' => 3,\n);\n`
    }},
    pihole: { intPort: 80, extraPorts: ["53:53/tcp", "53:53/udp"], capAdd: ["NET_ADMIN"], env: ["FTLCONF_dns_listeningMode=all"], configTemplates: {
      "pihole-FTL.conf": `PRIVACYLEVEL=0\nRATE_LIMIT=1000/60\nDBINTERVAL=60.0\nMAXDBDAYS=90\n`,
      "adlists.list": `https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts\nhttps://big.oisd.nl/domainswild\n`
    }},
    adguardhome: { intPort: 3000, extraPorts: ["53:53/tcp", "53:53/udp"], capAdd: ["NET_ADMIN"], configTemplates: {
      "AdGuardHome.yaml": `http:\n  address: 0.0.0.0:3000\ndns:\n  bind_hosts:\n    - 0.0.0.0\n  port: 53\n  upstream_dns:\n    - https://dns.cloudflare.com/dns-query\n    - https://dns.quad9.net/dns-query\n`
    }},
    wg_easy: { intPort: 51821, extraPorts: ["51820:51820/udp"], capAdd: ["NET_ADMIN", "SYS_MODULE"], sysctls: ["net.ipv4.ip_forward=1", "net.ipv4.conf.all.src_valid_mark=1"], configTemplates: {
      "wg0.env": `WG_HOST=instant-server.local\nWG_PORT=51820\nWG_DEFAULT_DNS=1.1.1.1,9.9.9.9\nWG_ALLOWED_IPS=0.0.0.0/0,::/0\n`
    }},
    nginxproxymanager: { intPort: 81, extraPorts: ["80:80/tcp", "443:443/tcp"] },
    kiwix: { intPort: 8080, command: "*.zim" },
    immich: { intPort: 2283, devices: ["/dev/dri:/dev/dri"], configTemplates: {
      "immich.env": `UPLOAD_LOCATION=/DATA/AppData/immich/upload\nIMMICH_TELEMETRY_INCLUDE=none\nTZ=UTC\n`
    }},
    homeassistant: { intPort: 8123, configTemplates: {
      "configuration.yaml": `default_config:\n\nhttp:\n  use_x_forwarded_for: true\n  trusted_proxies:\n    - 127.0.0.1\n    - 172.16.0.0/12\n    - 192.168.0.0/16\n\nautomation: !include automations.yaml\nscript: !include scripts.yaml\nscene: !include scenes.yaml\n`
    }},
    frigate: { intPort: 5000, extraPorts: ["8554:8554/tcp", "8555:8555/tcp", "8555:8555/udp"], devices: ["/dev/dri:/dev/dri"], configTemplates: {
      "config.yml": `mqtt:\n  enabled: false\ndetectors:\n  ov:\n    type: openvino\n    device: AUTO\ncameras:\n  front_door:\n    enabled: false\n    ffmpeg:\n      inputs:\n        - path: rtsp://127.0.0.1:8554/front_door\n          roles:\n            - detect\n            - record\n`
    }},
    vaultwarden: { intPort: 80, configTemplates: {
      "config.json": `{\n  "signups_allowed": true,\n  "invitations_allowed": true,\n  "websocket_enabled": true,\n  "web_vault_enabled": true\n}`
    }},
    stirlingpdf: { intPort: 8080, configTemplates: {
      "settings.yml": `security:\n  enableLogin: false\nsystem:\n  defaultLocale: en-US\n`
    }},
    filebrowser: { intPort: 80 },
    gitea: { intPort: 3000, extraPorts: ["2222:22/tcp"], configTemplates: {
      "app.ini": `[server]\nHTTP_PORT = 3000\nSSH_PORT = 2222\nDISABLE_REGISTRATION = false\n`
    }},
    bookstack: { intPort: 80 },
    audiobookshelf: { intPort: 80 },
    mealie: { intPort: 9000 },
    dozzle: { intPort: 8080 },
    ollama_webui: { intPort: 8080, configTemplates: {
      "models.txt": `# Pre-pull recommended local AI models:\nllama3.2:3b\ndeepseek-r1:7b\nqwen2.5-coder:7b\n`
    }},
    open_webui: { intPort: 8080 },
    minecraft: { intPort: 25565, extraPorts: ["127.0.0.1:25575:25575/tcp"], configTemplates: {
      "server.properties": `motd=Instant Server Minecraft World\ngamemode=survival\ndifficulty=normal\nmax-players=20\nview-distance=10\nsimulation-distance=10\nonline-mode=true\nenable-rcon=true\nrcon.port=25575\npvp=true\nspawn-protection=0\nallow-flight=true\n`,
      "ops.json": `[\n  {\n    "uuid": "00000000-0000-0000-0000-000000000000",\n    "name": "admin",\n    "level": 4,\n    "bypassesPlayerLimit": true\n  }\n]`,
      "whitelist.json": `[]`,
      "paper-global.yml": `chunk-loading-advanced:\n  auto-config-send-distance: true\nproxies:\n  velocity:\n    enabled: false\n`
    }},
    minecraft_bedrock: { intPort: 19132, udpOnly: true, env: ["EULA=TRUE", "GAMEMODE=survival", "DIFFICULTY=normal"], configTemplates: {
      "server.properties": `server-name=Instant Server Bedrock\ngamemode=survival\ndifficulty=normal\nmax-players=16\nonline-mode=true\nview-distance=12\ntick-distance=4\n`
    }},
    valheim: { intPort: 2456, udpOnly: true, extraPorts: ["2457:2457/udp"] },
    palworld: { intPort: 8211, udpOnly: true, extraPorts: ["27015:27015/udp"], configTemplates: {
      "PalWorldSettings.ini": `[/Script/Pal.PalGameWorldSettings]\nOptionSettings=(Difficulty=None,DayTimeSpeedRate=1.000000,NightTimeSpeedRate=1.000000,ExpRate=1.000000,PalCaptureRate=1.000000,CoopPlayerMaxNum=4,ServerPlayerMaxNum=16,PublicPort=8211)\n`
    }},
    factorio: { intPort: 34197, udpOnly: true, extraPorts: ["27015:27015/tcp"] },
    terraria: { intPort: 7777 },
    satisfactory: { intPort: 7777, extraPorts: ["7777:7777/udp"] },
    cs2_server: { intPort: 27015, extraPorts: ["27015:27015/udp"] }
  };

  function getAppDefaultConfigFiles(app) {
    if (!app) return {};
    const spec = VERIFIED_CONTAINER_SPECS[app.id];
    if (spec && spec.configTemplates) {
      return Object.assign({}, spec.configTemplates);
    }
    return {
      ".env": `TZ=UTC\nPUID=1000\nPGID=1000\nAPP_PORT=${app.port || 8080}\n`,
      "README-AppData.txt": `Persistent host folder: /DATA/AppData/${app.id}\nAll files edited here are mounted directly into ${app.title} (${app.image || app.id}).\n`
    };
  }

  root.INSTANT_SERVER_CONTAINER_SPECS = VERIFIED_CONTAINER_SPECS;
  root.getAppDefaultConfigFiles = getAppDefaultConfigFiles;
})(typeof window !== "undefined" ? window : globalThis);