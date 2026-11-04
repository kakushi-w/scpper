import { Avatar } from "./ui";
import { formatNumber } from "@/lib/format";
import type { WealthRankRow } from "@/lib/types";

export function RankRow({ row, me }: { row: WealthRankRow; me?: boolean }) {
  return (
    <div className={`lr${me ? " me" : ""}`}>
      <span className="rk sc-num">{row.rank}</span>
      <Avatar name={row.name} size={36} />
      <span className="nm">
        <a href={`/-/users/${row.pwikitId}`}>{row.name}</a>
      </span>
      <span className="sc sc-num">{formatNumber(row.coins)}</span>
    </div>
  );
}
