import { typeLabels } from "./format";
import { useEffect, useRef, useState, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from "react";

export function cx(...parts: (string | false | null | undefined)[]) {
  return parts.filter(Boolean).join(" ");
}

type Variant = "primary" | "secondary" | "danger" | "ghost";

const variants: Record<Variant, string> = {
  primary: "bg-accent text-accent-fg shadow-sm shadow-accent/25 hover:bg-accent-hover",
  secondary: "bg-surface border border-line shadow-card hover:bg-surface-2",
  danger: "bg-danger text-white shadow-sm hover:opacity-90",
  ghost: "hover:bg-surface-2",
};

export function Button({
  variant = "secondary",
  busy,
  className,
  children,
  disabled,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; busy?: boolean }) {
  return (
    <button
      className={cx(
        "inline-flex items-center justify-center gap-2 rounded-lg px-3.5 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50",
        variants[variant],
        className,
      )}
      disabled={disabled || busy}
      {...rest}
    >
      {busy && <Spinner className="h-3.5 w-3.5" />}
      {children}
    </button>
  );
}

export function Spinner({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg className={cx("animate-spin", className)} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeOpacity="0.25" strokeWidth="3" />
      <path d="M22 12a10 10 0 0 0-10-10" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

export const inputClass =
  "rounded-lg border border-line bg-surface px-3 py-2 text-sm shadow-card placeholder:text-muted/70 focus:border-accent focus:ring-3 focus:ring-accent/15 focus:outline-none";

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={cx(inputClass, "w-full", props.className)} />;
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  // Pass a width class (e.g. w-32) to override the default full width.
  return <select {...props} className={cx(inputClass, "pr-8", /\bw-/.test(props.className ?? "") ? "" : "w-full", props.className)} />;
}

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <label className="block space-y-1">
        <span className="text-sm font-medium">{label}</span>
        {children}
      </label>
      {hint && <p className="text-xs text-muted">{hint}</p>}
    </div>
  );
}

export function Card({ title, actions, children, className }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={cx("rounded-xl border border-line bg-surface shadow-card", className)}>
      {(title || actions) && (
        <header className="flex min-h-14 items-center justify-between gap-2 border-b border-line px-5 py-3">
          <h2 className="text-sm font-semibold">{title}</h2>
          <div className="flex items-center gap-2">{actions}</div>
        </header>
      )}
      <div className="p-5">{children}</div>
    </section>
  );
}

export function Alert({ kind = "error", children }: { kind?: "error" | "warn" | "info"; children: ReactNode }) {
  const styles = {
    error: "bg-danger-soft text-danger border-danger/30",
    warn: "bg-warn-soft text-warn border-warn/30",
    info: "bg-accent-soft text-fg border-accent/30",
  }[kind];
  return <div className={cx("rounded-lg border px-3 py-2 text-sm whitespace-pre-wrap break-words", styles)}>{children}</div>;
}

const statusStyles: Record<string, string> = {
  running: "bg-ok",
  creating: "bg-warn animate-pulse",
  stopped: "bg-muted/50",
  error: "bg-danger",
};

const badgeStyles: Record<string, string> = {
  running: "bg-ok-soft text-ok",
  creating: "bg-warn-soft text-warn",
  error: "bg-danger-soft text-danger",
};

export function StatusDot({ status }: { status: string }) {
  return <span className={cx("inline-block h-2 w-2 shrink-0 rounded-full", statusStyles[status] ?? "bg-muted/50")} aria-hidden="true" />;
}

export function StatusBadge({ status, busy }: { status: string; busy?: string }) {
  return (
    <span
      className={cx(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap",
        busy ? "bg-accent-soft text-accent" : (badgeStyles[status] ?? "bg-surface-2 text-muted"),
      )}
    >
      {busy ? <Spinner className="h-3 w-3" /> : <StatusDot status={status} />}
      {busy ?? status}
    </span>
  );
}

export function Badge({ children }: { children: ReactNode }) {
  return <span className="rounded-md bg-surface-2 px-2 py-0.5 text-xs font-medium text-muted">{children}</span>;
}

export function Empty({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-line bg-surface/50 px-6 py-12 text-center">
      <p className="font-medium">{title}</p>
      {children && <div className="mt-2 text-sm text-muted">{children}</div>}
    </div>
  );
}

export function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      variant="ghost"
      className="px-2 py-0.5 text-xs"
      onClick={() => {
        navigator.clipboard?.writeText(text).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        });
      }}
    >
      {copied ? "Copied" : label}
    </Button>
  );
}

export function Modal({ open, onClose, title, children }: { open: boolean; onClose: () => void; title: string; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);
  return (
    <dialog
      ref={ref}
      onClose={onClose}
      className="m-auto w-[min(32rem,calc(100vw-2rem))] rounded-xl border border-line bg-surface p-0 text-fg shadow-pop backdrop:bg-slate-950/40 backdrop:backdrop-blur-sm"
    >
      <div className="border-b border-line px-5 py-4 font-semibold">{title}</div>
      <div className="p-5">{children}</div>
    </dialog>
  );
}

export function Meter({ value, label, alarm = true }: { value: number; label: string; alarm?: boolean }) {
  const pct = Math.max(0, Math.min(100, value));
  return (
    <div>
      <div className="mb-1 flex justify-between text-xs text-muted">
        <span>{label}</span>
        <span>{pct.toFixed(0)}%</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-surface-2" role="meter" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} aria-label={label}>
        <div className={cx("h-full rounded-full transition-all", alarm && pct > 90 ? "bg-danger" : alarm && pct > 75 ? "bg-warn" : "bg-accent")} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

export function Sparkline({ values, max }: { values: number[]; max: number }) {
  if (values.length < 2) return <div className="h-10" />;
  const w = 200;
  const h = 40;
  const top = Math.max(max, ...values, 1);
  const pts = values.map((v, i) => `${(i / (values.length - 1)) * w},${h - (v / top) * (h - 2) - 1}`).join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-10 w-full text-accent" preserveAspectRatio="none" aria-hidden="true">
      <polyline points={pts} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

const typeTiles: Record<string, string> = {
  paper: "from-[#4687ff] to-[#2c6bed]",
  vanilla: "from-[#34c26b] to-[#16a34a]",
  spigot: "from-[#f59e0b] to-[#d97706]",
  fabric: "from-[#d6b98c] to-[#a8875a]",
  forge: "from-[#64748b] to-[#334155]",
  neoforge: "from-[#fb923c] to-[#ea580c]",
  quilt: "from-[#a78bfa] to-[#7c3aed]",
};

export function TypeTile({ type, className = "h-10 w-10 text-sm" }: { type: string; className?: string }) {
  return (
    <span
      className={cx(
        "flex shrink-0 items-center justify-center rounded-lg bg-gradient-to-br font-semibold text-white shadow-sm",
        className,
        typeTiles[type] ?? typeTiles.paper,
      )}
      aria-hidden="true"
    >
      {(typeLabels[type] ?? type).slice(0, 1)}
    </span>
  );
}

