"use client";

import { useCallback, useEffect, useState } from "react";
import { EmptyState, ErrorState, ListSkeleton, Panel, Section, StatTile } from "@/components/ui";
import { api } from "@/lib/api";
import { formatDate, formatDateTime, formatNumber } from "@/lib/format";
import type { AdminStockState, AdminTask, GachaParams, StockParams } from "@/lib/types";

type Tab = "overview" | "users" | "tasks" | "gacha" | "stock";

const TABS: { id: Tab; label: string }[] = [
  { id: "overview", label: "概览" },
  { id: "users", label: "用户与云币" },
  { id: "tasks", label: "任务" },
  { id: "gacha", label: "抽卡" },
  { id: "stock", label: "股市调控" },
];

export function AdminView({ overview }: { overview: Record<string, number> }) {
  const [tab, setTab] = useState<Tab>("overview");

  return (
    <>
      <div className="mb-4 flex flex-wrap gap-1 border-b" style={{ borderColor: "var(--sc-border)" }}>
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => setTab(t.id)}
            className="relative px-3 py-2 text-[13px] font-semibold transition-colors"
            style={{ color: tab === t.id ? "var(--sc-primary)" : "var(--sc-text-3)" }}
          >
            {t.label}
            {tab === t.id && (
              <span className="absolute bottom-[-1px] left-0 h-[2px] w-full" style={{ background: "var(--sc-primary)" }} />
            )}
          </button>
        ))}
      </div>

      {tab === "overview" && <OverviewTab overview={overview} />}
      {tab === "users" && <UsersTab />}
      {tab === "tasks" && <TasksTab />}
      {tab === "gacha" && <GachaTab />}
      {tab === "stock" && <StockTab />}
    </>
  );
}

function OverviewTab({ overview }: { overview: Record<string, number> }) {
  return (
    <Section title="概况">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
        <StatTile label="用户" value={overview.users ?? 0} tone="primary" />
        <StatTile label="今日活跃" value={overview.dailyActive ?? 0} tone="positive" />
        <StatTile label="云币总量" value={overview.totalCoins ?? 0} tone="coin" />
        <StatTile label="打赏总额" value={overview.tips ?? 0} tone="coin" />
        <StatTile label="页面" value={overview.pages ?? 0} />
      </div>
    </Section>
  );
}

interface AdminUser {
  id: number;
  pwikitId: number;
  name: string;
  role: string;
  coins: number;
  createdAt: string;
  bannedAt: string | null;
}

