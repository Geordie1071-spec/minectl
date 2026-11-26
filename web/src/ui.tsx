import { useEffect, useRef, useState, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from "react";

export function cx(...parts: (string | false | null | undefined)[]) {
  return parts.filter(Boolean).join(" ");
}

type Variant = "primary" | "secondary" | "danger" | "ghost";

const variants: Record<Variant, string> = {
  primary: "bg-accent text-accent-fg hover:bg-accent-hover",
  secondary: "bg-surface border border-line hover:bg-surface-2",
  danger: "bg-danger text-white hover:opacity-90",
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
        "inline-flex items-center justify-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50",
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
  "rounded-md border border-line bg-surface px-3 py-1.5 text-sm placeholder:text-muted focus:border-accent focus:outline-none";

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
    <section className={cx("rounded-lg border border-line bg-surface", className)}>
      {(title || actions) && (
        <header className="flex items-center justify-between gap-2 border-b border-line px-4 py-3">
          <h2 className="text-sm font-semibold">{title}</h2>
          <div className="flex items-center gap-2">{actions}</div>
        </header>
      )}
      <div className="p-4">{children}</div>
    </section>
  );
}

export function Alert({ kind = "error", children }: { kind?: "error" | "warn" | "info"; children: ReactNode }) {
  const styles = {
    error: "bg-danger-soft text-danger border-danger/30",
    warn: "bg-warn-soft text-warn border-warn/30",
    info: "bg-accent-soft text-fg border-accent/30",
  }[kind];
  return <div className={cx("rounded-md border px-3 py-2 text-sm whitespace-pre-wrap break-words", styles)}>{children}</div>;
}

const statusStyles: Record<string, string> = {
  running: "bg-accent",
  creating: "bg-warn animate-pulse",
  stopped: "bg-muted/50",
  error: "bg-danger",
};

export function StatusDot({ status }: { status: string }) {
  return <span className={cx("inline-block h-2 w-2 shrink-0 rounded-full", statusStyles[status] ?? "bg-muted/50")} aria-hidden="true" />;
}

export function StatusBadge({ status, busy }: { status: string; busy?: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs font-medium">
      {busy ? <Spinner className="h-3 w-3" /> : <StatusDot status={status} />}
      {busy ?? status}
    </span>
  );
}

export function Badge({ children }: { children: ReactNode }) {
  return <span className="rounded bg-surface-2 px-1.5 py-0.5 text-xs font-medium text-muted">{children}</span>;
}

export function Empty({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed border-line px-6 py-10 text-center">
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
      className="m-auto w-[min(32rem,calc(100vw-2rem))] rounded-lg border border-line bg-surface p-0 text-fg shadow-xl backdrop:bg-black/50"
    >
      <div className="border-b border-line px-4 py-3 font-semibold">{title}</div>
      <div className="p-4">{children}</div>
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
