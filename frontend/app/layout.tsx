
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "PacketLens | Network Observability",
  description:
    "Real-time network monitoring and analytics.",
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
      <body>{children}</body>
    </html>
  );
}
