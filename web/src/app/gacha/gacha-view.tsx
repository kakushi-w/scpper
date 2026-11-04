"use client";

import { useCallback, useEffect, useState } from "react";
import { CardFace, RARITIES, RARITY_LABEL, RARITY_RANGE, RarityTag } from "@/components/gacha-card";
import { GachaResult } from "@/components/gacha-reveal";
import { loginHref, useSession } from "@/components/session";
import { api } from "@/lib/api";
import { articleHref, formatDateTime, formatNumber, formatRating } from "@/lib/format";
import type { GachaCard, GachaInfo, GachaListing, MyListing, OwnedCard, PullRecord } from "@/lib/types";
import cloud from "@/assets/scp-cloud.webp";

type Tab = "draw" | "collection" | "market" | "history";

const TABS: { id: Tab; label: string }[] = [
  { id: "draw", label: "抽卡" },
  { id: "collection", label: "我的收藏" },
  { id: "market", label: "交易市场" },
  { id: "history", label: "抽卡记录" },
];

const fail = (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback);

export function GachaView({ initial }: { initial: GachaInfo | null }) {
  const { user, refresh } = useSession();
  const [tab, setTab] = useState<Tab>("draw");
  const [info, setInfo] = useState<GachaInfo | null>(initial);

  const reloadInfo = useCallback(async () => {
    try {
      setInfo(await api.gachaInfo());
    } catch {}
  }, []);

  const changed = () => {
    void reloadInfo();
    void refresh();
  };

  return (
    <div className="ga">
      <div className="ga-in">
        <div className="ga-top">
          <div className="ga-title">抽卡</div>
          {user && (
            <div className="ga-res">
              云币 <b className="rb">{formatNumber(user.coins)}</b>
            </div>
          )}
        </div>
        <div className="ga-tabs">
          {TABS.map((t) => (
            <button key={t.id} type="button" className={tab === t.id ? "on" : ""} onClick={() => setTab(t.id)}>
              {t.label}
            </button>
          ))}
        </div>

        {tab === "draw" && <Draw info={info} loggedIn={!!user} coins={user?.coins ?? 0} onPulled={changed} onHistory={() => setTab("history")} />}
        {tab === "collection" && <Collection loggedIn={!!user} onChanged={changed} />}
        {tab === "market" && <Market loggedIn={!!user} onChanged={changed} />}
        {tab === "history" && <History loggedIn={!!user} />}
      </div>
    </div>
  );
}

