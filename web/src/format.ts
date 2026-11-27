export function formatMB(mb: number): string {
  if (mb >= 1024 && mb % 1024 === 0) return `${mb / 1024} GB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

// formatBytes uses 1024-based units, like formatMB and the memory settings,
// so a 2.5 GB container limit doesn't show up as 2.7 GB.
export function formatBytes(n: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n < 10 && i > 0 ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
}

export function timeAgo(iso: string | undefined): string {
  if (!iso || iso.startsWith("0001")) return "never";
  const s = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 45) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m} min ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h} h ago`;
  const d = Math.round(h / 24);
  if (d < 30) return `${d} d ago`;
  return new Date(iso).toLocaleDateString();
}

export function dateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

export function compact(n: number): string {
  return new Intl.NumberFormat(undefined, { notation: "compact" }).format(n);
}

export const typeLabels: Record<string, string> = {
  paper: "Paper",
  vanilla: "Vanilla",
  spigot: "Spigot",
  fabric: "Fabric",
  forge: "Forge",
  neoforge: "NeoForge",
  quilt: "Quilt",
};

export const isModded = (type: string) => ["fabric", "forge", "neoforge", "quilt"].includes(type);

// Strips the itzg/Minecraft log prefix and ANSI codes for display.
// eslint-disable-next-line no-control-regex
const ansi = /\u001b\[[0-9;]*m/g;
export const cleanLog = (line: string) => line.replace(ansi, "");
