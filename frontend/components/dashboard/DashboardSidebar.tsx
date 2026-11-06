
"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";

import {
  Activity,
  BookOpen,
  Globe,
  LayoutDashboard,
  Network,
} from "lucide-react";

import type { ConnectionStatus } from "@/types/metrics";

interface Props {
  status: ConnectionStatus;
}

const navigation = [
  { label: "Overview", icon: LayoutDashboard, href: "/" },
  { label: "TCP Analytics", icon: Network, href: "/tcp" },
  { label: "DNS Analytics", icon: Globe, href: "/dns" },
  { label: "Network Health", icon: Activity, href: "/health" },
];

export default function DashboardSidebar({ status }: Props) {
  const pathname = usePathname();

  const linkClass = (active: boolean) =>
    `flex items-center gap-3 rounded-lg border px-3 py-3 text-sm transition-all ${
      active
        ? "border-teal-500/20 bg-teal-500/10 text-teal-400"
        : "border-transparent text-gray-400 hover:bg-white/[0.04] hover:text-white"
    }`;

  const learnActive = pathname === "/learn";

  return (
    <aside className="hidden w-[250px] shrink-0 border-r border-white/[0.07] bg-[#0D131A] lg:flex lg:flex-col">
      <div className="border-b border-white/[0.07] px-6 py-6">
        <Link href="/">
          <Image
            src="/brand/packetlens-logo.svg"
            alt="PacketLens"
            width={200}
            height={55}
            priority
            className="h-auto w-[190px]"
          />
        </Link>
      </div>

      <div className="px-4 py-6">
        <p className="mb-3 px-3 text-[10px] font-semibold uppercase tracking-[0.18em] text-gray-600">
          Workspace
        </p>

        <nav className="space-y-1">
          {navigation.map(({ label, icon: Icon, href }) => {
            const active = pathname === href;

            return (
              <Link
                key={label}
                href={href}
                aria-current={active ? "page" : undefined}
                className={linkClass(active)}
              >
                <Icon size={17} />
                {label}
              </Link>
            );
          })}
        </nav>

        <p className="mb-3 mt-9 px-3 text-[10px] font-semibold uppercase tracking-[0.18em] text-gray-600">
          Resources
        </p>

        <Link
          href="/learn"
          aria-current={learnActive ? "page" : undefined}
          className={linkClass(learnActive)}
        >
          <BookOpen size={17} />
          Understanding Metrics
        </Link>
      </div>
    </aside>
  );
}