function UsersTab() {
  const [q, setQ] = useState("");
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");

  const load = useCallback(async (query: string) => {
    setLoading(true);
    setError("");
    try {
      const res = await api.admin.users(query, 1, 50);
      setUsers(res.items as unknown as AdminUser[]);
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load("");
  }, [load]);

  async function act(fn: () => Promise<string>) {
    setError("");
    try {
      setMsg(await fn());
      await load(q);
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败");
    }
  }

  function adjust(user: AdminUser) {
    const raw = window.prompt(`请输入为 ${user.name} 增减的云币数量，正数为增加，负数为扣除。当前余额 ${user.coins}`, "100");
    if (!raw) return;
    const delta = Number.parseInt(raw, 10);
    if (!Number.isFinite(delta) || delta === 0) return;
    const memo = window.prompt("请输入备注，将记入对方的收支记录", "") ?? "";
    void act(async () => {
      const res = await api.admin.grantCoins(user.id, delta, delta > 0 ? "admin_grant" : "correction", memo);
      return `${user.name} 的余额已变为 ${formatNumber(res.balance)}`;
    });
  }

  function clear(user: AdminUser) {
    if (!window.confirm(`确定清空 ${user.name} 的 ${formatNumber(user.coins)} 云币吗？`)) return;
    const memo = window.prompt("请输入备注，将记入对方的收支记录", "") ?? "";
    void act(async () => {
      await api.admin.clearCoins(user.id, memo);
      return `${user.name} 的云币已清空`;
    });
  }

  function toggleAdmin(user: AdminUser) {
    const promote = user.role !== "admin";
    if (!window.confirm(promote ? `确定将 ${user.name} 设为管理员吗？` : `确定撤销 ${user.name} 的管理员身份吗？`)) return;
    void act(async () => {
      await api.admin.setRole(user.id, promote ? "admin" : "user");
      return promote ? `${user.name} 已设为管理员` : `已撤销 ${user.name} 的管理员身份`;
    });
  }

  return (
    <>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void load(q);
        }}
        className="mb-3 flex gap-2"
      >
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="用户名" className="sc-input h-9 flex-1" />
        <button type="submit" className="sc-btn h-9 px-4 text-[13px]">
          搜索
        </button>
      </form>

      {error && <ErrorState message={error} />}
      {msg && (
        <p className="mb-3 text-[12.5px]" style={{ color: "var(--sc-positive)" }}>
          {msg}
        </p>
      )}

      <Panel>
        {loading ? (
          <div className="p-4">
            <div className="sc-skeleton h-32 w-full" />
          </div>
        ) : users.length ? (
          users.map((u) => (
            <div
              key={u.id}
              className="flex flex-wrap items-center gap-3 border-b px-3.5 py-3 last:border-b-0"
              style={{ borderColor: "var(--sc-border)" }}
            >
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-[13.5px] font-semibold">{u.name}</span>
                  {u.role === "admin" && <span className="sc-tag">管理员</span>}
                  {u.bannedAt && <span className="sc-tag">已封禁</span>}
                </div>
                <div className="sc-num mt-0.5 text-[11.5px]" style={{ color: "var(--sc-text-3)" }}>
                  首次使用 {formatDateTime(u.createdAt)}
                </div>
              </div>
              <span className="sc-num text-[14px] font-bold" style={{ color: "var(--sc-coin)" }}>
                {formatNumber(u.coins)}
              </span>
              <div className="flex gap-1.5">
                <button type="button" onClick={() => adjust(u)} className="sc-btn h-8 px-2.5 text-[12px]">
                  增减云币
                </button>
                <button type="button" onClick={() => clear(u)} className="sc-btn h-8 px-2.5 text-[12px]" disabled={u.coins === 0}>
                  清空
                </button>
                <button type="button" onClick={() => toggleAdmin(u)} className="sc-btn h-8 px-2.5 text-[12px]">
                  {u.role === "admin" ? "撤销管理员" : "设为管理员"}
                </button>
              </div>
            </div>
          ))
        ) : (
          <EmptyState title="未找到匹配的用户" />
        )}
      </Panel>
    </>
  );
}

interface MetricInfo {
  id: AdminTask["metric"];
  label: string;
  unit: string;
  tag?: boolean;
  category?: boolean;
  score?: boolean;
}

const METRICS: MetricInfo[] = [
  { id: "vote", label: "评分", unit: "次", tag: true, category: true },
  { id: "vote_up", label: "评分赞成", unit: "次", tag: true, category: true },
  { id: "vote_down", label: "评分反对", unit: "次", tag: true, category: true },
  { id: "comment", label: "发表评论", unit: "条" },
  { id: "thread_post", label: "在论坛发帖", unit: "条" },
  { id: "post", label: "发帖或评论", unit: "条" },
  { id: "edit", label: "编辑页面", unit: "次", tag: true, category: true },
  { id: "page", label: "发布作品", unit: "篇", tag: true, category: true },
  { id: "page_score", label: "本期发布、评分达到", unit: "篇", tag: true, category: true, score: true },
  { id: "comments_received", label: "作品收到评论", unit: "条", tag: true, category: true },
  { id: "score_received", label: "作品收到的评分净值", unit: "分", tag: true, category: true },
  { id: "tip_given", label: "打赏他人", unit: "次" },
  { id: "tip_received", label: "收到打赏", unit: "云币" },
  { id: "gacha", label: "抽卡", unit: "次" },
  { id: "trade", label: "股票成交", unit: "笔" },
];

function describe(t: AdminTask): string {
  const m = METRICS.find((x) => x.id === t.metric);
  if (!m) return t.metric;
  const where = [t.category && `限 ${t.category === "_default" ? "默认" : t.category} 分类`, t.tag && `限「${t.tag}」标签`]
    .filter(Boolean)
    .join("，");
  const parts = [] as string[];
  if (m.score) {
    const line = t.score == null ? "" : t.score < 0 ? `${t.score} 及以下` : `${t.score} 及以上`;
    parts.push(`${m.label} ${line} 的作品 ${t.target} ${m.unit}`);
  } else {
    parts.push(`${m.label} ${t.target} ${m.unit}`);
  }
  if (where) parts.push(where);
  return parts.join("，");
}

