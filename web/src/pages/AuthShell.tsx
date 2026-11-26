import { type ReactNode } from "react";
import { Mark } from "../layout";

export function AuthShell({ title, subtitle, children }: { title: string; subtitle?: ReactNode; children: ReactNode }) {
  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-12">
      <div className="grid-bg pointer-events-none absolute inset-0" aria-hidden="true" />
      <div className="relative w-full max-w-[22rem]">
        <div className="mb-8 flex flex-col items-center text-center">
          <Mark className="h-12 w-12" />
          <h1 className="mt-6 text-2xl font-semibold tracking-tight">{title}</h1>
          {subtitle && <p className="mt-2 text-sm leading-relaxed text-muted">{subtitle}</p>}
        </div>
        <div className="rounded-xl border border-line bg-surface p-6 shadow-pop">{children}</div>
        <p className="mt-6 text-center text-[13px] text-subtle">minectl · self-hosted Minecraft servers</p>
      </div>
    </div>
  );
}
