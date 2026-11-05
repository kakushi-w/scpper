const TZ = "Asia/Shanghai";

function format(iso: string | null | undefined, opts: Intl.DateTimeFormatOptions): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat("zh-CN", { timeZone: TZ, ...opts }).format(d);
}

export function formatDate(iso: string | null | undefined): string {
  return format(iso, { year: "numeric", month: "2-digit", day: "2-digit" });
}

export function formatDateTime(iso: string | null | undefined): string {
  return format(iso, { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });
}

export function formatCompact(n: number | null | undefined): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return "0";
  const abs = Math.abs(n);
  if (abs < 10000) return n.toLocaleString("zh-CN");
  if (abs < 100000000) return `${(n / 10000).toFixed(abs < 100000 ? 1 : 0)}万`;
  return `${(n / 100000000).toFixed(1)}亿`;
}

export function formatNumber(n: number | null | undefined): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return "0";
  return n.toLocaleString("zh-CN");
}

export function formatRating(n: number | null | undefined): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return "0";
  return n > 0 ? `+${n}` : String(n);
}

export function articleHref(a: { page: string }): string {
  return "/" + a.page.split("/").map(encodeURIComponent).join("/");
}

export const REASON_LABEL: Record<string, string> = {
  task: "任务奖励",
  admin_grant: "管理员发放",
  admin_clear: "管理员清空",
  recharge: "充值",
  correction: "账目更正",
  tip_in: "收到打赏",
  tip_out: "打赏",
  gacha_pull: "抽卡",
  gacha_buy: "购买卡牌",
  gacha_sell: "出售卡牌",
  stock_hold: "下单冻结",
  stock_unhold: "解除冻结",
  stock_settle: "成交结算",
  stock_margin: "保证金",
};

export function coinReasonLabel(reason: string): string {
  return REASON_LABEL[reason] ?? reason;
}
