"use client";

import { Container, EmptyState, ListSkeleton } from "@/components/ui";
import { useSession } from "@/components/session";
import { api } from "@/lib/api";
import { useLoad } from "@/lib/load";
import { AdminView } from "./admin-view";

function Admin() {
  const { data, loading } = useLoad(() => api.admin.overview());
  if (loading) return <ListSkeleton rows={6} />;
  return <AdminView overview={data ?? {}} />;
}

export default function AdminPage() {
  const { user, loading } = useSession();
  return (
    <Container>
        {loading ? <ListSkeleton rows={6} /> : user?.role === "admin" ? <Admin /> : <EmptyState title="仅管理员可访问此页面" />}
    </Container>
  );
}
