"use client";

import { useEffect, useRef, useState } from "react";
import type { Candle } from "@/lib/types";

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

  const active = hover !== null ? candles[hover] : candles[candles.length - 1];

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