function Draw({
  info,
  loggedIn,
  coins,
  onPulled,
  onHistory,
}: {
  info: GachaInfo | null;
  loggedIn: boolean;
  coins: number;
  onPulled: () => void;
  onHistory: () => void;
}) {
  const [busy, setBusy] = useState(0);
  const [last, setLast] = useState<"single" | "ten">("ten");
  const [cards, setCards] = useState<GachaCard[] | null>(null);
  const [showing, setShowing] = useState(false);
  const [err, setErr] = useState("");
  const [rates, setRates] = useState(false);

  const costSingle = info?.costSingle ?? 10;
  const costTen = info?.costTen ?? 90;
  const pool = info?.tiers.reduce((n, t) => n + t.pool, 0) ?? 0;
  const ur = info?.tiers.find((t) => t.rarity === "ur");

  async function pull(batch: "single" | "ten") {
    if (busy) return;
    if (!loggedIn) {
      window.location.href = loginHref();
      return;
    }
    setLast(batch);
    setErr("");
    setCards(null);
    setShowing(true);
    setBusy(batch === "ten" ? 10 : 1);
    try {
      const r = await api.gachaPull(batch);
      setCards(r.cards);
      onPulled();
    } catch (e) {
      setShowing(false);
      setErr(fail(e, "抽卡失败"));
    } finally {
      setBusy(0);
    }
  }

  const againCost = last === "ten" ? costTen : costSingle;

  return (
    <>
      <div className="bnr">
        <div className="stripe" />
        <img className="art" src={cloud.src} alt="" />
        <div className="ttl">
          <span className="s1">常驻卡池</span>
          <h2>全站档案</h2>
          <h3>卡池共 {formatNumber(pool)} 篇作品</h3>
        </div>
        {info && (
          <div className="tiers">
            {info.tiers.slice(0, 3).map((t) => (
              <div key={t.rarity} className={t.rarity}>
                <em>{RARITY_LABEL[t.rarity]}</em>
                {t.rate}%<span>{RARITY_RANGE[t.rarity]}</span>
              </div>
            ))}
          </div>
        )}
        {ur && (
          <div className="rate">
            UR 出现概率
            <br />
            <b className="rb">{ur.rate}</b>%
          </div>
        )}
        {info && <div className="per">十连抽必出 {RARITY_LABEL[info.tenGuarantee]} 及以上</div>}
      </div>

      <div className="glinks">
        <button type="button" onClick={() => setRates(true)}>
          概率详情
        </button>
        <button type="button" onClick={onHistory}>
          抽卡记录
        </button>
        {info && (
          <span className="pt">
            距保底<b className="rb">{info.me ? info.me.pityIn : info.pityCount}</b>抽
          </span>
        )}
      </div>

      <div className="gbtns">
        <button type="button" className="gb g1" disabled={busy > 0 || (loggedIn && coins < costSingle)} onClick={() => pull("single")}>
          <span className="lbl">单抽</span>
          <span className="cost rb">{formatNumber(costSingle)} 云币</span>
        </button>
        <button type="button" className="gb g10" disabled={busy > 0 || (loggedIn && coins < costTen)} onClick={() => pull("ten")}>
          {info && <span className="rib">必出 {RARITY_LABEL[info.tenGuarantee]} 以上</span>}
          <span className="lbl">十连抽</span>
          <span className="cost rb">{formatNumber(costTen)} 云币</span>
        </button>
      </div>
      {!loggedIn && <p className="gnote">请登录后抽卡。</p>}
      {err && <div className="msg err" style={{ margin: "10px 0 0" }}>{err}</div>}
      {info && (
        <p className="gnote">
          ※ 品质按作品评分的绝对值划分。※ 连续 {info.pityCount} 抽未出 SSR 及以上时，下一抽必定为 SSR 及以上。※ 抽到的卡牌可在交易市场出售，成交时收取 {info.tradeFeePct}% 手续费。
        </p>
      )}

      {showing && (
        <GachaResult
          cards={cards}
          busy={busy}
          canAgain={loggedIn && coins >= againCost}
          againLabel={last === "ten" ? "再次十连抽" : "再抽一次"}
          onClose={() => setShowing(false)}
          onAgain={() => pull(last)}
        />
      )}

      {rates && info && (
        <div className="modal-bg" onClick={() => setRates(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h4 className="mb-2 text-[15px] font-bold" style={{ color: "var(--sc-navy)" }}>
              概率详情
            </h4>
            <table className="gtable">
              <thead>
                <tr>
                  <th>品质</th>
                  <th>作品评分</th>
                  <th>概率</th>
                  <th>卡池</th>
                </tr>
              </thead>
              <tbody>
                {info.tiers.map((t) => (
                  <tr key={t.rarity}>
                    <td>
                      <RarityTag rarity={t.rarity} />
                    </td>
                    <td>{RARITY_RANGE[t.rarity]}</td>
                    <td className="sc-num">{t.rate}%</td>
                    <td className="sc-num">{formatNumber(t.pool)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="gnote" style={{ padding: "8px 0 0" }}>
              十连抽中若前九抽均未出 {RARITY_LABEL[info.tenGuarantee]} 及以上，最后一抽必定为 {RARITY_LABEL[info.tenGuarantee]} 及以上。保底机制只会使实际概率高于公示概率。
            </p>
            <div className="mt-3 text-right">
              <button type="button" className="gbtn plain px-5" onClick={() => setRates(false)}>
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function RarityFilter({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <>
      <button type="button" className={value === "" ? "on" : ""} onClick={() => onChange("")}>
        全部
      </button>
      {RARITIES.map((r) => (
        <button key={r} type="button" className={value === r ? "on" : ""} onClick={() => onChange(r)}>
          {RARITY_LABEL[r]}
        </button>
      ))}
    </>
  );
}

function Login({ what }: { what: string }) {
  return (
    <div className="gcard text-center text-[13px]">
      <p className="py-4">请登录后查看{what}。</p>
      <a href={loginHref()} className="gbtn inline-flex items-center px-6">
        登录
      </a>
    </div>
  );
}

function Collection({ loggedIn, onChanged }: { loggedIn: boolean; onChanged: () => void }) {
  const [rarity, setRarity] = useState("");
  const [items, setItems] = useState<OwnedCard[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [listing, setListing] = useState<OwnedCard | null>(null);

  const load = useCallback(async () => {
    if (!loggedIn) return;
    setLoading(true);
    setErr("");
    try {
      const r = await api.gachaCollection(rarity, 1, 60);
      setItems(r.items);
      setTotal(r.meta.total);
    } catch (e) {
      setErr(fail(e, "加载失败"));
    } finally {
      setLoading(false);
    }
  }, [loggedIn, rarity]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!loggedIn) return <Login what="您的收藏" />;

  return (
    <div className="gcard">
      <div className="gfilter">
        <RarityFilter value={rarity} onChange={setRarity} />
        <span className="count sc-num">{formatNumber(total)} 种</span>
      </div>
      {err && <div className="msg err" style={{ margin: 0 }}>{err}</div>}
      {loading ? (
        <p className="py-8 text-center text-[12px]" style={{ color: "#7a8aa6" }}>正在加载</p>
      ) : items.length ? (
        <div className="ggrid">
          {items.map((c) => (
            <div key={c.articleId} className="gitem">
              <CardFace card={c} qty={c.qty} />
              <button type="button" className="gbtn plain" onClick={() => setListing(c)}>
                上架出售
              </button>
            </div>
          ))}
        </div>
      ) : (
        <p className="py-8 text-center text-[12px]" style={{ color: "#7a8aa6" }}>暂无卡牌</p>
      )}
      {listing && (
        <ListDialog
          card={listing}
          onClose={() => setListing(null)}
          onDone={() => {
            setListing(null);
            void load();
            onChanged();
          }}
        />
      )}
    </div>
  );
}

function ListDialog({ card, onClose, onDone }: { card: OwnedCard; onClose: () => void; onDone: () => void }) {
  const [price, setPrice] = useState("100");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function submit() {
    const n = Number.parseInt(price, 10);
    if (!Number.isFinite(n) || n <= 0) {
      setErr("售价须为正整数");
      return;
    }
    setBusy(true);
    setErr("");
    try {
      await api.gachaList(card.articleId, n);
      onDone();
    } catch (e) {
      setErr(fail(e, "上架失败"));
      setBusy(false);
    }
  }

  return (
    <div className="modal-bg" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="flex gap-3">
          <div className="w-[96px] shrink-0">
            <CardFace card={card} qty={card.qty} />
          </div>
          <div className="min-w-0 flex-1">
            <div className="text-[14px] font-bold">{card.title || card.page}</div>
            <div className="mt-1 text-[12px]" style={{ color: "#7a8aa6" }}>
              {card.author}，{formatRating(card.rating)}，持有 {card.qty} 张
            </div>
            <label className="mt-3 block text-[12px]" htmlFor="listing-price" style={{ color: "#4a5d7e" }}>
              售价
            </label>
            <input
              id="listing-price"
              className="sc-input mt-1"
              inputMode="numeric"
              value={price}
              onChange={(e) => setPrice(e.target.value.replace(/\D/g, ""))}
            />
          </div>
        </div>
        {err && <div className="msg err" style={{ margin: "10px 0 0" }}>{err}</div>}
        <div className="mt-4 flex gap-2">
          <button type="button" className="gbtn flex-1" disabled={busy} onClick={submit}>
            {busy ? "正在上架" : "上架"}
          </button>
          <button type="button" className="gbtn plain px-5" onClick={onClose}>
            取消
          </button>
        </div>
      </div>
    </div>
  );
}

function Market({ loggedIn, onChanged }: { loggedIn: boolean; onChanged: () => void }) {
  const [rarity, setRarity] = useState("");
  const [sort, setSort] = useState("new");
  const [items, setItems] = useState<GachaListing[]>([]);
  const [mine, setMine] = useState<MyListing[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [busyId, setBusyId] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setErr("");
    try {
      const r = await api.gachaMarket({ rarity, sort, limit: 40 });
      setItems(r.items);
      if (loggedIn) setMine((await api.gachaMyListings()).items);
    } catch (e) {
      setErr(fail(e, "加载失败"));
    } finally {
      setLoading(false);
    }
  }, [rarity, sort, loggedIn]);

  useEffect(() => {
    void load();
  }, [load]);

  async function act(id: number, run: () => Promise<unknown>, fallback: string) {
    setBusyId(id);
    setErr("");
    try {
      await run();
      await load();
      onChanged();
    } catch (e) {
      setErr(fail(e, fallback));
    } finally {
      setBusyId(null);
    }
  }

  const open = mine.filter((m) => m.status === "open");

  return (
    <>
      {open.length > 0 && (
        <div className="gcard">
          <h4>我的在售卡牌</h4>
          {open.map((m) => (
            <div key={m.id} className="grow">
              <RarityTag rarity={m.rarity} />
              <a href={articleHref(m)} target="_blank" rel="noreferrer">
                {m.title || m.page}
              </a>
              <span className="sc-num font-bold" style={{ color: "var(--sc-coin)" }}>
                {formatNumber(m.price)}
              </span>
              <button
                type="button"
                className="gbtn plain px-3"
                disabled={busyId === m.id}
                onClick={() => act(m.id, () => api.gachaUnlist(m.id), "撤单失败")}
              >
                撤单
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="gcard">
        <div className="gfilter">
          <RarityFilter value={rarity} onChange={setRarity} />
          <select value={sort} onChange={(e) => setSort(e.target.value)} aria-label="排序">
            <option value="new">最新上架</option>
            <option value="price">价格从低到高</option>
            <option value="rarity">品质优先</option>
          </select>
        </div>
        {err && <div className="msg err" style={{ margin: "0 0 10px" }}>{err}</div>}
        {loading ? (
          <p className="py-8 text-center text-[12px]" style={{ color: "#7a8aa6" }}>正在加载</p>
        ) : items.length ? (
          <div className="ggrid">
            {items.map((it) => (
              <div key={it.id} className="gitem">
                <CardFace card={it} />
                <div className="meta">
                  <span>{it.seller}</span>
                  <span className="price">{formatNumber(it.price)}</span>
                </div>
                <button
                  type="button"
                  className="gbtn"
                  disabled={busyId === it.id}
                  onClick={() => (loggedIn ? act(it.id, () => api.gachaBuy(it.id), "购买失败") : (window.location.href = loginHref()))}
                >
                  {busyId === it.id ? "正在购买" : "购买"}
                </button>
              </div>
            ))}
          </div>
        ) : (
          <p className="py-8 text-center text-[12px]" style={{ color: "#7a8aa6" }}>暂无在售卡牌</p>
        )}
      </div>
    </>
  );
}

function History({ loggedIn }: { loggedIn: boolean }) {
  const [items, setItems] = useState<PullRecord[] | null>(null);

  useEffect(() => {
    if (!loggedIn) return;
    let live = true;
    api
      .gachaHistory(1, 100)
      .then((r) => live && setItems(r.items))
      .catch(() => live && setItems([]));
    return () => {
      live = false;
    };
  }, [loggedIn]);

  if (!loggedIn) return <Login what="抽卡记录" />;

  return (
    <div className="gcard">
      {!items ? (
        <p className="py-8 text-center text-[12px]" style={{ color: "#7a8aa6" }}>正在加载</p>
      ) : items.length === 0 ? (
        <p className="py-8 text-center text-[12px]" style={{ color: "#7a8aa6" }}>暂无抽卡记录</p>
      ) : (
        items.map((h, i) => (
          <div key={`${h.createdAt}-${i}`} className="grow">
            <RarityTag rarity={h.rarity} />
            <a href={articleHref(h)} target="_blank" rel="noreferrer">
              {h.title || h.page}
            </a>
            {h.byPity && <span className="side">保底</span>}
            <time className="sc-num">{formatDateTime(h.createdAt)}</time>
          </div>
        ))
      )}
    </div>
  );
}
