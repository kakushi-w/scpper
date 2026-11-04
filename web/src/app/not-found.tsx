import Link from "next/link";
import { Container, EmptyState } from "@/components/ui";

export default function NotFound() {
  return (
    <Container>
      <EmptyState
        title="该页面不存在"
        action={
          <div className="flex gap-2">
            <Link href="/" className="sc-btn sc-btn-primary h-9 px-5">返回首页</Link>
          </div>
        }
      />
    </Container>
  );
}
