export function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null }
export function isNonnegative(value: unknown): value is number { return typeof value === "number" && Number.isFinite(value) && value >= 0 }
export function isCount(value: unknown): value is number { return isNonnegative(value) && Number.isSafeInteger(value) }
export function isTimestamp(value: unknown): value is string { return typeof value === "string" && Number.isFinite(Date.parse(value)) }
