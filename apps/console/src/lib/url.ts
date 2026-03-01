export function buildSearchParams(
  searchParams: { toString(): string },
  key: string,
  value: string | null,
): string {
  const params = new URLSearchParams(searchParams.toString());
  if (!value || value === "all") {
    params.delete(key);
  } else {
    params.set(key, value);
  }
  const qs = params.toString();
  return qs ? "?" + qs : "";
}