const BLANK: AdminTask = {
  code: "",
  kind: "daily",
  metric: "vote",
  tag: "",
  category: "",
  score: null,
  title: "",
  description: "",
  reward: 5,
  target: 1,
  sort: 0,
  enabled: true,
};

function TasksTab() {
  const [items, setItems] = useState<AdminTask[]>([]);
  const [draft, setDraft] = useState<AdminTask | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setItems((await api.admin.tasks()).items);
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!draft) return;
    setError("");
    try {
      await api.admin.saveTask(draft);
      setDraft(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    }
  }

  const field = <K extends keyof AdminTask>(key: K, value: AdminTask[K]) =>
    setDraft((d) => (d ? { ...d, [key]: value } : d));

  return (
    <>
      {error && <ErrorState message={error} />}
      <div className="mb-3 flex justify-end">
        <button
          type="button"
          className="sc-btn sc-btn-primary h-9 px-4 text-[13px]"
          onClick={() => {
            setDraft({ ...BLANK });
            setIsNew(true);
          }}
        >
          新建任务
        </button>
      </div>

      {draft && (
        <Panel className="mb-4 p-4">
          <form onSubmit={save} className="grid gap-3 sm:grid-cols-2">
            <label className="grid gap-1 text-[12.5px]">
              任务代码
              <input
                className="sc-input h-9"
                value={draft.code}
                disabled={!isNew}
                onChange={(e) => field("code", e.target.value)}
                placeholder="daily_vote"
                required
              />
            </label>
            <label className="grid gap-1 text-[12.5px]">
              标题
              <input className="sc-input h-9" value={draft.title} onChange={(e) => field("title", e.target.value)} required />
            </label>
            <label className="grid gap-1 text-[12.5px]">
              周期
              <select
                className="sc-input h-9"
                value={draft.kind}
                onChange={(e) => field("kind", e.target.value as AdminTask["kind"])}
              >
                <option value="daily">每日</option>
                <option value="weekly">每周</option>
                <option value="monthly">每月</option>
              </select>
            </label>
            <label className="grid gap-1 text-[12.5px]">
              统计
              <select
                className="sc-input h-9"
                value={draft.metric}
                onChange={(e) => field("metric", e.target.value as AdminTask["metric"])}
              >
                {METRICS.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.label}
                  </option>
                ))}
              </select>
            </label>
            {METRICS.find((m) => m.id === draft.metric)?.tag && (
              <label className="grid gap-1 text-[12.5px]">
                仅统计带有该标签的作品
                <input className="sc-input h-9" value={draft.tag} onChange={(e) => field("tag", e.target.value)} placeholder="不限" />
              </label>
            )}
            {METRICS.find((m) => m.id === draft.metric)?.category && (
              <label className="grid gap-1 text-[12.5px]">
                仅统计该分类的作品
                <input
                  className="sc-input h-9"
                  value={draft.category}
                  onChange={(e) => field("category", e.target.value)}
                  placeholder="不限。默认分类请填写 _default"
                />
              </label>
            )}
            {METRICS.find((m) => m.id === draft.metric)?.score && (
              <label className="grid gap-1 text-[12.5px]">
                评分线，负数表示该分数及以下
                <input
                  type="number"
                  className="sc-input h-9"
                  value={draft.score ?? ""}
                  onChange={(e) => field("score", e.target.value === "" ? null : Number(e.target.value))}
                  required
                />
              </label>
            )}
            <label className="grid gap-1 text-[12.5px]">
              目标数量
              <input
                type="number"
                min={1}
                className="sc-input h-9"
                value={draft.target}
                onChange={(e) => field("target", Number(e.target.value))}
              />
            </label>
            <label className="grid gap-1 text-[12.5px]">
              奖励云币
              <input
                type="number"
                min={0}
                className="sc-input h-9"
                value={draft.reward}
                onChange={(e) => field("reward", Number(e.target.value))}
              />
            </label>
            <label className="grid gap-1 text-[12.5px] sm:col-span-2">
              说明
              <input
                className="sc-input h-9"
                value={draft.description ?? ""}
                onChange={(e) => field("description", e.target.value)}
              />
            </label>
            <label className="flex items-center gap-2 text-[12.5px]">
              <input type="checkbox" checked={draft.enabled} onChange={(e) => field("enabled", e.target.checked)} />
              启用
            </label>
            <div className="flex justify-end gap-2 sm:col-span-2">
              <button type="button" className="sc-btn h-9 px-4 text-[13px]" onClick={() => setDraft(null)}>
                取消
              </button>
              <button type="submit" className="sc-btn sc-btn-primary h-9 px-4 text-[13px]">
                保存
              </button>
            </div>
          </form>
        </Panel>
      )}

      <Panel>
        {items.length ? (
          items.map((t) => (
            <div
              key={t.code}
              className="flex flex-wrap items-center gap-3 border-b px-3.5 py-3 last:border-b-0"
              style={{ borderColor: "var(--sc-border)" }}
            >
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-[13.5px] font-semibold">{t.title}</span>
                  <span className="sc-tag">{t.kind === "daily" ? "每日" : t.kind === "weekly" ? "每周" : "每月"}</span>
                  {!t.enabled && <span className="sc-tag">已停用</span>}
                </div>
                <div className="mt-0.5 text-[11.5px]" style={{ color: "var(--sc-text-3)" }}>
                  {describe(t)}
                </div>
              </div>
              <span className="sc-num text-[14px] font-bold" style={{ color: "var(--sc-coin)" }}>
                {formatNumber(t.reward)}
              </span>
              <button
                type="button"
                className="sc-btn h-8 px-2.5 text-[12px]"
                onClick={() => {
                  setDraft({ ...t });
                  setIsNew(false);
                }}
              >
                编辑
              </button>
            </div>
          ))
        ) : (
          <EmptyState title="暂无任务" />
        )}
      </Panel>
    </>
  );
}

