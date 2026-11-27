import { useQuery } from "@tanstack/react-query";
import { api, type PublicAccess } from "./api";

export type JoinScope = "internet" | "public" | "lan" | "local";

export interface PanelAddress {
  public: string;
  lan: string;
  // hosted is true on a machine that's on the internet directly (a VPS):
  // its own address works for everyone and nothing needs forwarding.
  hosted: boolean;
  // firewall says where to open a port in the cloud provider's firewall,
  // with {port} for the port.
  firewall?: string;
  // tip is one more recommendation for the cloud provider's console.
  tip?: string;
}

export const useAddress = () =>
  useQuery({ queryKey: ["address"], queryFn: () => api.get<PanelAddress>("/address"), staleTime: Infinity });

export interface JoinAddress {
  address: string;
  scope: JoinScope;
}

const isLocal = (host: string) => host === "localhost" || host === "127.0.0.1" || host === "::1" || host === "[::1]";

function format(host: string, port: number) {
  return port === 25565 ? host : `${host}:${port}`;
}

// useJoinAddress works out what players type into Minecraft: the server's
// forwarded or tunnelled address if it has one, else the address set with
// --public-address, else the panel's own hostname (the servers run on the same
// machine), else this machine's address on the local network.
export function useJoinAddress(port: number, pub?: PublicAccess): JoinAddress {
  const { data } = useAddress();
  if (pub) return { address: pub.address.replace(/:25565$/, ""), scope: "internet" };
  if (data?.public) return { address: format(data.public, port), scope: "public" };
  const host = window.location.hostname;
  if (!isLocal(host)) return { address: format(host, port), scope: "public" };
  if (data?.lan) return { address: format(data.lan, port), scope: data.hosted ? "public" : "lan" };
  return { address: format("localhost", port), scope: "local" };
}

export const scopeHint: Record<JoinScope, string> = {
  internet: "",
  public: "",
  lan: "Works for players on your network. To let friends elsewhere join, make the server public below.",
  local: "Only works on this computer.",
};
