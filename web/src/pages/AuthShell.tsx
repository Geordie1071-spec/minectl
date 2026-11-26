import { type ReactNode } from "react";
import { Logo } from "../layout";

const points = [
  "Spin up Paper, Fabric, Forge or any modpack in a few clicks",
  "Live console, players, backups and usage graphs",
  "Share servers with friends, with the right level of access",
];

export function AuthShell({ title, subtitle, children }: { title: string; subtitle?: ReactNode; children: ReactNode }) {
  return (
    <div className="grid min-h-screen lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
      <aside className="brand-panel relative hidden flex-col justify-between overflow-hidden p-12 text-white lg:flex">
        <Logo onDark className="h-9 self-start" />
        <div className="max-w-md">
          <h2 className="text-3xl leading-tight font-semibold tracking-tight">Your Minecraft servers, in one place.</h2>
          <ul className="mt-8 space-y-4">
            {points.map((p) => (
              <li key={p} className="flex gap-3 text-[15px] text-white/85">
                <span className="mt-1.5 h-2.5 w-2.5 shrink-0 bg-brand-2" aria-hidden="true" />
                {p}
              </li>
            ))}
          </ul>
        </div>
        <p className="text-sm text-white/60">Self-hosted. Your machine, your data.</p>
      </aside>
      <main className="flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm">
          <div className="mb-10 lg:hidden">
            <Logo className="h-8" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          {subtitle && <p className="mt-2 text-sm leading-relaxed text-muted">{subtitle}</p>}
          <div className="mt-8">{children}</div>
        </div>
      </main>
    </div>
  );
}
