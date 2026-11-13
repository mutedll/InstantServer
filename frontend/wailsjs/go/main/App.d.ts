// frontend/wailsjs/go/main/App.d.ts
export namespace usb {
  export interface CustomAppEntry {
    id: string;
    title: string;
    category: string;
    image: string;
    port: number;
    volumes: string[];
    description: string;
    compose_yaml: string;
  }

  export interface ServerProfileConfig {
    hostname: string;
    username: string;
    password: string;
    wifi_ssid: string;
    wifi_password: string;
    enable_usb_tethering: boolean;
    disable_wifi_power_save: boolean;
    enable_mosh_server: boolean;
    enable_tailscale: boolean;
    tailscale_authkey: string;
    tailscale_mode: string;
    cluster_mode: string;
    cluster_storage_policy: string;
    cluster_confirm_wipe_secondary: string;
    device_form_factor: string;
    dashboard_choice: string;
    selected_apps: string[];
    custom_apps: CustomAppEntry[];
    install_mode: string;
    dual_boot_Allocation_gb: number;
    confirm_erase_main_drive: boolean;
    ubuntu_release: string;
    storage_layout: string;
    boot_mode: string;
    usb_partition_scheme: string;
    ignore_lid_switch: boolean;
    screen_blank_seconds: number;
    battery_charge_start_pct: number;
    battery_charge_stop_pct: number;
    ups_shutdown_battery_pct: number;
    enable_low_ram_zram: boolean;
    cpu_governor: string;
    disable_cpu_turbo_boost: boolean;
    selected_usb_device: string;
  }

  export interface USBDeviceCandidate {
    device_id: string;
    device_node: string;
    model: string;
    size_gb: number;
    bus: string;
    removable: boolean;
    mountpoints: string[];
    triple_lock_passed: boolean;
    block_reasons: string[];
  }

  export interface FlashProgressEvent {
    stage: string;
    pct: number;
    written_bytes: number;
    total_bytes: number;
    mb_s: number;
    eta_sec: number;
    crc32: string;
    sha256?: string;
    grub_patches?: number;
    error_code?: string;
    message: string;
  }
}

export function ProbeHost(): Promise<Record<string, any>>;
export function ElevateAdmin(): Promise<Record<string, any>>;
export function ScanUSBDevices(): Promise<usb.USBDeviceCandidate[]>;
export function ResetUSBDrive(devicePath: string): Promise<Record<string, any>>;
export function AddCustomApp(payload: Record<string, any>): Promise<Record<string, any>>;
export function SaveConfig(profile: usb.ServerProfileConfig): Promise<Record<string, any>>;
export function LoadConfig(): Promise<Record<string, any>>;
export function PollBuildStatus(): Promise<Record<string, any>>;
export function FlashUSBDevice(profile: usb.ServerProfileConfig, isoPath: string): Promise<Record<string, any>>;
export function PauseFlashing(): Promise<Record<string, any>>;
export function ResumeFlashing(): Promise<Record<string, any>>;
export function CancelFlashing(): Promise<Record<string, any>>;
export function DiscoverServers(hostname: string): Promise<Record<string, any>>;