import { fetchUserInfo } from "@/api/client";
import { addToast } from "@heroui/react";
import {
  type QueryClient,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

const handleLogout = (queryClient: QueryClient) => {
  queryClient.invalidateQueries({ queryKey: ["userInfo"] });
  queryClient.removeQueries({ queryKey: ["userInfo"] });

  addToast({
    title: "三秒后跳转到统一认证界面，跳转后请点击注销",
    timeout: 3000,
    color: "danger",
  });

  setTimeout(() => {
    window.location.href = "https://auth.darkeli.com";
  }, 3000);
};

export const useUserInfo = () => {
  const queryClient = useQueryClient();

  const {
    data: userInfo,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["userInfo"],
    queryFn: fetchUserInfo,
    staleTime: 1000 * 60 * 5,
  });

  return {
    userInfo,
    isLoading,
    isError,
    logout: () => handleLogout(queryClient),
  };
};
