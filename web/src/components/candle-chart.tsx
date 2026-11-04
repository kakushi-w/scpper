"use client";

import { useEffect, useRef, useState } from "react";
import type { Candle } from "@/lib/types";

const MA: [number, string][] = [
  [5, "#f0a20b"],
  [10, "#2f7be5"],
  [20, "#a24fd6"],
];

function movingAverage(candles: Candle[], n: number): (number | null)[] {
  let sum = 0;
  return candles.map((c, i) => {
    sum += c.close;
    if (i >= n) sum -= candles[i - n]!.close;
    return i >= n - 1 ? sum / n : null;
  });
}

// Monotone cubic keeps the curve from swinging past the values it joins, which
// a plain spline does whenever the average turns sharply.
function smoothCurve(ctx: CanvasRenderingContext2D, pts: [number, number][]) {
  const n = pts.length;
  if (n < 2) return;
  const d: number[] = [];
  const m: number[] = [];
  for (let i = 0; i < n - 1; i++) d.push((pts[i + 1]![1] - pts[i]![1]) / (pts[i + 1]![0] - pts[i]![0]));
  m.push(d[0]!);
  for (let i = 1; i < n - 1; i++) m.push(d[i - 1]! * d[i]! <= 0 ? 0 : (d[i - 1]! + d[i]!) / 2);
  m.push(d[n - 2]!);
  for (let i = 0; i < n - 1; i++) {
    if (d[i] === 0) {
      m[i] = 0;
      m[i + 1] = 0;
      continue;
    }
    const a = m[i]! / d[i]!;
    const b = m[i + 1]! / d[i]!;
    const h = a * a + b * b;
    if (h > 9) {
      const t = 3 / Math.sqrt(h);
      m[i] = t * a * d[i]!;
      m[i + 1] = t * b * d[i]!;
    }
  }
  ctx.beginPath();
  ctx.moveTo(pts[0]![0], pts[0]![1]);
  for (let i = 0; i < n - 1; i++) {
    const [x0, y0] = pts[i]!;
    const [x1, y1] = pts[i + 1]!;
    const dx = (x1 - x0) / 3;
    ctx.bezierCurveTo(x0 + dx, y0 + m[i]! * dx, x1 - dx, y1 - m[i + 1]! * dx, x1, y1);
  }
  ctx.stroke();
}

