import { apiUrl } from "@/config/config";
import type { Topic } from "@/types/topic";
import axios, { type AxiosInstance, type AxiosResponse } from "axios";

// 创建 axios 实例
const apiClient: AxiosInstance = axios.create({
  baseURL: apiUrl,
  headers: {
    "Content-Type": "application/json",
  },
  timeout: 10000, // 设置超时时间为 10 秒
});

// 添加一个响应拦截器处理错误
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const errorMessage = error.response?.data?.message || "请求失败";
    return Promise.reject(new Error(errorMessage));
  },
);

export const ContentType = {
  Answer: "answer",
  Pin: "pin",
  Article: "article",
} as const;

export type ContentType = (typeof ContentType)[keyof typeof ContentType];

// 定义请求体类型
interface ArchiveRequest {
  /**
   * 作者 url token，例如：canglimo
   */
  author: string;
  /**
   * 数量
   */
  count: number;
  /**
   * 页码
   */
  page: number;
  /**
   * 平台
   */
  platform: string;
  /**
   * 类型
   */
  type: string;
  /**
   * 开始日期
   */
  start_date: string;
  /**
   * 结束日期
   */
  end_date: string;
  order: number;
}

// 定义分页信息类型
interface ArchivePaging {
  /**
   * 当前页码
   */
  current: number;
  /**
   * 页数
   */
  total: number;
}

export interface ArchiveResponse {
  count: number;
  paging: ArchivePaging;
  topics: Topic[];
}

interface RandomRequest {
  author: string;
  count: number;
  platform: string;
  type: string;
}

export interface RandomResponse {
  topics: Topic[];
}

/**
 * 获取归档话题
 * @param page 页码
 * @param start_date 开始日期
 * @param end_date 结束日期
 * @param type 内容类型
 * @param author 作者
 * @param order 排序方式
 * @returns 归档响应
 */
export async function fetchArchiveTopics(
  page: number,
  start_date = "",
  end_date = "",
  type: string = ContentType.Answer,
  author = "canglimo",
  order = 0,
): Promise<ArchiveResponse> {
  const requestBody: ArchiveRequest = {
    platform: "zhihu",
    type: type,
    count: 10,
    page: page,
    author: author,
    start_date: start_date,
    end_date: end_date,
    order: order,
  };

  try {
    const response: AxiosResponse<ArchiveResponse> = await apiClient.post(
      "/archive",
      requestBody,
    );
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("获取归档话题失败");
  }
}

/**
 * 获取随机话题
 * @returns 随机话题响应
 */
export async function fetchRandomTopics(): Promise<RandomResponse> {
  const requestBody: RandomRequest = {
    platform: "zhihu",
    type: "answer",
    author: "canglimo",
    count: 10,
  };

  try {
    const response: AxiosResponse<RandomResponse> = await apiClient.post(
      "/archive/random",
      requestBody,
    );
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("获取随机话题失败");
  }
}

/**
 * 获取统计数据
 * @returns 统计数据
 */
export async function fetchStatistics(): Promise<Record<string, number>> {
  try {
    const response: AxiosResponse<Record<string, number>> = await apiClient.get(
      "/archive/statistics",
    );
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("获取统计数据失败");
  }
}
