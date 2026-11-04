"use client";

import { useCallback, useEffect, useState } from "react";
import { CandleChart } from "@/components/candle-chart";
import { loginHref, useSession } from "@/components/session";
import { api } from "@/lib/api";
import { formatDateTime, formatNumber } from "@/lib/format";
import type { AuctionEvent, Candle, CandleSpan, StockAuction, StockQuote, StockSymbol, StockTrade } from "@/lib/types";

const SPANS: [CandleSpan, string][] = [
  ["tick", "逐场"],
  ["day", "日K"],
  ["week", "周K"],
  ["month", "月K"],
  ["year", "年K"],
];

const cls = (v: number) => (v > 0 ? "sc-up" : v < 0 ? "sc-down" : "sc-flat");
const pill = (v: number) => (v > 0 ? "pill-up" : v < 0 ? "pill-down" : "pill-flat");
const signed = (v: number, digits = 2) => `${v > 0 ? "+" : ""}${v.toFixed(digits)}`;
const delta = (price: number, pct: number) => price - price / (1 + pct / 100);

const hhmm = (iso: string) =>
  new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", hour: "2-digit", minute: "2-digit", hour12: false }).format(new Date(iso));

export function StockView({
  initialSymbols,
  initialSymbol,
}: {
  initialSymbols: StockSymbol[];
  initialSymbol: string;
}) {
  const [symbols, setSymbols] = useState(initialSymbols);
  const [symbol, setSymbol] = useState(initialSymbol);

  useEffect(() => {
    const t = setInterval(() => {
      api
        .stockSymbols()
        .then((r) => setSymbols(r.items))
        .catch(() => {});
    }, 60_000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    const back = () => setSymbol((new URLSearchParams(window.location.search).get("symbol") || "").toUpperCase());
    window.addEventListener("popstate", back);
    return () => window.removeEventListener("popstate", back);
  }, []);

  const go = useCallback((code: string) => {
    setSymbol(code);
    window.history.pushState(null, "", code ? `?symbol=${code}` : window.location.pathname);
    window.scrollTo(0, 0);
  }, []);

  if (!symbol) return <Watchlist symbols={symbols} onOpen={go} />;
  return <Detail key={symbol} symbol={symbol} onBack={() => go("")} />;
}

function Watchlist({ symbols, onOpen }: { symbols: StockSymbol[]; onOpen: (code: string) => void }) {
  return (
    <div className="st">
      <div className="idxbar">
        {symbols.slice(0, 3).map((s) => (
          <button key={s.code} type="button" onClick={() => onOpen(s.code)}>
            <p>{s.shortName}</p>
            <b className={`sc-num ${cls(s.changePct)}`}>{s.price.toFixed(2)}</b>
            <span className={`sc-num ${cls(s.changePct)}`}>
              {signed(delta(s.price, s.changePct))}　{signed(s.changePct)}%
            </span>
          </button>
        ))}
      </div>
      <div className="wl-h">
        <span>名称代码</span>
        <span>最新</span>
        <span>涨幅</span>
      </div>
      {symbols.map((s) => (
        <button key={s.code} type="button" className="wl" onClick={() => onOpen(s.code)}>
          <span className="nm">
            <b>{s.name}</b>
            <span>
              {s.myQty !== 0 && <i className="hold">持</i>}
              {s.source === "crom" && <i>外</i>}
              {s.code}
            </span>
          </span>
          <span className={`px sc-num ${cls(s.changePct)}`}>{s.price.toFixed(2)}</span>
          <span className="pc">
            <span className={`sc-num ${pill(s.changePct)}`}>{signed(s.changePct)}%</span>
          </span>
        </button>
      ))}
      {symbols.length === 0 && <div className="wl-f">暂无上市的指数</div>}
    </div>
  );
}

function Detail({ symbol, onBack }: { symbol: string; onBack: () => void }) {
  const { user, refresh } = useSession();
  const [quote, setQuote] = useState<StockQuote | null>(null);
  const [span, setSpan] = useState<CandleSpan>("day");
  const [candles, setCandles] = useState<Candle[] | null>(null);
  const [auctions, setAuctions] = useState<StockAuction[]>([]);
  const [trades, setTrades] = useState<StockTrade[]>([]);
  const [trade, setTrade] = useState<"buy" | "sell" | null>(null);
  const [err, setErr] = useState("");

  const reload = useCallback(async () => {
    try {
      const [q, a] = await Promise.all([api.stockQuote(symbol), api.stockAuctions(symbol, 12)]);
      setQuote(q);
      setAuctions(a.items);
      setErr("");
      if (user) setTrades((await api.stockTrades(1, 30, symbol)).items);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "加载失败");
    }
  }, [symbol, user]);

  useEffect(() => {
    void reload();
    const t = setInterval(() => void reload(), 60_000);
    return () => clearInterval(t);
  }, [reload]);

  useEffect(() => {
    setCandles(null);
    api
      .stockCandles(span, symbol)
      .then((r) => setCandles(r.items))
      .catch(() => setCandles([]));
  }, [span, symbol]);

  const traded = () => {
    void reload();
    void refresh();
  };

  const q = quote;
  const d = q ? q.price - q.prevClose : 0;

  return (
    <div className="st">
      <div className="st-nav">
        <button type="button" className="side" onClick={onBack}>
          全部指数
        </button>
        <div className="tt">
          <b>{q ? `${q.name}(${q.code})` : symbol}</b>
          {q && <Countdown quote={q} onDue={() => void reload()} />}
        </div>
        <span className="side" />
      </div>

      {err && !q && <div className="msg err" style={{ margin: 12 }}>{err}</div>}

      {q && (
        <>
          <div className="qt">
            <div className="l">
              <div className={`big sc-num ${cls(d)}`}>{q.price.toFixed(2)}</div>
              <div className={`chg sc-num ${cls(d)}`}>
                {signed(d)}　{signed(q.changePct)}%
              </div>
            </div>
            <div className="g">
              <div>
                合理价<b className="sc-num">{q.fair.toFixed(2)}</b>
              </div>
              <div>
                昨收<b className="sc-num">{q.prevClose.toFixed(2)}</b>
              </div>
              <div>
                本场上限<b className="sc-num sc-up">{q.capPrice.toFixed(2)}</b>
              </div>
              <div>
                本场下限<b className="sc-num sc-down">{q.floorPrice.toFixed(2)}</b>
              </div>
              <div>
                成交量<b className="sc-num">{formatNumber(q.dayVolume)}</b>
              </div>
              <div>
                流通股<b className="sc-num">{formatNumber(q.floatShares)}</b>
              </div>
            </div>
          </div>
          <div className="qt2">
            <div>
              单场幅度<b className="sc-num">±{q.bandPct}%</b>
            </div>
            <div>
              手续费<b className="sc-num">{q.feePct}%</b>
            </div>
            <div>
              持仓上限<b className="sc-num">{formatNumber(q.maxPosition)}</b>
            </div>
            <div>
              杠杆上限<b className="sc-num">{q.maxLeverage}x</b>
            </div>
          </div>

          <div className="ktab" role="tablist">
            {SPANS.map(([s, label]) => (
              <button key={s} type="button" role="tab" aria-selected={span === s} className={span === s ? "on" : ""} onClick={() => setSpan(s)}>
                {label}
              </button>
            ))}
          </div>
          <div className="chartwrap">
            <div className="chart">
              {candles ? <CandleChart candles={candles} /> : <div className="sc-skeleton m-3 h-[280px]" />}
            </div>
            <Queue quote={q} />
          </div>

          {q.me && q.me.qty !== 0 && <Position quote={q} />}

          <div className="mx">
            <h5>
              历次竞价<span>每日 {q.auctionTimes.join("、")}</span>
            </h5>
            {auctions.length ? (
              <>
                <div className="t head">
                  <span>时间</span>
                  <span>成交价</span>
                  <span>涨跌</span>
                  <span>买 / 卖</span>
                </div>
                {auctions.map((a) => (
                  <div key={a.at} className="t">
                    <span className="sc-num" style={{ color: "#666" }}>
                      {formatDateTime(a.at)}
                    </span>
                    <span className={`sc-num ${cls(a.changePct)}`}>{a.priceAfter.toFixed(2)}</span>
                    <span className={`sc-num ${cls(a.changePct)}`}>{signed(a.changePct)}%</span>
                    <span className="sc-num">
                      <span className="sc-up">{formatNumber(a.buyQty)}</span> / <span className="sc-down">{formatNumber(a.sellQty)}</span>
                    </span>
                  </div>
                ))}
              </>
            ) : (
              <div className="empty">暂无竞价记录</div>
            )}
          </div>

          {user && (
            <div className="mx">
              <h5>我的成交</h5>
              {trades.length ? (
                <>
                  <div className="t head">
                    <span>时间</span>
                    <span>价格</span>
                    <span>数量</span>
                    <span>金额</span>
                  </div>
                  {trades.map((t, i) => (
                    <div key={`${t.at}-${i}`} className="t">
                      <span className="sc-num" style={{ color: "#666" }}>
                        {formatDateTime(t.at)}
                      </span>
                      <span className="sc-num">{t.price.toFixed(2)}</span>
                      <span className={`sc-num ${t.side === "buy" ? "sc-up" : "sc-down"}`}>
                        {formatNumber(t.qty)} {t.side === "buy" ? "买" : "卖"}
                      </span>
                      <span className="sc-num">{formatNumber(t.gross)}</span>
                    </div>
                  ))}
                </>
              ) : (
                <div className="empty">暂无成交记录</div>
              )}
            </div>
          )}

          <div className="tb">
            {user ? (
              <>
                <div className="hold">
                  持仓
                  <b className="sc-num">{formatNumber(q.me?.qty ?? 0)}</b>
                </div>
                <button type="button" className="buy" onClick={() => setTrade("buy")}>
                  买入
                </button>
                <button type="button" className="sell" onClick={() => setTrade("sell")}>
                  卖出
                </button>
              </>
            ) : (
              <a className="go" href={loginHref()}>
                登录后交易
              </a>
            )}
          </div>

          {trade && <TradeSheet quote={q} side={trade} onSide={setTrade} onClose={() => setTrade(null)} onTraded={traded} />}
        </>
      )}
    </div>
  );
}

