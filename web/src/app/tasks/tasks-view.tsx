"use client";

import { useState } from "react";
import { ErrorState, ProgressBar } from "@/components/ui";
import { useSession } from "@/components/session";
import { api } from "@/lib/api";
import { formatNumber } from "@/lib/format";
import type { TaskRow, TasksResponse } from "@/lib/types";

export function TasksView({ initial }: { initial: TasksResponse }) {
  const { refresh } = useSession();
  const [data, setData] = useState(initial);
  const [claiming, setClaiming] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [toast, setToast] = useState("");

  async function claim(code: string) {
    setClaiming(code);
    setError("");
    setToast("");
    try {
      const res = await api.claimTask(code);
      setToast(`领取成功，获得 ${res.reward} 云币，余额 ${formatNumber(res.balance)}`);
      const next = await api.tasks();
      setData(next);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "领取失败");
    } finally {
      setClaiming(null);
    }
  }

  async function refreshTasks() {
    setRefreshing(true);
    setError("");
    setToast("");
    try {
      setData(await api.refreshTasks());
      setToast("已重新判定任务进度");
    } catch (e) {
      setError(e instanceof Error ? e.message : "判定失败");
    } finally {
      setRefreshing(false);
    }
  }

  const allTasks = [...data.daily, ...(data.weekly ?? []), ...data.monthly];
  const completed = allTasks.filter((t) => t.completedAt).length;

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3 text-[13.5px]">
        <p>
          已完成 <b className="sc-num">{completed}</b> 项，共 {allTasks.length} 项
          {data.claimableCoins > 0 && (
            <>
              ，另有 <b className="sc-num" style={{ color: "var(--sc-coin)" }}>{data.claimableCoins}</b> 云币待领取
            </>
          )}
        </p>
        <button
          type="button"
          onClick={refreshTasks}
          disabled={refreshing}
          className="sc-btn h-8 px-3 text-[12.5px]"
        >
          {refreshing ? "正在判定" : "重新判定进度"}
        </button>
      </div>

      {error && <div className="mb-3"><ErrorState message={error} /></div>}
      {toast && (
        <div
          className="msg mb-3"
          style={{ margin: "0 0 12px" }}
        >
          {toast}
        </div>
      )}

      <div className="block mb-3">
        <div className="block-h">{`每日任务（${data.dailyPeriod}）`}</div>
          {data.daily.map((t) => (
            <TaskItem key={t.code} task={t} claiming={claiming === t.code} onClaim={claim} />
          ))}
      </div>

      {(data.weekly ?? []).length > 0 && (
        <div className="block mb-3">
          <div className="block-h">{`每周任务（${data.weeklyPeriod}）`}</div>
            {data.weekly.map((t) => (
              <TaskItem key={t.code} task={t} claiming={claiming === t.code} onClaim={claim} />
            ))}
        </div>
      )}

      <div className="block mb-3">
        <div className="block-h">{`每月任务（${data.monthlyPeriod}）`}</div>
          {data.monthly.map((t) => (
            <TaskItem key={t.code} task={t} claiming={claiming === t.code} onClaim={claim} />
          ))}
      </div>
    </>
  );
}

function TaskItem({
  task,
  claiming,
  onClaim,
}: {
  task: TaskRow;
  claiming: boolean;
  onClaim: (code: string) => void;
}) {
  const done = Boolean(task.completedAt);
  const claimed = Boolean(task.claimedAt);
  const pct = Math.min(task.progress, task.target);

  return (
    <div
      className="border-b px-3 py-3 last:border-b-0"
      style={{ borderColor: "#f2f2f2" }}
    >
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-[14px] font-medium">{task.title}</h3>
            <span
              className="sc-tag sc-num"
              style={{ borderColor: "var(--sc-coin)", color: "var(--sc-coin)" }}
            >
              +{task.reward} 云币
            </span>
          </div>

          {task.description && (
            <p className="mt-1 text-[12.5px]" style={{ color: "var(--sc-text-2)" }}>
              {task.description}
            </p>
          )}

          <div className="mt-2">
            <ProgressBar
              value={pct}
              max={task.target}
              tone={done ? "positive" : "primary"}
            />
            <div
              className="sc-num mt-1 text-[11.5px]"
              style={{ color: done ? "var(--sc-positive)" : "var(--sc-text-3)" }}
            >
              {pct} / {task.target}
              {done && !claimed && "，已完成，可领取"}
            </div>
          </div>
        </div>

        <div className="shrink-0 pt-0.5">
          {claimed ? (
            <span
              className="inline-flex h-8 w-[72px] items-center justify-center text-[12.5px]"
              style={{ color: "var(--sc-text-3)" }}
            >
              已领取
            </span>
          ) : (
            <button
              type="button"
              disabled={!done || claiming}
              onClick={() => onClaim(task.code)}
              className={`sc-btn h-8 w-[72px] text-[12.5px] ${done ? "sc-btn-primary" : ""}`}
            >
              {claiming ? "正在领取" : done ? "领取" : "未完成"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
