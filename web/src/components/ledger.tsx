"use client";

import { coinReasonLabel, formatNumber } from "@/lib/format";
import type { CoinLedgerRow, SessionUser, Standing } from "@/lib/types";
import cloud from "@/assets/scp-cloud.webp";

function day(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function AccountHead({ user, standing, balance }: { user: SessionUser; standing: Standing | null; balance?: number }) {
  return (
    <div className="acct">
      <img className="acct-mark" src={cloud.src} alt="" />
      <div className="acct-name">{user.name}</div>
      <div className="acct-balance">
        {formatNumber(balance ?? user.coins)}
        <small>云币</small>
      </div>
      {standing?.rank != null && (
        <div className="acct-rank">
          <span>
            财富榜第 <b>{standing.rank}</b> 名
          </span>
          <span>
            共 <b>{formatNumber(standing.ranked)}</b> 人
          </span>
        </div>
      )}
    </div>
  );
}

export function LedgerTable({ rows }: { rows: CoinLedgerRow[] }) {
  if (!rows.length) return <div className="block-empty">暂无收支记录</div>;
  return (
    <table className="ledger-table">
      <thead>
        <tr>
          <th>时间</th>
          <th>摘要</th>
          <th className="num">收入</th>
          <th className="num">支出</th>
          <th className="num hide-sm">结余</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.id}>
            <td className="date">{day(row.createdAt)}</td>
            <td>
              {coinReasonLabel(row.reason)}
              {row.memo && <div className="memo">{row.memo}</div>}
            </td>
            <td className="num in">{row.delta > 0 ? `+${formatNumber(row.delta)}` : ""}</td>
            <td className="num out">{row.delta < 0 ? formatNumber(row.delta) : ""}</td>
            <td className="num hide-sm">{formatNumber(row.balanceAfter)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
