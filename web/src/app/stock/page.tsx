"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Container, ErrorState, ListSkeleton } from "@/components/ui";
import { api } from "@/lib/api";
import { useLoad } from "@/lib/load";
import { StockView } from "./stock-view";

function Stock() {
  const asked = (useSearchParams().get("symbol") || "").toUpperCase();
  const { data, error, loading } = useLoad(() => api.stockSymbols().then((r) => r.items), []);
  if (loading) return <Container><ListSkeleton rows={6} /></Container>;
  if (error || !data) return <Container><ErrorState message={error} /></Container>;
  return <StockView initialSymbols={data} initialSymbol={asked} />;
}

export default function StockPage() {
  return (
    <Suspense>
      <Stock />
    </Suspense>
  );
}