export function CandleChart({ candles }: { candles: Candle[] }) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [hover, setHover] = useState<number | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const wrap = wrapRef.current;
    if (!canvas || !wrap || candles.length === 0) return;

    const draw = () => {
      const css = (n: string) =>
        getComputedStyle(document.documentElement).getPropertyValue(n).trim();

      const dpr = window.devicePixelRatio || 1;
      const w = wrap.clientWidth;
      const h = 280;
      canvas.width = Math.round(w * dpr);
      canvas.height = Math.round(h * dpr);
      canvas.style.height = `${h}px`;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);

      const padL = 6;
      const padR = 56;
      const padT = 12;
      const volH = 46;
      const gap = 10;
      const priceH = h - padT - volH - gap - 18;
      const innerW = w - padL - padR;

      let hi = -Infinity;
      let lo = Infinity;
      let maxVol = 0;
      for (const c of candles) {
        if (c.high > hi) hi = c.high;
        if (c.low < lo) lo = c.low;
        if (c.volume > maxVol) maxVol = c.volume;
      }
      const pad = (hi - lo) * 0.08 || 1;
      hi += pad;
      lo -= pad;

      const y = (v: number) => padT + ((hi - v) / (hi - lo)) * priceH;

      ctx.strokeStyle = "#e6e6e6";
      ctx.fillStyle = css("--sc-text-3");
      ctx.font = '11px "Helvetica Neue", Arial, sans-serif';
      ctx.textBaseline = "middle";
      ctx.lineWidth = 1;
      for (let i = 0; i <= 4; i++) {
        const v = lo + ((hi - lo) * i) / 4;
        const yy = Math.round(y(v)) + 0.5;
        ctx.globalAlpha = 0.5;
        ctx.beginPath();
        ctx.moveTo(padL, yy);
        ctx.lineTo(padL + innerW, yy);
        ctx.stroke();
        ctx.globalAlpha = 1;
        ctx.fillText(v.toFixed(2), padL + innerW + 8, yy);
      }

      const up = css("--sc-negative");
      const down = css("--sc-positive");
      const slot = innerW / candles.length;
      const bw = Math.max(1.5, Math.min(11, slot * 0.66));

      candles.forEach((c, i) => {
        const cx = padL + slot * (i + 0.5);
        const rising = c.close >= c.open;
        ctx.strokeStyle = ctx.fillStyle = rising ? up : down;

        ctx.beginPath();
        ctx.moveTo(Math.round(cx) + 0.5, y(c.high));
        ctx.lineTo(Math.round(cx) + 0.5, y(c.low));
        ctx.stroke();

        const top = y(Math.max(c.open, c.close));
        const bot = y(Math.min(c.open, c.close));
        ctx.fillRect(cx - bw / 2, top, bw, Math.max(1, bot - top));

        if (maxVol > 0) {
          const vh = (c.volume / maxVol) * volH;
          ctx.globalAlpha = 0.45;
          ctx.fillRect(cx - bw / 2, padT + priceH + gap + (volH - vh), bw, vh);
          ctx.globalAlpha = 1;
        }
      });

      ctx.lineWidth = 1.3;
      ctx.lineJoin = "round";
      for (const [n, color] of MA) {
        const pts: [number, number][] = [];
        movingAverage(candles, n).forEach((v, i) => {
          if (v !== null) pts.push([padL + slot * (i + 0.5), y(v)]);
        });
        ctx.strokeStyle = color;
        smoothCurve(ctx, pts);
      }
      ctx.lineWidth = 1;

      if (hover !== null && candles[hover]) {
        const cx = padL + slot * (hover + 0.5);
        ctx.strokeStyle = css("--sc-primary");
        ctx.setLineDash([3, 3]);
        ctx.beginPath();
        ctx.moveTo(Math.round(cx) + 0.5, padT);
        ctx.lineTo(Math.round(cx) + 0.5, padT + priceH + gap + volH);
        ctx.stroke();
        ctx.setLineDash([]);
      }

      ctx.fillStyle = css("--sc-text-3");
      ctx.textBaseline = "alphabetic";
      ctx.fillText(label(candles[0]!.at), padL, h - 4);
      const lastLabel = label(candles[candles.length - 1]!.at);
      ctx.fillText(lastLabel, padL + innerW - ctx.measureText(lastLabel).width, h - 4);
    };

    draw();
    const ro = new ResizeObserver(draw);
    ro.observe(wrap);
    return () => ro.disconnect();
  }, [candles, hover]);

  function onMove(e: React.MouseEvent<HTMLCanvasElement>) {
    const wrap = wrapRef.current;
    if (!wrap || candles.length === 0) return;
    const rect = wrap.getBoundingClientRect();
    const x = e.clientX - rect.left - 6;
    const innerW = rect.width - 6 - 56;
    const i = Math.floor((x / innerW) * candles.length);
    setHover(i >= 0 && i < candles.length ? i : null);
  }

  const at = hover !== null ? hover : candles.length - 1;
  const active = candles[at];
  const averages = MA.map(([n, color]) => [n, color, movingAverage(candles, n)[at] ?? null] as const);

  if (candles.length === 0) {
    return (
      <div className="px-4 py-12 text-center text-[13px]" style={{ color: "var(--sc-text-3)" }}>
        暂无行情数据
      </div>
    );
  }

  return (
    <div>
      <div ref={wrapRef} className="relative w-full">
        <canvas
          ref={canvasRef}
          className="block w-full"
          onMouseMove={onMove}
          onMouseLeave={() => setHover(null)}
        />
      </div>
      {active && (
        <div
          className="sc-num flex flex-wrap gap-x-4 gap-y-1 border-t px-3 py-2 text-[11.5px]"
          style={{ borderColor: "#eee", color: "#666" }}
        >
          <span style={{ color: "var(--sc-text-3)" }}>{label(active.at)}</span>
          <span>开 {active.open.toFixed(2)}</span>
          <span>高 {active.high.toFixed(2)}</span>
          <span>低 {active.low.toFixed(2)}</span>
          <span
            style={{
              color: active.close >= active.open ? "var(--sc-negative)" : "var(--sc-positive)",
            }}
          >
            收 {active.close.toFixed(2)}
          </span>
          <span style={{ color: "var(--sc-text-3)" }}>量 {active.volume}</span>
          {averages.map(([n, color, v]) =>
            v === null ? null : (
              <span key={n} className="inline-flex items-center gap-1">
                <i style={{ display: "inline-block", width: 10, height: 2, background: color }} />
                MA{n} {v.toFixed(2)}
              </span>
            ),
          )}
        </div>
      )}
    </div>
  );
}

function label(at: string): string {
  if (at.length === 10) return at.slice(5);
  const d = new Date(at);
  return `${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")} ${String(
    d.getHours(),
  ).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}
