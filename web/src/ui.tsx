import { useEffect, useId, useRef, useState, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, Copy } from "lucide-react";
import { typeLabels } from "./format";

export function cx(...parts: (string | false | null | undefined)[]) {
  return parts.filter(Boolean).join(" ");
}

type Variant = "primary" | "secondary" | "danger" | "ghost";

const variants: Record<Variant, string> = {
  primary: "bg-primary text-primary-fg hover:bg-primary-hover",
  secondary: "bg-surface border border-line text-fg hover:bg-hover hover:border-line-strong",
  danger: "bg-danger text-white hover:opacity-90",
  ghost: "text-muted hover:bg-hover hover:text-fg",
};

// buttonClass styles a link to look like a Button.
export function buttonClass(variant: Variant = "secondary", size: "sm" | "md" = "md") {
  return cx(
    "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md font-medium whitespace-nowrap transition-[color,background-color,border-color,transform] duration-150 active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-50 disabled:active:scale-100",
    size === "sm" ? "h-8 px-3 text-[13px]" : "h-9 px-4 text-sm",
    variants[variant],
  );
}

export function Button({
  variant = "secondary",
  size = "md",
  busy,
  className,
  children,
  disabled,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: "sm" | "md"; busy?: boolean }) {
  return (
    <button
      className={cx(buttonClass(variant, size), className)}
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
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeOpacity="0.2" strokeWidth="2.5" />
      <path d="M22 12a10 10 0 0 0-10-10" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

export const inputClass =
  "h-9 rounded-md border border-line bg-surface px-3 text-sm text-fg transition-colors placeholder:text-subtle hover:border-line-strong focus:border-fg/40 focus:ring-3 focus:ring-fg/8 disabled:opacity-60";

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={cx(inputClass, "w-full", props.className)} />;
}

export interface Option {
  value: string;
  label: ReactNode;
  disabled?: boolean;
  hint?: string;
}

// Select is a custom dropdown (listbox). Focus stays on the button and the
// highlighted option is announced with aria-activedescendant. The list is
// rendered in a portal so cards with overflow-hidden don't clip it.
export function Select({
  value,
  onChange,
  options,
  placeholder = "Select…",
  className,
  variant = "field",
  disabled,
  ...aria
}: {
  value: string;
  onChange: (value: string) => void;
  options: Option[];
  placeholder?: string;
  className?: string;
  variant?: "field" | "inline";
  disabled?: boolean;
  "aria-label"?: string;
  id?: string;
}) {
  const id = useId();
  const btn = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLUListElement>(null);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const [pos, setPos] = useState<{ left: number; top?: number; bottom?: number; width: number; maxHeight: number }>();
  const selected = options.find((o) => o.value === value);
  const typed = useRef({ text: "", at: 0 });

  const place = () => {
    const r = btn.current?.getBoundingClientRect();
    if (!r) return;
    const below = window.innerHeight - r.bottom - 8;
    const above = r.top - 8;
    const width = Math.max(r.width, 160);
    const left = Math.min(r.left, window.innerWidth - width - 8);
    if (below < 220 && above > below) setPos({ left, bottom: window.innerHeight - r.top + 4, width, maxHeight: Math.min(320, above) });
    else setPos({ left, top: r.bottom + 4, width, maxHeight: Math.min(320, below) });
  };

  const openList = () => {
    if (disabled) return;
    place();
    setActive(Math.max(0, options.findIndex((o) => o.value === value)));
    setOpen(true);
  };
  const choose = (i: number) => {
    const o = options[i];
    if (!o || o.disabled) return;
    onChange(o.value);
    setOpen(false);
    btn.current?.focus();
  };
  const move = (from: number, dir: 1 | -1) => {
    for (let i = 1; i <= options.length; i++) {
      const n = (from + dir * i + options.length) % options.length;
      if (!options[n]?.disabled) return n;
    }
    return from;
  };

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => {
      if (!btn.current?.contains(e.target as Node) && !list.current?.contains(e.target as Node)) setOpen(false);
    };
    const reflow = (e: Event) => {
      if (list.current?.contains(e.target as Node)) return;
      place();
    };
    document.addEventListener("mousedown", close);
    window.addEventListener("resize", reflow);
    window.addEventListener("scroll", reflow, true);
    return () => {
      document.removeEventListener("mousedown", close);
      window.removeEventListener("resize", reflow);
      window.removeEventListener("scroll", reflow, true);
    };
  }, [open]);

  useEffect(() => {
    if (open) list.current?.querySelector(`[data-index="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [open, active]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (!open) {
      if (["ArrowDown", "ArrowUp", "Enter", " "].includes(e.key)) {
        e.preventDefault();
        openList();
      }
      return;
    }
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => move(a, e.key === "ArrowDown" ? 1 : -1));
    } else if (e.key === "Home" || e.key === "End") {
      e.preventDefault();
      setActive(e.key === "Home" ? move(-1, 1) : move(options.length, -1));
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      choose(active);
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
    } else if (e.key === "Tab") {
      setOpen(false);
    } else if (e.key.length === 1) {
      const now = Date.now();
      typed.current = { text: (now - typed.current.at < 600 ? typed.current.text : "") + e.key.toLowerCase(), at: now };
      const i = options.findIndex((o) => !o.disabled && String(typeof o.label === "string" ? o.label : o.value).toLowerCase().startsWith(typed.current.text));
      if (i >= 0) setActive(i);
    }
  };

  const optionId = (i: number) => `${id}-opt-${i}`;
  return (
    <>
      <button
        ref={btn}
        type="button"
        id={aria.id}
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? `${id}-list` : undefined}
        aria-activedescendant={open && active >= 0 ? optionId(active) : undefined}
        aria-label={aria["aria-label"]}
        disabled={disabled}
        onClick={() => (open ? setOpen(false) : openList())}
        onKeyDown={onKeyDown}
        className={cx(
          "inline-flex items-center justify-between gap-2 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-60",
          variant === "field"
            ? cx(inputClass, "pr-2.5", open && "border-fg/40 ring-3 ring-fg/8", /\bw-/.test(className ?? "") ? "" : "w-full")
            : "rounded px-1 py-0.5 text-fg hover:bg-hover",
          className,
        )}
      >
        <span className={cx("truncate", !selected && "text-subtle")}>{selected ? selected.label : placeholder}</span>
        <ChevronDown className={cx("h-4 w-4 shrink-0 text-subtle transition-transform duration-200", open && "rotate-180")} strokeWidth={1.75} />
      </button>
      {open &&
        pos &&
        createPortal(
          <ul
            ref={list}
            id={`${id}-list`}
            role="listbox"
            aria-label={aria["aria-label"]}
            className={cx(
              "fixed z-50 animate-scale-in overflow-y-auto rounded-lg border border-line bg-surface-2 p-1 text-sm shadow-pop",
              pos.bottom !== undefined ? "origin-bottom" : "origin-top",
            )}
            style={{ left: pos.left, top: pos.top, bottom: pos.bottom, minWidth: pos.width, maxHeight: pos.maxHeight }}
          >
            {options.map((o, i) => (
              <li
                key={o.value}
                id={optionId(i)}
                data-index={i}
                role="option"
                aria-selected={o.value === value}
                aria-disabled={o.disabled || undefined}
                onMouseEnter={() => !o.disabled && setActive(i)}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => choose(i)}
                className={cx(
                  "flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 select-none",
                  o.disabled ? "cursor-not-allowed text-subtle" : i === active ? "bg-hover text-fg" : "text-muted",
                )}
              >
                <span className="min-w-0 flex-1 truncate">{o.label}</span>
                {o.hint && <span className="text-xs text-subtle">{o.hint}</span>}
                <Check className={cx("h-4 w-4 shrink-0 text-accent", o.value === value ? "opacity-100" : "opacity-0")} strokeWidth={2} />
              </li>
            ))}
          </ul>,
          document.body,
        )}
    </>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label className="block space-y-1.5">
        <span className="text-[13px] font-medium text-muted">{label}</span>
        {children}
      </label>
      {hint && <p className="text-xs text-subtle">{hint}</p>}
    </div>
  );
}

export function Card({
  title,
  description,
  actions,
  footer,
  children,
  className,
  flush,
}: {
  title?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  className?: string;
  flush?: boolean;
}) {
  return (
    <section className={cx("overflow-hidden rounded-xl border border-line bg-surface-2 shadow-card", className)}>
      {(title || actions) && (
        <header className="flex items-start justify-between gap-3 px-5 pt-5">
          <div className="min-w-0">
            <h2 className="text-base font-semibold tracking-tight">{title}</h2>
            {description && <p className="mt-1 text-sm text-muted">{description}</p>}
          </div>
          {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
        </header>
      )}
      <div className={flush ? "mt-4" : "p-5"}>{children}</div>
      {footer && <footer className="flex items-center justify-between gap-3 border-t border-line bg-hover/40 px-5 py-3 text-sm text-muted">{footer}</footer>}
    </section>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cx("skeleton", className)} aria-hidden="true" />;
}

export function Alert({ kind = "error", children }: { kind?: "error" | "warn" | "info"; children: ReactNode }) {
  const styles = {
    error: "bg-danger-soft text-danger border-danger/25",
    warn: "bg-warn-soft text-warn border-warn/25",
    info: "bg-accent-soft text-fg border-accent/25",
  }[kind];
  return <div className={cx("animate-fade-in rounded-md border px-3 py-2.5 text-sm break-words whitespace-pre-wrap", styles)}>{children}</div>;
}

const statusLabels: Record<string, string> = {
  running: "Running",
  stopped: "Stopped",
  creating: "Building",
  error: "Error",
};

const dotStyles: Record<string, string> = {
  running: "bg-ok",
  creating: "bg-warn animate-pulse",
  stopped: "bg-subtle",
  error: "bg-danger",
};

export function StatusDot({ status, className }: { status: string; className?: string }) {
  return <span className={cx("inline-block h-2 w-2 shrink-0 rounded-full", dotStyles[status] ?? "bg-subtle", className)} aria-hidden="true" />;
}

// LiveDot is a status dot that gently pulses while a server is running.
export function LiveDot({ status }: { status: string }) {
  if (status !== "running") return <StatusDot status={status} />;
  return (
    <span className="relative inline-flex h-2 w-2 shrink-0" aria-hidden="true">
      <span className="absolute inset-0 animate-ping-soft rounded-full bg-ok" />
      <span className="relative inline-block h-2 w-2 rounded-full bg-ok" />
    </span>
  );
}

export function StatusBadge({ status, busy }: { status: string; busy?: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-[13px] font-medium whitespace-nowrap">
      {busy ? <Spinner className="h-3 w-3 text-warn" /> : <LiveDot status={status} />}
      <span className={busy || status === "running" ? "text-fg" : "text-muted"}>{busy ?? statusLabels[status] ?? status}</span>
    </span>
  );
}

export function Badge({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={cx("inline-flex h-5 items-center rounded-full border border-line bg-surface-2 px-2 text-xs font-medium text-muted", className)}>
      {children}
    </span>
  );
}

export function Empty({ title, children, icon }: { title: string; children?: ReactNode; icon?: ReactNode }) {
  return (
    <div className="dots flex animate-fade-up flex-col items-center rounded-xl border border-dashed border-line-strong px-6 py-16 text-center">
      {icon && <div className="mb-4">{icon}</div>}
      <p className="text-base font-semibold tracking-tight">{title}</p>
      {children && <div className="mt-1.5 max-w-sm text-sm text-muted">{children}</div>}
    </div>
  );
}

export function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      title={copied ? "Copied" : label}
      aria-label={copied ? "Copied" : `${label} ${text}`}
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        navigator.clipboard?.writeText(text).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        });
      }}
      className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded text-subtle transition-colors hover:bg-hover hover:text-fg"
    >
      {copied ? <Check className="h-3.5 w-3.5 animate-scale-in text-ok" strokeWidth={2} /> : <Copy className="h-3.5 w-3.5" strokeWidth={1.75} />}
    </button>
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
      className="m-auto w-[min(30rem,calc(100vw-2rem))] rounded-xl border border-line bg-surface-2 p-0 text-fg shadow-pop backdrop:bg-black/40 backdrop:backdrop-blur-[2px]"
    >
      <div className="px-6 pt-6 text-lg font-semibold tracking-tight">{title}</div>
      <div className="px-6 pt-3 pb-6">{children}</div>
    </dialog>
  );
}

export function Meter({ value, label, detail, alarm = true }: { value: number; label: string; detail?: string; alarm?: boolean }) {
  const pct = Math.max(0, Math.min(100, value));
  return (
    <div>
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <span className="text-[13px] text-muted">{label}</span>
        <span className="font-mono text-[13px] tabular-nums">{detail ?? `${pct.toFixed(0)}%`}</span>
      </div>
      <div
        className="h-1.5 overflow-hidden rounded-full bg-surface-2 ring-1 ring-line ring-inset"
        role="meter"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={label}
      >
        <div
          className={cx("h-full rounded-full transition-all", alarm && pct > 90 ? "bg-danger" : alarm && pct > 75 ? "bg-warn" : "bg-fg")}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

export function Sparkline({ values, max }: { values: number[]; max: number }) {
  if (values.length < 2) return <div className="h-16" />;
  const w = 300;
  const h = 64;
  const top = Math.max(max, ...values, 1);
  const pts = values.map((v, i) => [(i / (values.length - 1)) * w, h - (v / top) * (h - 4) - 2] as const);
  const line = pts.map(([x, y]) => `${x},${y}`).join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-16 w-full text-accent" preserveAspectRatio="none" aria-hidden="true">
      <defs>
        <linearGradient id="spark-fill" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stopColor="currentColor" stopOpacity="0.18" />
          <stop offset="1" stopColor="currentColor" stopOpacity="0" />
        </linearGradient>
      </defs>
      <polygon points={`0,${h} ${line} ${w},${h}`} fill="url(#spark-fill)" />
      <polyline points={line} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
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

export function TypeTile({ type, className = "h-8 w-8 text-[13px]" }: { type: string; className?: string }) {
  return (
    <span
      className={cx(
        "flex shrink-0 items-center justify-center rounded-md bg-gradient-to-br font-semibold text-white ring-1 ring-black/5 ring-inset",
        typeTiles[type] ?? typeTiles.paper,
        className,
      )}
      aria-hidden="true"
    >
      {(typeLabels[type] ?? type).slice(0, 1)}
    </span>
  );
}
