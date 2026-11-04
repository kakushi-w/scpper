"use client";

import { useEffect, useState } from "react";
import { CardBack, CardFace, RARITIES, RARITY_LABEL } from "./gacha-card";
import type { GachaCard } from "@/lib/types";

export function GachaResult({
  cards,
  busy,
  canAgain,
  againLabel,
  onClose,
  onAgain,
}: {
  cards: GachaCard[] | null;
  busy: number;
  canAgain: boolean;
  againLabel: string;
  onClose: () => void;
  onAgain: () => void;
}) {
  const [open, setOpen] = useState<boolean[]>([]);

  useEffect(() => {
    setOpen([]);
    if (cards?.length !== 1) return;
    const t = setTimeout(() => setOpen([true]), 380);
    return () => clearTimeout(t);
  }, [cards]);

  const shown = cards ?? [];
  const count = busy || shown.length;
  const done = !busy && shown.length > 0 && shown.every((_, i) => open[i]);
  const flip = (i: number) =>
    setOpen((o) => {
      const next = [...o];
      next[i] = true;
      return next;
    });
  const best = RARITIES.find((r) => shown.some((c) => c.rarity === r));
  const fresh = shown.filter((c) => c.isNew).length;

  return (
    <div className="ov" role="dialog" aria-label="抽卡结果">
      <h4>{busy ? "抽卡中" : "抽卡结果"}</h4>
      <div className={`cards${count === 1 ? " one" : ""}`}>
        {Array.from({ length: count }, (_, i) => {
          const c = busy ? undefined : shown[i];
          return (
            <div key={i} className={`flip${c && open[i] ? " open" : ""}`} style={{ animationDelay: `${i * 0.05}s` }}>
              <div className="flip-in">
                <div className="flip-face">
                  <button type="button" disabled={!c} onClick={() => flip(i)} aria-label={`翻开第 ${i + 1} 张`}>
                    <CardBack />
                  </button>
                </div>
                {c && (
                  <div className="flip-face front">
                    <CardFace card={c} isNew={c.isNew} />
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
      {done && (
        <p className="ovs">
          最高 {best ? RARITY_LABEL[best] : "N"}，新卡 {fresh} 张
          {shown.some((c) => c.byPity) ? "，含保底" : ""}
        </p>
      )}
      <div className="ovb">
        {!done && !busy && shown.length > 1 ? (
          <button type="button" className="ok" onClick={() => setOpen(shown.map(() => true))}>
            全部翻开
          </button>
        ) : (
          <button type="button" className="ok" onClick={onClose} disabled={!done}>
            确定
          </button>
        )}
        {canAgain && done && (
          <button type="button" className="ag" onClick={onAgain}>
            {againLabel}
          </button>
        )}
      </div>
    </div>
  );
}
