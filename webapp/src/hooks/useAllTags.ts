import { fetchAllTags } from "@/api/client";
import { useQuery } from "@tanstack/react-query";

export const useAllTags = () => {
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ["allTags"],
    queryFn: fetchAllTags,
    staleTime: 1000 * 60 * 10,
  });

  return {
    tags: data?.data.tags || [],
    message: data?.message || "",
    isLoading,
    isError,
    error,
    refetch,
  };
};
