"use client";

import Link from "next/link";
import { Container, ListSkeleton } from "@/components/ui";
import { AccountHead, LedgerTable } from "@/components/ledger";
import { RankRow } from "@/components/rank-row";
import { loginHref, useSession } from "@/components/session";
import { api } from "@/lib/api";
import { useLoad } from "@/lib/load";

function Recent() {
  const { data } = useLoad(() => api.coins(1, 8));
  if (!data) return <div className="block-empty">正在加载</div>;
  return <LedgerTable rows={data.items} />;
}

function Top() {
  const { user } = useSession();
  const { data } = useLoad(() => api.rankWealth(1, 10));
  const rows = data?.items ?? [];
  if (!rows.length) return <div className="block-empty">{data ? "财富榜暂无数据" : "正在加载"}</div>;
  return (
    <>
      {rows.map((row) => (
        <RankRow key={row.pwikitId} row={row} me={user?.pwikitId === row.pwikitId} />
      ))}
    </>
  );
}

export default function Home() {
  const { user, standing, loading } = useSession();

  return (
    <Container>
      <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div>
          {loading ? (
            <ListSkeleton rows={4} />
          ) : user ? (
            <div className="block">
              <AccountHead user={user} standing={standing} />
              <div className="block-h">
                最近收支
                <Link href="/me/coins">全部收支</Link>
              </div>
              <Recent />
            </div>
          ) : (
            <div className="block p-5">
              <p className="text-[14px]">请登录后查看您的云币账户。</p>
              <a href={loginHref()} className="sc-btn sc-btn-primary mt-3 h-9 px-5">
                登录
              </a>
            </div>
          )}
        </div>
        <aside className="block self-start">
          <div className="block-h">
            财富榜
            <Link href="/rank">全部</Link>
          </div>
          <Top />
        </aside>
      </div>
    </Container>
  );
}
