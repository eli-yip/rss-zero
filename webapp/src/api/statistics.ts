import { apiUrl } from "@/config/config";

export async function fetchStatistics(): Promise<Record<string, number>> {
  const response = await fetch(`${apiUrl}/archive/statistics`);

  if (!response.ok) {
    throw new Error("Failed to fetch statistics");
  }

  return await response.json();
}
