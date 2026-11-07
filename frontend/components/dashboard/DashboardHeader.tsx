"use client";

import { navigation } from "@/lib/navigation";
import { RefreshCw } from "lucide-react";
import Image from "next/image";
import { usePathname } from "next/navigation";
import type { ConnectionStatus } from "@/types/metrics";

interface Props {
  status: ConnectionStatus;
  onReconnect: () => void;
}

export default function DashboardHeader({ status, onReconnect }: Props) {
  const pathname = usePathname();
  const pageTitle =
    navigation.find((item) => item.href === pathname)?.label ?? "PacketLens";

  return (
    <header className="sticky top-0 z-20 flex h-[72px] items-center justify-between border-b border-white/[0.07] bg-[#0B1016]/95 px-5 backdrop-blur-xl sm:px-8">
      <div className="flex items-center gap-3 lg:hidden">
        <Image
          src="/brand/packetlens-icon.svg"
          alt="PacketLens"
          width={32}
          height={32}
        />
        <span className="font-semibold">
          Packet<span className="text-teal-400">Lens</span>
        </span>
      </div>

      <div className="hidden text-sm text-gray-500 lg:block">
        Workspace <span className="mx-2">/</span>
        <span className="text-gray-200">{pageTitle}</span>
      </div>

      <div className="ml-auto flex items-center gap-4">
        <div className="flex items-center gap-2 text-xs">
          <span
            className={`h-2 w-2 rounded-full ${
              status === "connected"
                ? "animate-pulse bg-teal-400"
                : "bg-yellow-400"
            }`}
          />
          <span className="capitalize text-gray-300">
            {status === "connected" ? "Live monitoring" : status}
          </span>
        </div>

        <button
          onClick={onReconnect}
          aria-label="Reconnect WebSocket"
          className="rounded-lg border border-white/[0.08] p-2 text-gray-400 transition hover:border-teal-500/40 hover:text-teal-400"
        >
          <RefreshCw size={16} />
        </button>
      </div>
    </header>
  );
}
