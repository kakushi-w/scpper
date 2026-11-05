import type { Metadata, Viewport } from "next";
import { Shell } from "@/components/shell";
import { SessionProvider } from "@/components/session";
import "@/styles/globals.css";

export const metadata: Metadata = {
  title: { default: "SCPPER-CLOUD", template: "%s" },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>
        <SessionProvider>
          <Shell>{children}</Shell>
        </SessionProvider>
      </body>
    </html>
  );
}
