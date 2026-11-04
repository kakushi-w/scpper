"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Container, EmptyState, ErrorState, ListSkeleton } from "@/components/ui";
import { loginHref, useSession } from "@/components/session";
import { api } from "@/lib/api";
import { formatNumber } from "@/lib/format";
import { useLoad } from "@/lib/load";

const PRESETS = [5, 10, 50, 100];

function split(authors: string[], amount: number): string {
  if (authors.length === 1) return `${authors[0]} 将获得 ${amount} 云币`;
  const share = Math.floor(amount / authors.length);
  const rest = amount - share * authors.length;
  return authors.map((a, i) => `${a} 将获得 ${share + (i === 0 ? rest : 0)} 云币`).join("，");
}

function Tip() {
  const pageId = Number(useSearchParams().get("page") || 0);
  const { user, refresh } = useSession();
  const { data, error, loading, reload } = useLoad(() => api.pageTips(pageId), [pageId]);
  const [amount, setAmount] = useState(10);
  const [sending, setSending] = useState(false);
  const [problem, setProblem] = useState("");
  const [done, setDone] = useState("");

  if (!pageId) return <EmptyState title="未指定页面" />;
  if (loading) return <ListSkeleton rows={4} />;
  if (error || !data) return <ErrorState message={error} />;

  async function send(e: React.FormEvent) {
    e.preventDefault();
    setSending(true);
    setProblem("");
    try {
      const res = await api.tip(pageId, amount);
      setDone(`已打赏 ${formatNumber(amount)} 云币，余额 ${formatNumber(res.balance)}`);
      await Promise.all([refresh(), reload()]);
    } catch (err) {
      setProblem(err instanceof Error ? err.message : "打赏失败");
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="block p-4">
      <h2 className="text-[17px] font-medium">打赏 {data.title}</h2>
      <div className="mt-1 text-[12.5px]" style={{ color: "var(--sc-text-3)" }}>
        作者 {data.authors.join("、")}，已收到 {data.count} 次打赏，共 {formatNumber(data.total)} 云币
      </div>
      {!user ? (
        <a href={loginHref()} className="sc-btn sc-btn-primary mt-4 h-9 px-5">
          请登录后打赏
        </a>
      ) : data.authors.includes(user.name) ? (
        <p className="mt-4 text-[12.5px]" style={{ color: "var(--sc-text-3)" }}>
          您无法打赏自己的页面。
        </p>
      ) : (
        <form onSubmit={send} className="mt-4 grid gap-3">
          <div className="flex flex-wrap gap-2">
            {PRESETS.map((n) => (
              <button
                key={n}
                type="button"
                className={`sc-btn h-8 px-3 text-[12.5px] ${amount === n ? "sc-btn-primary" : ""}`}
                onClick={() => setAmount(n)}
              >
                {n}
              </button>
            ))}
            <input
              type="number"
              min={1}
              className="sc-input h-8 w-28"
              value={amount}
              onChange={(e) => setAmount(Math.max(1, Number(e.target.value) || 1))}
              aria-label="打赏金额"
            />
          </div>
          <p className="text-[13px]" style={{ color: "var(--sc-text-2)" }}>
            {split(data.authors, amount)}，您的余额将从 {formatNumber(user.coins)} 变为 {formatNumber(Math.max(0, user.coins - amount))}。
          </p>
          {problem && <ErrorState message={problem} />}
          {done && (
            <p className="text-[12.5px]" style={{ color: "var(--sc-positive)" }}>
              {done}
            </p>
          )}
          <div>
            <button type="submit" className="sc-btn sc-btn-primary h-9 px-5" disabled={sending || amount > user.coins}>
              打赏 {formatNumber(amount)} 云币
            </button>
          </div>
        </form>
      )}
    </div>
  );
}

export default function TipPage() {
  return (
    <Container className="page-narrow">
      <Suspense>
        <Tip />
      </Suspense>
    </Container>
  );
}