const RARITIES: { id: string; label: string }[] = [
  { id: "ur", label: "UR" },
  { id: "ssr", label: "SSR" },
  { id: "sr", label: "SR" },
  { id: "r", label: "R" },
  { id: "n", label: "N" },
];

function GachaTab() {
  const [p, setP] = useState<GachaParams | null>(null);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");

  useEffect(() => {
    api.admin
      .gacha()
      .then(setP)
      .catch((e) => setError(e instanceof Error ? e.message : "加载失败"));
  }, []);

  if (!p) return error ? <ErrorState message={error} /> : <ListSkeleton rows={4} />;
  const total = RARITIES.reduce((sum, r) => sum + (p.rates[r.id] ?? 0), 0);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!p) return;
    setError("");
    setMsg("");
    try {
      setP(await api.admin.saveGacha(p));
      setMsg("已保存，将从下一次抽卡起生效");
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    }
  }

  const num = (key: "costSingle" | "costTen" | "pityCount" | "tradeFeePct", label: string) => (
    <label className="grid gap-1 text-[12.5px]">
      {label}
      <input
        type="number"
        min={0}
        className="sc-input h-9"
        value={p[key]}
        onChange={(e) => setP({ ...p, [key]: Number(e.target.value) })}
      />
    </label>
  );

  return (
    <Panel className="p-4">
      <form onSubmit={save} className="grid gap-4">
        <div className="grid gap-3 sm:grid-cols-4">
          {num("costSingle", "单抽价格")}
          {num("costTen", "十连抽价格")}
          {num("pityCount", "保底抽数")}
          {num("tradeFeePct", "交易手续费（%）")}
        </div>
        <div>
          <div className="mb-1.5 text-[12.5px] font-semibold">出货权重</div>
          <table className="ledger-table">
            <thead>
              <tr>
                <th>品质</th>
                <th className="num">权重</th>
                <th className="num">概率</th>
              </tr>
            </thead>
            <tbody>
              {RARITIES.map((r) => (
                <tr key={r.id}>
                  <td>{r.label}</td>
                  <td className="num">
                    <input
                      type="number"
                      min={0}
                      className="sc-input h-8 w-28 text-right"
                      value={p.rates[r.id] ?? 0}
                      onChange={(e) => setP({ ...p, rates: { ...p.rates, [r.id]: Number(e.target.value) } })}
                    />
                  </td>
                  <td className="num">{total > 0 ? (((p.rates[r.id] ?? 0) / total) * 100).toFixed(2) : "0.00"}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <label className="grid max-w-xs gap-1 text-[12.5px]">
          十连保底品质
          <select className="sc-input h-9" value={p.tenGuarantee} onChange={(e) => setP({ ...p, tenGuarantee: e.target.value })}>
            {RARITIES.map((r) => (
              <option key={r.id} value={r.id}>
                {r.label}
              </option>
            ))}
          </select>
        </label>
        {error && <ErrorState message={error} />}
        {msg && (
          <p className="text-[12.5px]" style={{ color: "var(--sc-positive)" }}>
            {msg}
          </p>
        )}
        <div>
          <button type="submit" className="sc-btn sc-btn-primary h-9 px-5">
            保存
          </button>
        </div>
      </form>
    </Panel>
  );
}

type NumParam = Exclude<keyof StockParams, "allowShort" | "auctionTimes">;

const pctColor = (v: number) => (v > 0 ? "var(--sc-negative)" : v < 0 ? "var(--sc-positive)" : "var(--sc-text-3)");
const pctText = (v: number) => (Math.abs(v) < 0.005 ? "—" : `${v > 0 ? "+" : ""}${v.toFixed(2)}%`);

function StockTab() {
  const [symbol, setSymbol] = useState("CLOUD");
  const [st, setSt] = useState<AdminStockState | null>(null);
  const [params, setParams] = useState<StockParams | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  const [raw, setRaw] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    try {
      const r = await api.admin.stock(symbol);
      setSt(r);
      setParams(r.params);
      setRaw({});
    } catch (e) {
      setErr(e instanceof Error ? e.message : "加载失败");
    }
  }, [symbol]);

  useEffect(() => {
    void load();
  }, [load]);

  async function run(fn: () => Promise<unknown>, ok: string) {
    setBusy(true);
    setErr("");
    setMsg("");
    try {
      await fn();
      if (ok) setMsg(ok);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "操作失败");
    } finally {
      setBusy(false);
    }
  }

  if (!st || !params) {
    return err ? <ErrorState message={err} /> : <ListSkeleton rows={5} />;
  }

  const bind = (key: string, cur: string | number) => ({
    value: raw[key] ?? String(cur),
    onChange: (e: React.ChangeEvent<HTMLInputElement>) =>
      setRaw({ ...raw, [key]: e.target.value }),
  });
  const P = (k: NumParam) => bind(k, params[k]);

  const parseNum = (key: string, cur: number) => {
    const v = raw[key];
    if (v === undefined || v.trim() === "") return cur;
    const n = Number.parseFloat(v);
    return Number.isFinite(n) ? n : cur;
  };
  const paramsToSave = (): StockParams => {
    const out = { ...params };
    for (const k of Object.keys(out) as (keyof StockParams)[]) {
      if (k === "allowShort" || k === "auctionTimes") continue;
      out[k] = parseNum(k, out[k]);
    }
    const times = raw.auctionTimes;
    if (times !== undefined) {
      out.auctionTimes = times
        .split(/[,，\s]+/)
        .map((t) => t.trim())
        .filter(Boolean);
    }
    return out;
  };

  const field = (k: NumParam, label: string, hint?: string) => (
    <div key={k}>
      <label className="sc-label mb-1 block" htmlFor={`sp-${k}`}>
        {label}
      </label>
      <input id={`sp-${k}`} className="sc-input h-9" inputMode="decimal" {...P(k)} />
      {hint && (
        <p className="mt-1 text-[11px]" style={{ color: "var(--sc-text-3)" }}>
          {hint}
        </p>
      )}
    </div>
  );

  return (
    <>
      <Panel className="mb-3 p-3">
        <div className="mb-2 text-[12px] font-semibold">标的</div>
        <div className="flex flex-wrap gap-1.5">
          {st.symbols.map((y) => (
            <button
              key={y.code}
              type="button"
              onClick={() => setSymbol(y.code)}
              className="sc-btn px-2.5 py-1 text-[11.5px]"
              style={
                y.code === symbol
                  ? {
                      background: "var(--sc-primary)",
                      borderColor: "var(--sc-primary)",
                      color: "var(--sc-on-primary)",
                    }
                  : y.enabled
                    ? undefined
                    : { opacity: 0.55 }
              }
            >
              {y.shortName}
              {!y.enabled && "（未上市）"}
              {y.upstreamStale && " ⚠"}
            </button>
          ))}
        </div>

        <div className="mt-3 overflow-x-auto">
          <table className="sc-table w-full text-[11.5px]">
            <thead>
              <tr>
                <th>代号</th>
                <th>来源</th>
                <th className="text-right">现价</th>
                <th className="text-right">合理价</th>
                <th className="text-right">镜像页数</th>
                <th className="text-right">scaleBase</th>
                <th>上游最后投票</th>
                <th>状态</th>
                <th className="text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {st.symbols.map((y) => (
                <tr key={y.code}>
                  <td className="sc-num">{y.code}</td>
                  <td>{y.source === "crom" ? "Crom" : "本地"}</td>
                  <td className="sc-num text-right">{y.price ? y.price.toFixed(2) : "—"}</td>
                  <td className="sc-num text-right">{y.fair ? y.fair.toFixed(2) : "—"}</td>
                  <td className="sc-num text-right">
                    {y.source === "crom" ? formatNumber(y.mirrorPages) : "—"}
                  </td>
                  <td className="sc-num text-right">{y.scaleBase || "—"}</td>
                  <td
                    className="sc-num"
                    style={{ color: y.upstreamStale ? "var(--sc-warning)" : undefined }}
                  >
                    {y.lastVoteAt ? formatDate(y.lastVoteAt) : "—"}
                  </td>
                  <td>
                    {y.enabled ? (
                      <span style={{ color: "var(--sc-positive)" }}>已上市</span>
                    ) : (
                      <span style={{ color: "var(--sc-text-3)" }}>未上市</span>
                    )}
                  </td>
                  <td className="text-right">
                    <div className="flex justify-end gap-1">
                      {y.source === "crom" && y.siteUrl && (
                        <button
                          type="button"
                          disabled={busy}
                          className="sc-btn h-6 px-2 text-[11px]"
                          onClick={() =>
                            run(
                              () => api.admin.stockCromBackfill(y.siteUrl as string),
                              `正在抓取 ${y.shortName} 的镜像，请稍后刷新`,
                            )
                          }
                        >
                          抓取镜像
                        </button>
                      )}
                      {y.source === "crom" && y.siteUrl && y.mirrorPages > 0 && (
                        <button
                          type="button"
                          disabled={busy}
                          className="sc-btn h-6 px-2 text-[11px]"
                          onClick={() =>
                            run(
                              () => api.admin.stockCromFull(y.siteUrl as string),
                              `正在全量同步 ${y.shortName}，请稍后刷新`,
                            )
                          }
                        >
                          全量同步
                        </button>
                      )}
                      {!y.enabled ? (
                        <button
                          type="button"
                          disabled={busy || (y.source === "crom" && y.mirrorPages === 0)}
                          className="sc-btn h-6 px-2 text-[11px]"
                          onClick={() =>
                            run(() => api.admin.stockList(y.code), `正在为 ${y.shortName} 办理上市，请稍后刷新`)
                          }
                        >
                          上市
                        </button>
                      ) : (
                        <>
                          <button
                            type="button"
                            disabled={busy}
                            className="sc-btn h-6 px-2 text-[11px]"
                            onClick={() =>
                              run(async () => {
                                const r = await api.admin.stockAuction(y.code);
                                setMsg(
                                  r.auction
                                    ? `${y.shortName} 撮合完成，由 ${r.auction.priceBefore.toFixed(2)} 变为 ${r.auction.priceAfter.toFixed(2)}，成交 ${r.auction.orders} 单`
                                    : `${y.shortName} 本场已撮合`,
                                );
                              }, "")
                            }
                          >
                            立即撮合
                          </button>
                          {y.code !== "CLOUD" && (
                            <button
                              type="button"
                              disabled={busy}
                              className="sc-btn h-6 px-2 text-[11px]"
                              onClick={() =>
                                run(
                                  () => api.admin.stockDelist(y.code),
                                  `正在为 ${y.shortName} 办理停牌，待撮合的订单将全部退款`,
                                )
                              }
                            >
                              停牌
                            </button>
                          )}
                        </>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>

      {st.shares.drift !== 0 && (
        <Panel className="mb-3 p-3" style={{ borderColor: "var(--sc-negative)" }}>
          <div className="text-[12px] font-semibold" style={{ color: "var(--sc-negative)" }}>
            股数不一致：持仓 {formatNumber(st.shares.positions)} + 做市商{" "}
            {formatNumber(st.shares.maker)} + 机器人 {formatNumber(st.shares.bots)} ={" "}
            {formatNumber(st.shares.total)}，应为 {formatNumber(st.shares.float)}（差{" "}
            {st.shares.drift > 0 ? "+" : ""}
            {formatNumber(st.shares.drift)}）
          </div>
        </Panel>
      )}

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <StatTile label="现价" value={st.book.pRef.toFixed(2)} tone="primary" />
        <StatTile label="合理价" value={st.book.pFair.toFixed(2)} />
        <StatTile label="市场备用金" value={formatNumber(st.book.reserve)} tone="coin" />
        <StatTile label="做市商库存" value={`${formatNumber(st.book.inventory)} 股`} />
      </div>

      <Section title="市场资金">
        <Panel className="p-4">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {[
              ["备用金", formatNumber(st.book.reserve)],
              ["做市商现金", formatNumber(st.book.cash)],
              ["库存市值", formatNumber(st.book.inventoryValue)],
              ["合计净值", formatNumber(st.book.netWorth)],
            ].map(([k, v]) => (
              <div key={k}>
                <div className="sc-label">{k}</div>
                <div className="sc-num text-[15px] font-bold">{v}</div>
              </div>
            ))}
          </div>
          <p
            className="mt-3 border-t pt-3 text-[12.5px]"
            style={{ borderColor: "var(--sc-border)", color: "var(--sc-text-2)" }}
          >
            现价比合理价{" "}
            <span className="sc-num font-bold">
              {pctText((st.book.pRef / st.book.pFair - 1) * 100 || 0)}
            </span>
          </p>
        </Panel>
      </Section>

      <Section title="用户整体盈亏">
        <Panel className="p-4">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {[
              ["持仓人数", formatNumber(st.players.holders)],
              ["用户持股", `${formatNumber(st.players.sharesHeld)} 股`],
              ["已实现盈亏", formatNumber(st.players.realizedPnl)],
              ["浮动盈亏", formatNumber(st.players.floatingPnl)],
              ["已付手续费", formatNumber(st.players.feesPaid)],
              ["今日成交", `${formatNumber(st.players.turnoverDay)} 股`],
              ["待撮合单", formatNumber(st.players.openOrders)],
              ["净盈亏", formatNumber(st.players.netTransfer)],
            ].map(([k, v]) => (
              <div key={k}>
                <div className="sc-label">{k}</div>
                <div className="sc-num text-[15px] font-bold">{v}</div>
              </div>
            ))}
          </div>
        </Panel>
      </Section>

      <Section title="竞价规则">
        <Panel className="p-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <label className="sc-label mb-1 block" htmlFor="sp-auctionTimes">
                竞价时间（北京）
              </label>
              <input
                id="sp-auctionTimes"
                className="sc-input h-9"
                {...bind("auctionTimes", params.auctionTimes.join(", "))}
              />
            </div>
            {field("impactPct", "满仓推动 %", "净买入量达到单账号持仓上限时的涨幅")}
            {field("eventPct", "站点动态幅度 %", "新页面数量为平时两倍时的涨幅，也是没有新页面时的最大跌幅")}
            {field("eventCapPct", "站点动态上限 %")}
            {field("dailyBandPct", "单场涨跌停 %")}
            {field("halfLifeDays", "回归合理价半衰期（天）")}
            {field("sigmaDaily", "日波动率 σ")}
            {field("feePct", "手续费 %")}
          </div>

          <div className="mt-5 border-t pt-4" style={{ borderColor: "var(--sc-border)" }}>
            <div className="mb-3 text-[13px] font-semibold">仓位与杠杆</div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {field("maxPositionPct", "单账号持仓上限 %")}
              {field("floatShares", "总股本")}
              {field("maxLeverage", "最高杠杆")}
              {field("maintenanceMarginPct", "维持保证金率 %")}
              {field("borrowFeeDailyPct", "融资费（每天 %）")}
              {field("minPrice", "最低价")}
            </div>
          </div>

          <div className="mt-5 border-t pt-4" style={{ borderColor: "var(--sc-border)" }}>
            <div className="mb-3 text-[13px] font-semibold">合理价</div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {field("basePrice", "全局基准价", "分站指数的基准价在上市时按页面数确定")}
              {field("scaleWeight", "规模权重")}
              {field("scaleBase", "云国指数规模基准（页面数）")}
            </div>
          </div>

          <button
            type="button"
            disabled={busy}
            onClick={() => run(() => api.admin.stockParams(paramsToSave()), "规则已更新")}
            className="sc-btn sc-btn-primary mt-4 h-9 px-5 text-[13px]"
          >
            保存规则
          </button>
        </Panel>
      </Section>

      <Section title="竞价记录">
        {st.auctions.length ? (
          <Panel>
            <div className="overflow-x-auto">
              <table className="sc-table w-full text-[11.5px]">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th className="text-right">价格</th>
                    <th className="text-right">涨跌</th>
                    <th className="text-right">单数</th>
                    <th className="text-right">买 / 卖</th>
                    <th className="text-right">推动</th>
                    <th className="text-right">新页 / 平时</th>
                    <th className="text-right">动态</th>
                    <th className="text-right">回归</th>
                    <th className="text-right">随机</th>
                  </tr>
                </thead>
                <tbody>
                  {st.auctions.map((a) => (
                    <tr key={a.at}>
                      <td className="sc-num">{formatDateTime(a.at)}</td>
                      <td className="sc-num text-right">
                        {a.priceBefore.toFixed(2)} 变为 {a.priceAfter.toFixed(2)}
                      </td>
                      <td className="sc-num text-right" style={{ color: pctColor(a.changePct) }}>
                        {pctText(a.changePct)}
                      </td>
                      <td className="sc-num text-right">{a.orders}</td>
                      <td className="sc-num text-right">
                        {formatNumber(a.buyQty)} / {formatNumber(a.sellQty)}
                      </td>
                      <td className="sc-num text-right" style={{ color: pctColor(a.impactPct) }}>
                        {pctText(a.impactPct)}
                      </td>
                      <td className="sc-num text-right">
                        {a.newPages} / {a.expected.toFixed(1)}
                      </td>
                      <td className="sc-num text-right" style={{ color: pctColor(a.eventPct) }}>
                        {pctText(a.eventPct)}
                      </td>
                      <td className="sc-num text-right" style={{ color: pctColor(a.driftPct) }}>
                        {pctText(a.driftPct)}
                      </td>
                      <td className="sc-num text-right" style={{ color: pctColor(a.shockPct) }}>
                        {pctText(a.shockPct)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Panel>
        ) : (
          <EmptyState title="暂无竞价记录" />
        )}
      </Section>

      <Section title="盈亏排行（前 20）">
        {st.top.length ? (
          <Panel>
            {st.top.map((p) => (
              <div
                key={p.userId}
                className="flex flex-wrap items-center gap-3 border-b px-3 py-2 last:border-b-0"
                style={{ borderColor: "var(--sc-border)" }}
              >
                <span className="flex-1 text-[13px] font-semibold">{p.name}</span>
                <span className="sc-num text-[12px]" style={{ color: "var(--sc-text-3)" }}>
                  {formatNumber(p.qty)} 股 @ {p.avgCost.toFixed(2)}
                </span>
                <span className="sc-num text-[12px]" style={{ color: "var(--sc-text-2)" }}>
                  已实现 {formatNumber(p.realizedPnl)}
                </span>
                <span
                  className="sc-num w-24 text-right text-[13px] font-bold"
                  style={{ color: pctColor(p.totalPnl) }}
                >
                  {p.totalPnl >= 0 ? "+" : ""}
                  {formatNumber(p.totalPnl)}
                </span>
              </div>
            ))}
          </Panel>
        ) : (
          <EmptyState title="暂无持仓" />
        )}
      </Section>

      {err && <ErrorState message={err} />}
      {msg && (
        <div
          className="mt-3 border-l-4 px-3 py-2 text-[12.5px]"
          style={{
            borderColor: "var(--sc-primary)",
            background: "var(--sc-surface-2)",
            color: "var(--sc-text-2)",
          }}
        >
          {msg}
        </div>
      )}
    </>
  );
}
