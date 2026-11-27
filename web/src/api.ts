// Types mirror the Go API in internal/api and internal/domain.

export type Role = "admin" | "operator" | "viewer";

export interface User {
  id: number;
  username: string;
  is_admin: boolean;
  created_at: string;
}

export interface Grant {
  user_id: number;
  server: string;
  role: Exclude<Role, "admin">;
}

export interface Me {
  user: User;
  grants: Grant[];
}

export interface UserWithGrants extends User {
  grants: Grant[];
}

export interface Mod {
  id: string;
  mod_id: string;
  mod_name: string;
  version_id: string;
  enabled: boolean;
}

export interface JobEvent {
  seq: number;
  time: string;
  type: "progress" | "stage" | "log" | "info" | "warn" | "done" | "error";
  percent?: number;
  message?: string;
}

export interface Job {
  id: string;
  kind: string;
  server: string;
  user_id?: number;
  status: "running" | "succeeded" | "failed";
  error?: string;
  created_at: string;
  finished_at?: string;
  events?: JobEvent[];
}

export interface Server {
  name: string;
  type: string;
  version: string;
  status: "running" | "stopped" | "creating" | "error" | string;
  port: number;
  memory_mb: number;
  cpu_cores: number;
  modpack?: { source: "modrinth" | "curseforge"; id: string; version?: string };
  mods: Mod[];
  created_at: string;
  last_started_at: string;
  role: Role;
  job?: Job;
  data_dir?: string;
  backups?: number;
  backup_schedule?: "hourly" | "6h" | "daily";
  next_backup_at?: string;
  last_crash_at?: string;
  last_crash_reason?: string;
  public?: PublicAccess;
}

// PublicAccess is how friends outside the local network reach a server: the
// router forwards its port, or a playit.gg tunnel relays it.
export interface PublicAccess {
  method: "upnp" | "nat-pmp" | "playit";
  address: string;
}

export interface Properties {
  motd?: string;
  max_players?: number;
  difficulty?: string;
  gamemode?: string;
  view_distance?: number;
  pvp?: boolean;
  hardcore?: boolean;
  online_mode?: boolean;
  whitelist?: boolean;
  allow_flight?: boolean;
}

export interface PanelSettings {
  discord_webhook_set: boolean;
  backup_retention: number;
  playit_linked: boolean;
}

export interface Backup {
  id: string;
  size_bytes: number;
  created_at: string;
}

export interface Players {
  online: number;
  max: number;
  names: string[];
}

export interface Stats {
  cpu_percent: number;
  memory_bytes: number;
  memory_limit: number;
  memory_percent: number;
}

export interface Modpack {
  slug: string;
  project_id: string;
  title: string;
  description: string;
  downloads: number;
  icon_url: string;
  author: string;
  versions: string[];
}

export interface Recommendation {
  memory_mb: number;
  cpu_cores: number;
  recommended: { memory_mb: number; cpu_cores: number; reason: string };
  warnings: string[];
  type: string;
  mods: number;
  modpack?: { slug: string; version_number: string; mc_version: string; loader: string };
  host: { memory_mb: number; cpus: number };
}

export interface APIToken {
  id: number;
  name: string;
  created_at: string;
  last_used_at?: string;
}

export interface AuditEntry {
  id: number;
  time: string;
  username: string;
  action: string;
  server?: string;
  detail?: string;
  ip?: string;
}

export interface AuditPage {
  entries: AuditEntry[];
  total: number;
  page: number;
  pages: number;
}

// KnownPlayer is someone who has joined a server or is on its op, whitelist
// or ban lists, read from the server's files.
export interface KnownPlayer {
  name: string;
  uuid?: string;
  op: boolean;
  whitelisted: boolean;
  banned: boolean;
  ban_reason?: string;
  last_seen?: string;
}

export class ApiError extends Error {
  status: number;
  body: Record<string, unknown>;
  constructor(status: number, message: string, body: Record<string, unknown>) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch("/api/v1" + path, {
    method,
    credentials: "same-origin",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(res.status, (data as { error?: string }).error ?? res.statusText, data);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body ?? {}),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body ?? {}),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, body ?? {}),
  del: <T>(path: string) => request<T>("DELETE", path),
};

export const srv = (name: string) => "/servers/" + encodeURIComponent(name);

export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
