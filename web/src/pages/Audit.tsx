import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Navigate } from "react-router-dom";
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from "lucide-react";
import { api, type AuditPage } from "../api";
import { useMe } from "../hooks";
import { PageTitle } from "../layout";
import { dateTime } from "../format";
import { Alert, Card, Spinner, cx } from "../ui";

const pageSize = 25;

// pageNumbers picks which page buttons to show: the first, the last, and a
// few around the current one, with gaps as null.
function pageNumbers(page: number, pages: number): (number | null)[] {
  const keep = new Set([1, pages, page - 1, page, page + 1].filter((n) => n >= 1 && n <= pages));
  const sorted = [...keep].sort((a, b) => a - b);
  const out: (number | null)[] = [];
  sorted.forEach((n, i) => {
    if (i > 0 && n - sorted[i - 1]! > 1) out.push(null);
    out.push(n);
  });
  return out;
}

function PageButton({ onClick, disabled, active, label, children }: { onClick: () => void; disabled?: boolean; active?: boolean; label: string; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      aria-current={active ? "page" : undefined}
      className={cx(
        "flex h-8 min-w-8 items-center justify-center rounded-md px-2 text-[13px] tabular-nums transition-colors disabled:cursor-not-allowed disabled:opacity-40",
        active ? "bg-primary font-medium text-primary-fg" : "text-muted hover:bg-hover hover:text-fg",
      )}
    >
      {children}
    </button>
  );
}

export function Audit() {
  const { data: me } = useMe();
  const [page, setPage] = useState(1);
  const q = useQuery({
    queryKey: ["audit", page],
    queryFn: () => api.get<AuditPage>(`/audit?page=${page}&limit=${pageSize}`),
    placeholderData: keepPreviousData,
    enabled: !!me?.user.is_admin,
  });
  if (me && !me.user.is_admin) return <Navigate to="/" replace />;
  const data = q.data;
  const pages = data?.pages ?? 1;
  const first = data && data.total > 0 ? (page - 1) * pageSize + 1 : 0;
  const last = data ? Math.min(page * pageSize, data.total) : 0;
  const go = (n: number) => setPage(Math.min(Math.max(1, n), pages));

  return (
    <>
      <PageTitle description="Every change made through the panel, newest first.">Activity</PageTitle>
      <Card flush>
        {q.isPending && (
          <div className="p-5">
            <Spinner />
          </div>
        )}
        {q.error && (
          <div className="p-5">
            <Alert>{q.error.message}</Alert>
          </div>
        )}
        {data && data.total === 0 && <p className="p-5 text-sm text-muted">Nothing has happened yet.</p>}
        {data && data.total > 0 && (
          <div className={cx("overflow-x-auto transition-opacity", q.isPlaceholderData && "opacity-60")}>
            <table className="w-full text-left text-sm">
              <thead className="border-b border-line bg-surface text-xs text-muted">
                <tr>
                  <th className="px-5 py-2.5 font-medium">When</th>
                  <th className="px-5 py-2.5 font-medium">Who</th>
                  <th className="px-5 py-2.5 font-medium">What</th>
                  <th className="px-5 py-2.5 font-medium">Server</th>
                  <th className="px-5 py-2.5 font-medium">Details</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {data.entries.map((e) => (
                  <tr key={e.id} className="transition-colors hover:bg-hover/40">
                    <td className="px-5 py-2.5 whitespace-nowrap text-muted">{dateTime(e.time)}</td>
                    <td className="px-5 py-2.5" title={e.ip}>
                      {e.username || "—"}
                    </td>
                    <td className="px-5 py-2.5 whitespace-nowrap">
                      <code className={e.action === "login.failed" ? "text-danger" : undefined}>{e.action}</code>
                    </td>
                    <td className="px-5 py-2.5">{e.server}</td>
                    <td className="max-w-xs truncate px-5 py-2.5 text-muted" title={e.detail}>
                      {e.detail}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {data && data.total > 0 && (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-5 py-3">
            <p className="text-[13px] text-muted tabular-nums">
              {first}–{last} of {data.total}
            </p>
            <nav className="flex items-center gap-0.5" aria-label="Pages">
              <PageButton label="First page" onClick={() => go(1)} disabled={page === 1}>
                <ChevronsLeft className="h-4 w-4" strokeWidth={1.75} />
              </PageButton>
              <PageButton label="Previous page" onClick={() => go(page - 1)} disabled={page === 1}>
                <ChevronLeft className="h-4 w-4" strokeWidth={1.75} />
              </PageButton>
              {pageNumbers(page, pages).map((n, i) =>
                n === null ? (
                  <span key={"gap" + i} className="px-1 text-subtle">
                    …
                  </span>
                ) : (
                  <PageButton key={n} label={`Page ${n}`} onClick={() => go(n)} active={n === page}>
                    {n}
                  </PageButton>
                ),
              )}
              <PageButton label="Next page" onClick={() => go(page + 1)} disabled={page === pages}>
                <ChevronRight className="h-4 w-4" strokeWidth={1.75} />
              </PageButton>
              <PageButton label="Last page" onClick={() => go(pages)} disabled={page === pages}>
                <ChevronsRight className="h-4 w-4" strokeWidth={1.75} />
              </PageButton>
            </nav>
          </div>
        )}
      </Card>
    </>
  );
}
