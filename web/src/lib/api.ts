import type {
  AdminStockState,
  AdminTask,
  Candle,
  CandleSpan,
  CoinLedgerRow,
  GachaCard,
  GachaInfo,
  GachaListing,
  GachaParams,
  ListResponse,
  MyListing,
  OwnedCard,
  PageTips,
  PullRecord,
  SessionUser,
  Standing,
  StockAuction,
  StockParams,
  StockPortfolio,
  StockQuote,
  StockSymbol,
  StockTrade,
  TasksResponse,
  WealthRankRow,
} from "./types";

export const BASE = process.env.NEXT_PUBLIC_BASE_PATH ?? "/scpper";

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(BASE + path, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    credentials: "same-origin",
    signal: opts.signal,
    cache: "no-store",
  });

  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      throw new ApiError(res.status, "bad_response", "无法解析服务器的响应");
    }
  }
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(res.status, err?.code ?? "unknown", err?.message ?? `请求失败（${res.status}）`);
  }
  return data as T;
}

function qs(params: Record<string, string | number | boolean | undefined | null>): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === "") continue;
    sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : "";
}

function seg(v: string): string {
  return encodeURIComponent(v);
}

export const api = {
  me: () => request<{ user: SessionUser | null; standing: Standing | null }>("/api/me"),
  rankWealth: (p = 1, limit = 50) =>
    request<ListResponse<WealthRankRow>>(`/api/rank/wealth${qs({ page: p, limit })}`),
  pageTips: (pageId: number) => request<PageTips>(`/api/pages/${pageId}/tips`),
  tip: (pageId: number, amount: number) =>
    request<{ balance: number }>("/api/tips", { method: "POST", body: { pageId, amount } }),

  tasks: () => request<TasksResponse>("/api/me/tasks"),
  claimTask: (code: string) =>
    request<{ claimed: boolean; reward: number; balance: number }>(`/api/me/tasks/${seg(code)}/claim`, {
      method: "POST",
    }),
  refreshTasks: () => request<TasksResponse>("/api/me/tasks/refresh", { method: "POST" }),

  coins: (p = 1, limit = 30) =>
    request<ListResponse<CoinLedgerRow> & { balance: number }>(`/api/me/coins${qs({ page: p, limit })}`),

  gachaInfo: () => request<GachaInfo>("/api/gacha"),
  gachaPull: (batch: "single" | "ten") =>
    request<{ cards: GachaCard[]; cost: number; balance: number }>("/api/gacha/pull", {
      method: "POST",
      body: { batch },
    }),
  gachaCollection: (rarity: string, p = 1, limit = 60) =>
    request<ListResponse<OwnedCard>>(`/api/gacha/collection${qs({ rarity, page: p, limit })}`),
  gachaHistory: (p = 1, limit = 50) =>
    request<ListResponse<PullRecord>>(`/api/gacha/history${qs({ page: p, limit })}`),
  gachaMarket: (params: Record<string, string | number | undefined>) =>
    request<ListResponse<GachaListing> & { tradeFeePct: number }>(`/api/gacha/market${qs(params)}`),
  gachaMyListings: () => request<{ items: MyListing[] }>("/api/gacha/my-listings"),
  gachaList: (articleId: number, price: number) =>
    request<{ id: number; ok: boolean }>("/api/gacha/market", {
      method: "POST",
      body: { articleId, price },
    }),
  gachaBuy: (id: number) =>
    request<{ ok: boolean; price: number; balance: number }>(`/api/gacha/market/${id}/buy`, {
      method: "POST",
    }),
  gachaUnlist: (id: number) => request<{ ok: boolean }>(`/api/gacha/market/${id}`, { method: "DELETE" }),

  stockQuote: (symbol = "CLOUD") => request<StockQuote>(`/api/stock${qs({ symbol })}`),
  stockSymbols: () => request<{ items: StockSymbol[] }>("/api/stock/symbols"),
  stockCandles: (span: CandleSpan = "day", symbol = "CLOUD") =>
    request<{ span: string; items: Candle[]; listedAt: string | null }>(
      `/api/stock/candles${qs({ span, symbol })}`,
    ),
  stockAuctions: (symbol = "CLOUD", limit = 12) =>
    request<{ items: StockAuction[] }>(`/api/stock/auctions${qs({ symbol, limit })}`),
  stockPortfolio: () => request<StockPortfolio>("/api/stock/portfolio"),
  stockPlace: (body: { symbol?: string; side: "buy" | "sell"; qty: number; leverage?: number }) =>
    request<{
      orderId: number;
      hold: number;
      balance: number;
      nextAuctionAt: string;
      symbol: string;
    }>(`/api/stock/orders${qs({ symbol: body.symbol ?? "CLOUD" })}`, { method: "POST", body }),
  stockCancel: (id: number) =>
    request<{ ok: boolean; balance: number }>(`/api/stock/orders/${id}`, { method: "DELETE" }),
  stockTrades: (p = 1, limit = 50, symbol = "") =>
    request<ListResponse<StockTrade>>(`/api/stock/trades${qs({ page: p, limit, symbol })}`),

  admin: {
    overview: () => request<Record<string, number>>("/api/admin/overview"),
    users: (q: string, p = 1, limit = 30) =>
      request<ListResponse<Record<string, unknown>>>(`/api/admin/users${qs({ q, page: p, limit })}`),
    grantCoins: (userId: number, delta: number, reason: string, memo: string) =>
      request<{ balance: number }>(`/api/admin/users/${userId}/coins`, {
        method: "POST",
        body: { delta, reason, memo },
      }),
    clearCoins: (userId: number, memo: string) =>
      request<{ balance: number }>(`/api/admin/users/${userId}/coins`, {
        method: "POST",
        body: { clear: true, memo },
      }),
    setRole: (userId: number, role: "admin" | "user", banned = false) =>
      request<{ ok: boolean }>(`/api/admin/users/${userId}/role`, {
        method: "POST",
        body: { role, banned },
      }),
    tasks: () => request<{ items: AdminTask[] }>("/api/admin/tasks"),
    saveTask: (task: AdminTask) => request<AdminTask>("/api/admin/tasks", { method: "POST", body: task }),
    gacha: () => request<GachaParams>("/api/admin/gacha"),
    saveGacha: (body: GachaParams) => request<GachaParams>("/api/admin/gacha", { method: "POST", body }),
    stock: (symbol = "CLOUD") => request<AdminStockState>(`/api/admin/stock${qs({ symbol })}`),
    stockSymbolParams: (code: string, body: Record<string, number>) =>
      request<unknown>(`/api/admin/stock/symbols/${code}/params`, { method: "POST", body }),
    stockList: (code: string) =>
      request<{ queued: string; symbol: string }>(`/api/admin/stock/symbols/${code}/list`, {
        method: "POST",
      }),
    stockDelist: (code: string) =>
      request<{ queued: string; symbol: string }>(`/api/admin/stock/symbols/${code}/delist`, {
        method: "POST",
      }),
    stockCromBackfill: (site: string) =>
      request<{ queued: string; site: string }>(`/api/admin/stock/crom/backfill${qs({ site })}`, {
        method: "POST",
      }),
    stockCromFull: (site: string) =>
      request<{ queued: string; site: string }>(`/api/admin/stock/crom/full${qs({ site })}`, {
        method: "POST",
      }),
    stockParams: (body: StockParams) =>
      request<StockParams>("/api/admin/stock/params", { method: "POST", body }),
    stockAuction: (symbol: string) =>
      request<{ symbol: string; auction: StockAuction | null }>(`/api/admin/stock/auction${qs({ symbol })}`, {
        method: "POST",
      }),
  },
};
