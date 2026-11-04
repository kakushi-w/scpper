import { Container, ListSkeleton } from "@/components/ui";

export default function Loading() {
  return (
    <Container>
      <ListSkeleton rows={6} />
    </Container>
  );
}
