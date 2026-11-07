export const chartTooltipStyle = {
  background: "#101923",
  border: "1px solid #2a3948",
  borderRadius: 12,
  color: "#e2e8f0",
  boxShadow: "0 12px 32px #0006",
  fontSize: 12,
  padding: "10px 14px",
};
export const chartTick = { fill: "#94a3b8", fontSize: 11 };
export const chartGrid = {
  stroke: "#25313e",
  strokeDasharray: "2 6",
  vertical: false,
};
export function chartTime(value: number) {
  return new Date(value).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
export function chartValue(value: number) {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(
    value,
  );
}
