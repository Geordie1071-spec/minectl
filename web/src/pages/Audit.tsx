import { useInfiniteQuery } from "@tanstack/react-query";
import { Navigate } from "react-router-dom";
import { api, type AuditEntry } from "../api";
import { useMe } from "../hooks";
import { PageTitle } from "../layout";
import { dateTime } from "../format";
import { Alert, Button, Card, Spinner } from "../ui";

const pageSize = 100;

export function Audit() {
  const { data: me } = useMe();
  const q = useInfiniteQuery({
    queryKey: ["audit"],
    queryFn: ({ pageParam }) => api.get<AuditEntry[]>(`/audit?limit=${pageSize}${pageParam ? `&before=${pageParam}` : ""}`),
    initialPageParam: 0,
    getNextPageParam: (last) => (last.length === pageSize ? last[last.length - 1]!.id : undefined),
    enabled: !!me?.user.is_admin,
  });
  if (me && !me.user.is_admin) return <Navigate to="/" replace />;
  const rows = q.data?.pages.flat() ?? [];

  return (
    <>
      <PageTitle>Activity</PageTitle>
      <Card>
        {q.isPending && <Spinner />}
        {q.error && <Alert>{q.error.message}</Alert>}
        <div className="-m-4 overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-line text-xs text-muted">
              <tr>
                <th className="px-4 py-2 font-medium">When</th>
                <th className="px-4 py-2 font-medium">Who</th>
                <th className="px-4 py-2 font-medium">What</th>
                <th className="px-4 py-2 font-medium">Server</th>
                <th className="px-4 py-2 font-medium">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {rows.map((e) => (
                <tr key={e.id}>
                  <td className="px-4 py-2 whitespace-nowrap text-muted">{dateTime(e.time)}</td>
                  <td className="px-4 py-2" title={e.ip}>
                    {e.username || "—"}
                  </td>
                  <td className="px-4 py-2 whitespace-nowrap">
                    <code className={e.action === "login.failed" ? "text-danger" : undefined}>{e.action}</code>
                  </td>
                  <td className="px-4 py-2">{e.server}</td>
                  <td className="max-w-xs truncate px-4 py-2 text-muted" title={e.detail}>
                    {e.detail}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {q.hasNextPage && (
          <div className="mt-8 text-center">
            <Button onClick={() => q.fetchNextPage()} busy={q.isFetchingNextPage}>
              Load older
            </Button>
          </div>
        )}
      </Card>
    </>
  );
}
