"use client";

import { useEffect } from "react";
import Link from "next/link";
import { Container, EmptyState } from "@/components/ui";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <Container>
      <EmptyState
        title="页面发生错误"
        description={error.message || "发生意外错误，请重试。"}
        action={
          <div className="flex gap-2">
            <button type="button" onClick={reset} className="sc-btn sc-btn-primary h-9 px-5">
              重试
            </button>
            <Link href="/" className="sc-btn h-9 px-5">返回首页</Link>
          </div>
        }
      />
    </Container>
  );
}
