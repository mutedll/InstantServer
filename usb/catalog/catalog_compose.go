// ============================================================================
// FILE: usb/catalog/catalog_compose.go
// PURPOSE: Generates multi-service Docker Compose YAML definitions (including
//
//	Immich, Paperless-ngx, Nextcloud, and Minecraft Server Studio) with
//	user port, volume, and environment variable overrides.
//
// ============================================================================
package catalog

import (
	"fmt"
	"sort"
	"strings"
)

func BuildVerifiedAppComposeYAML(appID string, fallbackImage string, fallbackHostPort int, mcCfg *MinecraftServerConfig) string {
	return BuildVerifiedAppComposeWithOverride(appID, fallbackImage, fallbackHostPort, mcCfg, nil)
}

func BuildImmichMultiContainerComposeYAML(hostPort int) string {
	if hostPort <= 0 {
		hostPort = 2283
	}
	return fmt.Sprintf(`name: immich
services:
  immich_server:
    image: ghcr.io/immich-app/immich-server:release
    container_name: immich_server
    restart: unless-stopped
    depends_on:
      - immich_redis
      - immich_postgres
    devices:
      - /dev/dri:/dev/dri
    environment:
      - TZ=UTC
      - DB_HOSTNAME=immich_postgres
      - DB_USERNAME=postgres
      - DB_PASSWORD=immichpostgrespass
      - DB_DATABASE_NAME=immich
      - REDIS_HOSTNAME=immich_redis
    ports:
      - "%d:2283"
    volumes:
      - /DATA/AppData/immich/upload:/usr/src/app/upload
      - /etc/localtime:/etc/localtime:ro
  immich_machine_learning:
    image: ghcr.io/immich-app/immich-machine-learning:release
    container_name: immich_machine_learning
    restart: unless-stopped
    volumes:
      - /DATA/AppData/immich/model-cache:/cache
  immich_redis:
    image: docker.io/valkey/valkey:8-alpine
    container_name: immich_redis
    restart: unless-stopped
  immich_postgres:
    image: ghcr.io/immich-app/postgres:17-vectorchord0.4.3-pgvectors0.3.0
    container_name: immich_postgres
    restart: unless-stopped
    environment:
      - POSTGRES_PASSWORD=immichpostgrespass
      - POSTGRES_USER=postgres
      - POSTGRES_DB=immich
      - POSTGRES_INITDB_ARGS=--data-checksums
    volumes:
      - /DATA/AppData/immich/postgres:/var/lib/postgresql/data
`, hostPort)
}

func BuildPaperlessMultiContainerComposeYAML(hostPort int) string {
	if hostPort <= 0 {
		hostPort = 8000
	}
	return fmt.Sprintf(`name: paperless
services:
  paperless:
    image: ghcr.io/paperless-ngx/paperless-ngx:latest
    container_name: paperless
    restart: unless-stopped
    depends_on:
      - paperless_redis
    environment:
      - PAPERLESS_REDIS=redis://paperless_redis:6379
      - PAPERLESS_TIME_ZONE=UTC
      - USERMAP_UID=1000
      - USERMAP_GID=1000
    ports:
      - "%d:8000"
    volumes:
      - /DATA/AppData/paperless/data:/usr/src/paperless/data
      - /DATA/AppData/paperless/media:/usr/src/paperless/media
      - /DATA/AppData/paperless/export:/usr/src/paperless/export
      - /DATA/AppData/paperless/consume:/usr/src/paperless/consume
  paperless_redis:
    image: docker.io/valkey/valkey:8-alpine
    container_name: paperless_redis
    restart: unless-stopped
    volumes:
      - /DATA/AppData/paperless/redisdata:/data
`, hostPort)
}

func BuildBookStackMultiContainerComposeYAML(hostPort int) string {
	if hostPort <= 0 {
		hostPort = 6875
	}
	return fmt.Sprintf(`name: bookstack
services:
  bookstack:
    image: lscr.io/linuxserver/bookstack:latest
    container_name: bookstack
    restart: unless-stopped
    depends_on:
      - bookstack_db
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=UTC
      - APP_URL=http://localhost:%d
      - APP_KEY=base64:CClA6bWC3qhP6yMVQECyTblYtj7uAxOh0dDpi0BcZ5w=
      - DB_HOST=bookstack_db
      - DB_PORT=3306
      - DB_USERNAME=bookstack
      - DB_PASSWORD=bookstackdbpass
      - DB_DATABASE=bookstack
    ports:
      - "%d:80"
    volumes:
      - /DATA/AppData/bookstack/config:/config
  bookstack_db:
    image: lscr.io/linuxserver/mariadb:latest
    container_name: bookstack_db
    restart: unless-stopped
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=UTC
      - MYSQL_ROOT_PASSWORD=bookstackrootpass
      - MYSQL_DATABASE=bookstack
      - MYSQL_USER=bookstack
      - MYSQL_PASSWORD=bookstackdbpass
    volumes:
      - /DATA/AppData/bookstack/db:/config
`, hostPort, hostPort)
}

