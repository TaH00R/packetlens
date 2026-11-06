
import type { Metadata } from "next";
import "./globals.css";

import { MonitoringProvider } from "@/context/MonitoringContext";
import DashboardShell from "@/components/layout/DashboardShell";

export const metadata: Metadata = {
  title: "PacketLens | Network Observability",
  description: "Real-time network monitoring and analytics.",
  icons: {
    icon: "/brand/packetlens-icon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        <MonitoringProvider>
          <DashboardShell>
            {children}
          </DashboardShell>
        </MonitoringProvider>
      </body>
    </html>
  );
}
