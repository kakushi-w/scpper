"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { loginHref, useSession } from "./session";
import { formatNumber } from "@/lib/format";

const NAV = [
  { href: "/", label: "云币" },
  { href: "/stock", label: "指数" },
  { href: "/gacha", label: "抽卡" },
  { href: "/tasks", label: "任务" },
  { href: "/rank", label: "财富榜" },
  { href: "/me/coins", label: "收支" },
];

function active(pathname: string, href: string): boolean {
  const path = pathname.replace(/\/$/, "") || "/";
  if (href === "/") return path === "/";
  return path === href || path.startsWith(href + "/");
}

function Who() {
  const { user, loading } = useSession();
  if (loading) return null;
  if (!user) {
    return (
      <a className="login" href={loginHref()}>
        登录
      </a>
    );
  }
  return (
    <>
      <span className="name">{user.name}</span>
      <span>
        <b>{formatNumber(user.coins)}</b> 云币
      </span>
    </>
  );
}

export function Shell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const { user } = useSession();
  const items = user?.role === "admin" ? [...NAV, { href: "/admin", label: "管理" }] : NAV;

  return (
    <>
      <header className="topbar">
        <div className="topbar-in">
          <nav className="topbar-tabs">
            {items.map((item) => (
              <Link key={item.href} href={item.href} className={active(pathname, item.href) ? "on" : ""}>
                {item.label}
              </Link>
            ))}
          </nav>
          <div className="topbar-who">
            <Who />
          </div>
        </div>
      </header>
      <main>{children}</main>
      <footer className="foot">©2026 由Kakushi制作</footer>
    </>
  );
}
