"use client";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { navigation, navigationGroups } from "@/lib/navigation";
export default function DashboardSidebar() {
  const pathname = usePathname();
  return (
    <aside className="sticky top-0 hidden h-screen w-[232px] shrink-0 flex-col border-r border-white/[0.07] bg-[#0D131A] lg:flex">
      <Link href="/" className="border-b border-white/[0.07] px-6 py-5">
        <Image
          src="/brand/packetlens-logo.svg"
          alt="PacketLens"
          width={200}
          height={55}
          priority
          className="h-auto w-[178px]"
        />
      </Link>
      <nav
        aria-label="Main navigation"
        className="min-h-0 flex-1 space-y-5 overflow-y-auto px-3 py-5"
      >
        {navigationGroups.map((group) => (
          <div key={group}>
            <p className="mb-2 px-3 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-500">
              {group}
            </p>
            <div className="space-y-1">
              {navigation
                .filter((item) => item.group === group)
                .map(({ label, href, icon: Icon }) => (
                  <Link
                    key={href}
                    href={href}
                    aria-current={pathname === href ? "page" : undefined}
                    className={`flex items-center gap-3 rounded-lg border px-3 py-2 text-sm transition-colors ${pathname === href ? "border-teal-500/20 bg-teal-500/10 text-teal-300" : "border-transparent text-slate-400 hover:bg-white/[0.04] hover:text-slate-100"}`}
                  >
                    <Icon size={16} aria-hidden="true" />
                    {label}
                  </Link>
                ))}
            </div>
          </div>
        ))}
      </nav>
    </aside>
  );
}
