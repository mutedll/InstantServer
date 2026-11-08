// src/instant_server/ui/app_studio.js
(function (root) {
  "use strict";

  function createAppStudioState() {
    return {
      selectedAppId: "minecraft",
      activeStudioTab: "config_files", // "config_files" | "env_limits" | "rcon_logs"
      selectedFileName: "server.properties",
      fileEditorContent: "",
      appOverrides: {}
    };
  }

  function ensureAppOverrideEntry(studioState, appObj) {
    if (!appObj || !appObj.id) return null;
    if (!studioState.appOverrides[appObj.id]) {
      const defaults = (typeof root.getAppDefaultConfigFiles === "function")
        ? root.getAppDefaultConfigFiles(appObj)
        : { ".env": `TZ=UTC\nPUID=1000\nPGID=1000\nAPP_PORT=${appObj.port || 8080}\n` };
      studioState.appOverrides[appObj.id] = {
        host_port: appObj.port || 8080,
        memory_limit: appObj.id === "minecraft" ? "4g" : (appObj.id === "ollama_webui" ? "8g" : ""),
        env_vars: {},
        config_files: defaults
      };
    }
    return studioState.appOverrides[appObj.id];
  }

  root.InstantAppStudio = {
    createAppStudioState,
    ensureAppOverrideEntry
  };
})(typeof window !== "undefined" ? window : globalThis);