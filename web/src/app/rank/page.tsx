"use client";

import { useState } from "react";
import { Avatar, ErrorState, ListSkeleton, Pager } from "@/components/ui";
import { RankRow } from "@/components/rank-row";
import { useSession } from "@/components/session";
import { api } from "@/lib/api";
import { formatNumber } from "@/lib/format";
import { useLoad } from "@/lib/load";
import type { WealthRankRow } from "@/lib/types";

const SIZE = 50;

function Podium({ rows }: { rows: WealthRankRow[] }) {
  const order: [WealthRankRow, string, number][] = [
    [rows[1]!, "n2", 48],
    [rows[0]!, "n1", 60],
    [rows[2]!, "n3", 48],
  ];
  return (
    <div className="podium">
      {order.map(([row, cls, size]) => (
        <div key={row.pwikitId} className={cls}>
          <Avatar name={row.name} size={size} />
          <p>
            <a href={`/-/users/${row.pwikitId}`}>{row.name}</a>
          </p>
          <em className="sc-num">{formatNumber(row.coins)}</em>
          <div className="step">{row.rank}</div>
        </div>
      ))}
    </div>
  );
}

export default function RankPage() {
  const { user, standing } = useSession();
  const [page, setPage] = useState(1);
  const { data, error, loading } = useLoad(() => api.rankWealth(page, SIZE), [page]);
  const pages = data ? Math.max(1, Math.ceil(data.meta.total / SIZE)) : 1;
  const rows = data?.items ?? [];
  const podium = page === 1 && rows.length >= 3;

  return (
    <div className="page page-narrow" style={{ padding: "0 0 72px" }}>
      <div className="rk-tip">
        <span>按持有云币排名，共 {data ? formatNumber(data.meta.total) : "-"} 人</span>
        <span className="sc-num">
          {page} / {pages}
        </span>
      </div>
      {loading && !data ? (
        <ListSkeleton rows={10} />
      ) : error || !data ? (
        <ErrorState message={error} />
      ) : rows.length === 0 ? (
        <div className="block block-empty">财富榜暂无数据</div>
      ) : (
        <>
          {podium && <Podium rows={rows} />}
          {(podium ? rows.slice(3) : rows).map((row) => (
            <RankRow key={row.pwikitId} row={row} me={user?.pwikitId === row.pwikitId} />
          ))}
        </>
      )}
      <Pager page={page} pages={pages} onPage={setPage} />
      {user && standing?.rank != null && (
        <div className="mine-bar">
          <RankRow row={{ rank: standing.rank, pwikitId: user.pwikitId, name: user.name, coins: standing.coins }} me />
        </div>
      )}
    </div>
  );
}
