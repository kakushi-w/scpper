"use client";

import { useState } from "react";
import { Container, ErrorState, ListSkeleton, Pager } from "@/components/ui";
import { AccountHead, LedgerTable } from "@/components/ledger";
import { loginHref, useSession } from "@/components/session";
import { api } from "@/lib/api";
import { useLoad } from "@/lib/load";

const SIZE = 30;

function Ledger() {
  const { user, standing } = useSession();
  const [page, setPage] = useState(1);
  const { data, error, loading } = useLoad(() => api.coins(page, SIZE), [page]);
  if (!user) return null;
  if (loading && !data) return <ListSkeleton rows={8} />;
  if (error || !data) return <ErrorState message={error} />;

  return (
    <>
      <div className="block">
        <AccountHead user={user} standing={standing} balance={data.balance} />
        <LedgerTable rows={data.items} />
      </div>
      <Pager page={page} pages={Math.max(1, Math.ceil(data.meta.total / SIZE))} onPage={setPage} />
    </>
  );
}

export default function CoinsPage() {
  const { user, loading } = useSession();
  return (
    <Container>
      {loading ? (
        <ListSkeleton rows={8} />
      ) : user ? (
        <Ledger />
      ) : (
        <div className="block p-5">
          <p className="text-[14px]">请登录后查看您的收支。</p>
          <a href={loginHref()} className="sc-btn sc-btn-primary mt-3 h-9 px-5">
            登录
          </a>
        </div>
      )}
    </Container>
  );
}
