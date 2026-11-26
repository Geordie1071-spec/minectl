// The panel usually runs on the same machine as the servers, so the panel's
// hostname is the best guess for what players type into Minecraft.
export function joinAddress(port: number): string {
  const host = window.location.hostname;
  const shown = host === "localhost" || host === "127.0.0.1" ? "<server-ip>" : host;
  return port === 25565 ? shown : `${shown}:${port}`;
}