func BuildVerifiedAppComposeWithOverride(appID string, fallbackImage string, fallbackHostPort int, mcCfg *MinecraftServerConfig, override *AppOverride) string {
	if appID == "minecraft" {
		if mcCfg != nil {
			return BuildMinecraftComposeYAML(*mcCfg)
		}
		return BuildMinecraftComposeYAML(DefaultMinecraftConfig())
	}
	if appID == "immich" {
		hp := 2283
		if override != nil && override.HostPort > 0 {
			hp = override.HostPort
		}
		return BuildImmichMultiContainerComposeYAML(hp)
	}
	if appID == "paperless" {
		hp := 8000
		if override != nil && override.HostPort > 0 {
			hp = override.HostPort
		}
		return BuildPaperlessMultiContainerComposeYAML(hp)
	}
	if appID == "bookstack" {
		hp := 6875
		if override != nil && override.HostPort > 0 {
			hp = override.HostPort
		}
		return BuildBookStackMultiContainerComposeYAML(hp)
	}

	spec, exists := VerifiedCoreAppSpecs[appID]
	if !exists {
		port := fallbackHostPort
		if override != nil && override.HostPort > 0 {
			port = override.HostPort
		}
		if port <= 0 {
			port = 8080
		}
		img := fallbackImage
		if img == "" {
			img = "alpine:latest"
		}
		spec = AppSpec{
			ID:           appID,
			Image:        img,
			HostPort:     port,
			InternalPort: port,
			Volumes:      []string{fmt.Sprintf("/DATA/AppData/%s:/data", appID)},
		}
	}

	hostPort := spec.HostPort
	if override != nil && override.HostPort > 0 {
		hostPort = override.HostPort
	}
	intPort := spec.InternalPort
	if intPort <= 0 {
		intPort = hostPort
	}

	var portLines []string
	if spec.UDPOnly {
		portLines = append(portLines, fmt.Sprintf(`      - "%d:%d/udp"`, hostPort, intPort))
	} else {
		portLines = append(portLines, fmt.Sprintf(`      - "%d:%d"`, hostPort, intPort))
	}
	for _, ep := range spec.ExtraPorts {
		portLines = append(portLines, fmt.Sprintf(`      - "%s"`, ep))
	}

	var capSection string
	if len(spec.CapAdd) > 0 {
		var caps []string
		for _, c := range spec.CapAdd {
			caps = append(caps, fmt.Sprintf("      - %s", c))
		}
		capSection = "\n    cap_add:\n" + strings.Join(caps, "\n")
	}

	var sysctlSection string
	if len(spec.Sysctls) > 0 {
		var sc []string
		for _, s := range spec.Sysctls {
			sc = append(sc, fmt.Sprintf("      - %s", s))
		}
		sysctlSection = "\n    sysctls:\n" + strings.Join(sc, "\n")
	}

	var devSection string
	if len(spec.Devices) > 0 {
		var dv []string
		for _, d := range spec.Devices {
			dv = append(dv, fmt.Sprintf("      - %s", d))
		}
		devSection = "\n    devices:\n" + strings.Join(dv, "\n")
	}

	var shmSection string
	if strings.TrimSpace(spec.ShmSize) != "" {
		shmSection = fmt.Sprintf("\n    shm_size: %q", strings.TrimSpace(spec.ShmSize))
	}

	var cmdSection string
	if spec.Command != "" {
		cmdSection = fmt.Sprintf("\n    command: %q", spec.Command)
	}

	var memSection string
	if override != nil && strings.TrimSpace(override.MemoryLimit) != "" {
		memSection = fmt.Sprintf("\n    mem_limit: %s", strings.TrimSpace(override.MemoryLimit))
	}

	mergedEnv := map[string]string{
		"TZ":   "UTC",
		"PUID": "1000",
		"PGID": "1000",
	}
	for k, v := range spec.Environment {
		mergedEnv[k] = v
	}
	if override != nil {
		for k, v := range override.EnvVars {
			mergedEnv[k] = v
		}
	}

	envKeys := make([]string, 0, len(mergedEnv))
	for k := range mergedEnv {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	var envLines []string
	for _, k := range envKeys {
		envLines = append(envLines, fmt.Sprintf("      - %s=%s", k, mergedEnv[k]))
	}

	var volLines []string
	for _, v := range spec.Volumes {
		volLines = append(volLines, fmt.Sprintf("      - %s", v))
	}

	return fmt.Sprintf(`name: %s
services:
  %s:
    image: %s
    container_name: %s
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true%s%s%s%s%s%s
    environment:
%s
    ports:
%s
    volumes:
%s
`, spec.ID, spec.ID, spec.Image, spec.ID, shmSection, memSection, cmdSection, capSection, sysctlSection, devSection, strings.Join(envLines, "\n"), strings.Join(portLines, "\n"), strings.Join(volLines, "\n"))
}