function Countdown({ quote, onDue }: { quote: StockQuote; onDue: () => void }) {
  const [now, setNow] = useState<number | null>(null);

  useEffect(() => {
    setNow(Date.now());
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  const target = new Date(quote.nextAuctionAt).getTime();

  useEffect(() => {
    const wait = target - Date.now() + 25_000;
    if (wait <= 0 || wait > 24 * 3600_000) return;
    const t = setTimeout(onDue, wait);
    return () => clearTimeout(t);
  }, [target, onDue]);

  const pad = (n: number) => String(n).padStart(2, "0");
  let left = "--:--:--";
  if (now !== null) {
    const s = Math.floor(Math.max(0, target - now) / 1000);
    left = `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
  }
  return (
    <span className="sc-num">
      {hhmm(quote.nextAuctionAt)} 竞价，剩余 {left}
    </span>
  );
}

function Queue({ quote }: { quote: StockQuote }) {
  const { buyQty, sellQty, orders, estImpactPct } = quote.pending;
  const total = buyQty + sellQty;
  const a = quote.lastAuction;
  return (
    <div className="ob">
      <h6>{hhmm(quote.nextAuctionAt)} 场排队</h6>
      <div className="r">
        <span>买入</span>
        <span className="sc-num sc-up">{formatNumber(buyQty)}</span>
      </div>
      <div className="r">
        <span>卖出</span>
        <span className="sc-num sc-down">{formatNumber(sellQty)}</span>
      </div>
      <div className="r">
        <span>订单</span>
        <span className="sc-num">{orders}</span>
      </div>
      <div className="r">
        <span>预计推动</span>
        <span className={`sc-num ${cls(estImpactPct)}`}>{signed(estImpactPct)}%</span>
      </div>
      <div className="wb">
        买
        <i>
          <b style={{ width: `${total ? (buyQty / total) * 100 : 50}%` }} />
        </i>
        卖
      </div>
      {a && (
        <>
          <hr />
          <h6>上一场 {hhmm(a.at)}</h6>
          {(
            [
              ["买卖推动", a.impactPct],
              ["站点动态", a.eventPct],
              ["回归合理价", a.driftPct],
              ["随机", a.shockPct],
            ] as [string, number][]
          ).map(([k, v]) => (
            <div key={k} className="r">
              <span>{k}</span>
              <span className={`sc-num ${cls(v)}`}>{Math.abs(v) < 0.005 ? "0.00%" : `${signed(v)}%`}</span>
            </div>
          ))}
          <div className="r">
            <span>新页面</span>
            <span className="sc-num">
              {a.newPages}，平均 {a.expected.toFixed(1)}
            </span>
          </div>
          {a.events.slice(0, 6).map((e) => {
            const name = `${quote.shortName}：${pageTitle(e)}`;
            return (
              <a key={e.url} className="ev" href={e.url} target="_blank" rel="noreferrer" title={name}>
                {name}
              </a>
            );
          })}
        </>
      )}
    </div>
  );
}

function pageTitle(e: AuctionEvent): string {
  const title = e.title || decodeURIComponent(e.url.replace(/\/+$/, "").split("/").pop() || e.url);
  return e.altTitle ? `${title} ${e.altTitle}` : title;
}

function Position({ quote }: { quote: StockQuote }) {
  const me = quote.me!;
  const tight = (me.marginRatio ?? 100) < quote.maintenancePct * 1.5;
  return (
    <div className="mx">
      <h5>
        我的持仓<span>{me.qty > 0 ? "多头" : "空头"}</span>
      </h5>
      <div className="pos">
        <div>
          持仓<b className="sc-num">{formatNumber(Math.abs(me.qty))}</b>
        </div>
        <div>
          成本<b className="sc-num">{me.avgCost.toFixed(2)}</b>
        </div>
        <div>
          浮动盈亏<b className={`sc-num ${cls(me.floatingPnl)}`}>{signed(me.floatingPnl, 0)}</b>
        </div>
        <div>
          已实现<b className={`sc-num ${cls(me.realizedPnl)}`}>{signed(me.realizedPnl, 0)}</b>
        </div>
        {me.liqPrice !== undefined && (
          <>
            <div>
              保证金<b className="sc-num">{formatNumber(me.margin)}</b>
            </div>
            <div>
              杠杆<b className="sc-num">{me.leverage?.toFixed(2)}x</b>
            </div>
            <div>
              保证金率
              <b className="sc-num" style={tight ? { color: "var(--sc-warning)" } : undefined}>
                {me.marginRatio?.toFixed(1)}%
              </b>
            </div>
            <div>
              强平价<b className="sc-num" style={{ color: "var(--sc-warning)" }}>{me.liqPrice > 0 ? me.liqPrice.toFixed(2) : "无"}</b>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function TradeSheet({
  quote,
  side,
  onSide,
  onClose,
  onTraded,
}: {
  quote: StockQuote;
  side: "buy" | "sell";
  onSide: (s: "buy" | "sell") => void;
  onClose: () => void;
  onTraded: () => void;
}) {
  const [lev, setLev] = useState(1);
  const [qty, setQty] = useState("10");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");

  const me = quote.me;
  const pending = me?.pendingOrder;
  const n = Number.parseInt(qty, 10) || 0;
  const curQty = me?.qty ?? 0;
  const dir = side === "buy" ? 1 : -1;
  const closing = curQty !== 0 && Math.sign(curQty) !== dir ? Math.min(n, Math.abs(curQty)) : 0;
  const opening = n - closing;
  const feeRate = quote.feePct / 100;
  const unitHold = (quote.capPrice * (1 + feeRate)) / lev;
  const hold = opening > 0 ? Math.ceil(opening * unitHold) : 0;
  const byMoney = unitHold > 0 ? Math.floor((me?.balanceCoins ?? 0) / unitHold) : 0;
  const closable = curQty !== 0 && Math.sign(curQty) !== dir ? Math.abs(curQty) : 0;
  const byCap = Math.max(0, quote.maxPosition - (closable ? 0 : Math.abs(curQty)));
  const most = closable + Math.min(byMoney, byCap);
  const fee = Math.round(n * quote.price * feeRate);
  const impact = quote.maxPosition > 0 ? (Math.exp(((quote.impactPct / 100) * dir * n) / quote.maxPosition) - 1) * 100 : 0;

  let liq = 0;
  if (opening > 0 && curQty === 0 && lev > 1) {
    const m = quote.maintenancePct / 100;
    const px = quote.price;
    const margin = (opening * px * (1 + feeRate)) / lev;
    liq = dir > 0 ? Math.max(0, (margin - opening * px) / (opening * (m - 1))) : (margin + opening * px) / (opening * (1 + m));
  }

  async function run(fn: () => Promise<string>) {
    setBusy(true);
    setErr("");
    setMsg("");
    try {
      setMsg(await fn());
      onTraded();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "操作失败");
    } finally {
      setBusy(false);
    }
  }

  const submit = () => {
    if (n <= 0) {
      setErr("股数须大于 0");
      return;
    }
    void run(async () => {
      const r = await api.stockPlace({ symbol: quote.code, side, qty: n, leverage: lev });
      return `已提交，将于 ${hhmm(r.nextAuctionAt)} 撮合，冻结 ${formatNumber(r.hold)} 云币`;
    });
  };

  return (
    <div className="sheet-bg" onClick={onClose}>
      <div className="sheet" onClick={(e) => e.stopPropagation()}>
        <div className="sheet-h">
          {quote.name}
          <button type="button" onClick={onClose}>
            关闭
          </button>
        </div>
        {pending ? (
          <>
            <div className="form-row">
              <span>已挂单</span>
              <b className={`sc-num ${pending.side === "buy" ? "sc-up" : "sc-down"}`}>
                {pending.side === "buy" ? "买入" : "卖出"} {formatNumber(pending.qty)} 股
                {pending.leverage > 1 ? `，${pending.leverage}x` : ""}
              </b>
            </div>
            <div className="form-row">
              <span>冻结</span>
              <span className="sc-num">
                {formatNumber(pending.hold)} 云币，将于 {hhmm(quote.nextAuctionAt)} 撮合
              </span>
            </div>
            {err && <div className="msg err" style={{ marginTop: 10 }}>{err}</div>}
            {msg && <div className="msg" style={{ marginTop: 10 }}>{msg}</div>}
            <div className="sheet-foot">
              <button
                type="button"
                style={{ background: "#888" }}
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    await api.stockCancel(pending.id);
                    return "已撤单";
                  })
                }
              >
                撤单
              </button>
            </div>
          </>
        ) : (
          <>
            <div className="side-tabs">
              <button type="button" className={`buy${side === "buy" ? " on" : ""}`} onClick={() => onSide("buy")}>
                买入
              </button>
              <button type="button" className={`sell${side === "sell" ? " on" : ""}`} onClick={() => onSide("sell")}>
                卖出
              </button>
            </div>
            <div className="form-row">
              <label htmlFor="stock-qty">股数</label>
              <input
                id="stock-qty"
                className="sc-input sc-num"
                inputMode="numeric"
                value={qty}
                onChange={(e) => setQty(e.target.value.replace(/[^0-9]/g, ""))}
              />
            </div>
            <div className="form-row">
              <span>快捷</span>
              <div className="chips">
                {[0.25, 0.5, 1].map((f) => (
                  <button key={f} type="button" onClick={() => setQty(String(Math.floor(most * f)))}>
                    {f === 1 ? "最多" : `${f * 100}%`}
                  </button>
                ))}
              </div>
            </div>
            {quote.maxLeverage > 1 && (
              <div className="form-row">
                <span>杠杆</span>
                <div className="chips">
                  {Array.from({ length: Math.floor(quote.maxLeverage) }, (_, i) => i + 1).map((v) => (
                    <button key={v} type="button" className={`lev${lev === v ? " on" : ""}`} onClick={() => setLev(v)}>
                      {v}x
                    </button>
                  ))}
                </div>
              </div>
            )}
            <div className="sum">
              <div>
                冻结<b className="sc-num">{formatNumber(hold)}</b>
              </div>
              <div>
                手续费<b className="sc-num">{formatNumber(fee)}</b>
              </div>
              <div>
                预计推动<b className={`sc-num ${cls(impact)}`}>{n > 0 ? `${signed(impact)}%` : "0.00%"}</b>
              </div>
              <div>
                可用<b className="sc-num">{formatNumber(me?.balanceCoins ?? 0)}</b>
              </div>
              <div>
                成交区间
                <b className="sc-num">
                  {quote.floorPrice.toFixed(2)} ~ {quote.capPrice.toFixed(2)}
                </b>
              </div>
              {closing > 0 && (
                <div>
                  平仓<b className="sc-num">{formatNumber(closing)}</b>
                </div>
              )}
              {side === "sell" && opening > 0 && (
                <div>
                  做空<b className="sc-num sc-down">{formatNumber(opening)}</b>
                </div>
              )}
              {liq > 0 && (
                <div>
                  强平价<b className="sc-num" style={{ color: "var(--sc-warning)" }}>{liq.toFixed(2)}</b>
                </div>
              )}
            </div>
            {err && <div className="msg err">{err}</div>}
            {msg && <div className="msg">{msg}</div>}
            <div className="sheet-foot">
              <button
                type="button"
                style={{ background: side === "buy" ? "var(--sc-up)" : "var(--sc-primary)" }}
                disabled={busy || n <= 0}
                onClick={submit}
              >
                {busy ? "正在提交" : `${side === "buy" ? "买入" : "卖出"} ${formatNumber(n)} 股`}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
