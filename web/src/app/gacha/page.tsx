"use client";

import { Container, ErrorState, ListSkeleton } from "@/components/ui";
import { api } from "@/lib/api";
import { useLoad } from "@/lib/load";
import { GachaView } from "./gacha-view";

export default function GachaPage() {
  const { data, error, loading } = useLoad(() => api.gachaInfo());
  if (loading) return <Container><ListSkeleton rows={6} /></Container>;
  if (error) return <Container><ErrorState message={error} /></Container>;
  return <GachaView initial={data} />;
}
