import { articleHref, formatRating } from "@/lib/format";
import type { GachaCard, Rarity } from "@/lib/types";
import scpLine from "@/assets/scp-line.png";
import scpWhite from "@/assets/scp-line-white.png";

export const RARITIES: Rarity[] = ["ur", "ssr", "sr", "r", "n"];

export const RARITY_LABEL: Record<Rarity, string> = {
  ur: "UR",
  ssr: "SSR",
  sr: "SR",
  r: "R",
  n: "N",
};

export const RARITY_RANGE: Record<Rarity, string> = {
  ur: "60 分以上",
  ssr: "30 ~ 59 分",
  sr: "15 ~ 29 分",
  r: "6 ~ 14 分",
  n: "0 ~ 5 分",
};

export function RarityTag({ rarity }: { rarity: Rarity }) {
  return <span className={`rtag ${rarity}`}>{RARITY_LABEL[rarity]}</span>;
}

export function CardFace({ card, qty, isNew }: { card: GachaCard; qty?: number; isNew?: boolean }) {
  return (
    <div className={`cd ${card.rarity}`}>
      <img className="wm" src={scpLine.src} alt="" />
      <span className="rl">{RARITY_LABEL[card.rarity]}</span>
      {isNew && <span className="nw">NEW</span>}
      {qty !== undefined && qty > 1 && <span className="qty">x{qty}</span>}
      <a className="ti" href={articleHref(card)} target="_blank" rel="noreferrer">
        {card.title || card.page}
      </a>
      <div className="ft">
        <span>{card.author || "无作者"}</span>
        <b>{formatRating(card.rating)}</b>
      </div>
    </div>
  );
}

export function CardBack() {
  return (
    <div className="cd-back">
      <img src={scpWhite.src} alt="" />
    </div>
  );
}
