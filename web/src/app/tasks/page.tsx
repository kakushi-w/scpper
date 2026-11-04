"use client";

import { Container, EmptyState, ListSkeleton } from "@/components/ui";
import { loginHref, useSession } from "@/components/session";
import { api } from "@/lib/api";
import { useLoad } from "@/lib/load";
import { TasksView } from "./tasks-view";

function Tasks() {
  const { data, loading } = useLoad(() => api.tasks());
  if (loading) return <ListSkeleton rows={6} />;
  if (!data) return <EmptyState title="暂时无法获取任务" />;
  return <TasksView initial={data} />;
}

export default function TasksPage() {
  const { user, loading } = useSession();
  return (
    <Container className="page-narrow">
        {loading ? (
          <ListSkeleton rows={6} />
        ) : user ? (
          <Tasks />
        ) : (
          <EmptyState
            title="请登录后查看任务"
            action={
              <a href={loginHref()} className="sc-btn sc-btn-primary h-9 px-5">
                登录
              </a>
            }
          />
        )}
    </Container>
  );
}
