import { ContentType } from "@/api/client";

export const AUTHORS = [
  { key: "canglimo", name: "墨苍离" },
  { key: "zi-e-79-23", name: "别打扰我修道" },
  { key: "fu-lan-ke-yang", name: "弗兰克扬" },
] as const;

export const SORT_OPTIONS = {
  NEWEST_FIRST: 0,
  OLDEST_FIRST: 1,
} as const;

export const isValidAuthor = (value: string | null): boolean =>
  value !== null && AUTHORS.some((author) => author.key === value);

export const isValidContentType = (
  value: string | null,
): value is ContentType =>
  value !== null && Object.values(ContentType).includes(value as ContentType);

export const isValidOrder = (value: string | null): boolean => {
  const orderNum = value !== null ? Number.parseInt(value) : Number.NaN;
  return (
    orderNum === SORT_OPTIONS.NEWEST_FIRST ||
    orderNum === SORT_OPTIONS.OLDEST_FIRST
  );
};
