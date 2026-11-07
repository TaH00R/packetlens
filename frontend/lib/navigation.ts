import {
  Activity,
  AlertTriangle,
  BookOpen,
  Download,
  Gauge,
  Globe,
  LayoutDashboard,
  Link2,
  Network,
  Radio,
  Settings,
  ShieldCheck,
  Waves,
} from "lucide-react";
export const navigation = [
  { group: "Workspace", label: "Overview", href: "/", icon: LayoutDashboard },
  { group: "Performance", label: "TCP analytics", href: "/tcp", icon: Network },
  { group: "Performance", label: "DNS analytics", href: "/dns", icon: Globe },
  { group: "Performance", label: "Jitter", href: "/jitter", icon: Waves },
  {
    group: "Performance",
    label: "DNS resolvers",
    href: "/resolvers",
    icon: Gauge,
  },
  {
    group: "Network health",
    label: "Availability",
    href: "/health",
    icon: ShieldCheck,
  },
  {
    group: "Network health",
    label: "Anomalies",
    href: "/anomalies",
    icon: Activity,
  },
  {
    group: "Network health",
    label: "Incidents",
    href: "/incidents",
    icon: AlertTriangle,
  },
  {
    group: "Network health",
    label: "Correlations",
    href: "/correlations",
    icon: Link2,
  },
  {
    group: "Operations",
    label: "Probe activity",
    href: "/activity",
    icon: Radio,
  },
  { group: "Operations", label: "Targets", href: "/targets", icon: Settings },
  { group: "Operations", label: "Exports", href: "/exports", icon: Download },
  {
    group: "Resources",
    label: "Metrics guide",
    href: "/learn",
    icon: BookOpen,
  },
];
export const navigationGroups = [
  ...new Set(navigation.map((item) => item.group)),
];
