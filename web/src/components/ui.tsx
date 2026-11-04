"use client";

import { useState, type CSSProperties, type ReactNode } from "react";
import { formatCompact } from "@/lib/format";
import scpLine from "@/assets/scp-line.png";

export function Container({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`page ${className}`}>{children}</div>;
}

export function Section({ title, action, children }: { title?: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="mb-6">
      {(title || action) && (
        <div className="mb-2 flex min-h-8 items-center justify-between gap-3">
          {title ? <h2 className="text-[14px] font-medium">{title}</h2> : <span />}
          {action}
        </div>
      )}
      {children}
    </section>
  );
}

export function Panel({ children, className = "", style }: { children: ReactNode; className?: string; style?: CSSProperties }) {
  return (
    <div className={`sc-panel ${className}`} style={style}>
      {children}
    </div>
  );
}

const TONE = {
  primary: "var(--sc-primary)",
  coin: "var(--sc-coin)",
  positive: "var(--sc-positive)",
  negative: "var(--sc-negative)",
};

export function StatTile({
  label,
  value,
  tone,
}: {
  label: string;
  value: string | number;
  tone?: keyof typeof TONE;
}) {
  return (
    <div className="flex items-baseline justify-between gap-3 border-b px-3 py-2 text-[12px]" style={{ borderColor: "#f2f2f2" }}>
      <span style={{ color: "#888" }}>{label}</span>
      <b className="sc-num text-[15px] font-medium" style={{ color: tone ? TONE[tone] : "#333" }}>
        {typeof value === "number" ? formatCompact(value) : value}
      </b>
    </div>
  );
}

export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="empty">
      <img src={scpLine.src} alt="" />
      <h3>{title}</h3>
      {description && <p>{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className="msg err" style={{ margin: 0 }}>
      {message || "加载失败"}
    </div>
  );
}

export function ListSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="block">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="border-b px-3 py-3 last:border-b-0" style={{ borderColor: "#f2f2f2" }}>
          <div className="sc-skeleton h-4 w-2/3" />
          <div className="sc-skeleton mt-2 h-3 w-1/3" />
        </div>
      ))}
    </div>
  );
}

export function ProgressBar({ value, max, tone = "primary" }: { value: number; max: number; tone?: "primary" | "coin" | "positive" }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div className="h-1 w-full" style={{ background: "#eef0f3" }}>
      <div className="h-full transition-[width] duration-300" style={{ width: `${pct}%`, background: TONE[tone] }} />
    </div>
  );
}

const AVATAR_COLORS = ["#8a9bb0", "#b0938a", "#8ab09a", "#a08ab0", "#b0a98a", "#7f9fb8", "#b88a8a"];

export function Avatar({ name, url, size = 36 }: { name: string; url?: string | null; size?: number }) {
  const [failed, setFailed] = useState(false);
  if (url && !failed) {
    return (
      <img
        className="av"
        src={url}
        alt=""
        width={size}
        height={size}
        style={{ width: size, height: size, objectFit: "cover", background: "#eef0f3" }}
        onError={() => setFailed(true)}
      />
    );
  }
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0;
  return (
    <span
      className="av"
      style={{ width: size, height: size, fontSize: size * 0.38, background: AVATAR_COLORS[h % AVATAR_COLORS.length] }}
      aria-hidden
    >
      {[...name.trim()][0]?.toUpperCase() ?? "?"}
    </span>
  );
}

export function Pager({ page, pages, onPage }: { page: number; pages: number; onPage: (p: number) => void }) {
  if (pages <= 1) return null;
  return (
    <div className="pager">
      <button type="button" className="sc-btn h-7 px-3 text-[12px]" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        上一页
      </button>
      <span className="sc-num">
        {page} / {pages}
      </span>
      <button type="button" className="sc-btn h-7 px-3 text-[12px]" disabled={page >= pages} onClick={() => onPage(page + 1)}>
        下一页
      </button>
    </div>
  );
}
