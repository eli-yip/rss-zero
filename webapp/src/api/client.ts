import { apiUrl } from "@/config/config";
import type { Topic } from "@/types/Topic";
import { addToast } from "@heroui/react";
import axios, { type AxiosInstance, type AxiosResponse } from "axios";

const apiClient: AxiosInstance = axios.create({
  baseURL: apiUrl,
  headers: {
    "Content-Type": "application/json",
  },
  timeout: 10000,
});

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const errorMessage = error.response?.data?.message || "请求失败";
    const requestId = error.response?.headers?.["X-Request-Id	"] || "未知 ID";
    addToast({
      title: errorMessage,
      description: `请求 ID: ${requestId}`,
      timeout: 3000,
      color: "warning",
    });
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

interface UserInfoResponse {
  data: {
    username: string;
    nickname: string;
  };
}

export const fetchUserInfo = async () => {
  try {
    const response: AxiosResponse<UserInfoResponse> =
      await apiClient.get("/user");
    return response.data.data;
  } catch (error) {
    if (axios.isAxiosError(error)) {
      const errorMessage = error.response?.data?.message || "获取用户信息失败";
      throw new Error(errorMessage);
    }
    throw new Error("获取用户信息失败");
  }
};

export const ContentTypeV2 = {
  Answer: 0,
  Article: 1,
  Pin: 2,
} as const;

export type ContentTypeV2 = (typeof ContentTypeV2)[keyof typeof ContentTypeV2];

interface NewBookmarkRequest {
  content_type: ContentTypeV2;
  content_id: string;
}

interface NewBookmarkResponse {
  message: string;
  data: {
    bookmark_id: string;
  };
}

export async function addBookmark(
  content_type: ContentTypeV2,
  content_id: string,
): Promise<NewBookmarkResponse> {
  const requestBody: NewBookmarkRequest = {
    content_type,
    content_id,
  };

  try {
    const response: AxiosResponse<NewBookmarkResponse> = await apiClient.put(
      "/bookmark",
      requestBody,
    );
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("添加收藏失败");
  }
}

export async function removeBookmark(
  bookmark_id: string,
): Promise<NewBookmarkResponse> {
  try {
    const response: AxiosResponse<NewBookmarkResponse> = await apiClient.delete(
      `/bookmark/${bookmark_id}`,
    );
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("删除收藏失败");
  }
}

interface BookmarkUpdateRequest {
  comment: string | null;
  note: string | null;
  tags: string[] | null;
}

export async function updateBookmark(
  bookmark_id: string,
  note: string | null = null,
  tags: string[] | null = null,
  comment: string | null = null,
): Promise<NewBookmarkResponse> {
  const requestBody: BookmarkUpdateRequest = {
    comment,
    note,
    tags,
  };

  try {
    const response: AxiosResponse<NewBookmarkResponse> = await apiClient.patch(
      `/bookmark/${bookmark_id}`,
      requestBody,
    );
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("更新收藏失败");
  }
}

interface AllTagsResponse {
  message: string;
  data: {
    tags: {
      name: string;
      count: number;
    }[];
  };
}

export async function fetchAllTags(): Promise<AllTagsResponse> {
  try {
    const response: AxiosResponse<AllTagsResponse> =
      await apiClient.get("/tag");
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("获取所有标签失败");
  }
}

export const TimeBy = {
  Create: 0,
  Update: 1,
} as const;

export type TimeBy = (typeof TimeBy)[keyof typeof TimeBy];

interface BookmarkListRequest {
  page: number;
  tags: TagFilter | null;
  start_date: string;
  end_date: string;
  date_by: TimeBy;
  order: number;
  order_by: TimeBy;
}

interface TagFilter {
  include: string[] | null;
  exclude: string[] | null;
  no_tag: boolean;
}

interface BookmarkListResponse {
  message: string;
  data: {
    count: number;
    paging: ArchivePaging;
    topics: Topic[];
  } | null;
}

/**
 * 获取书签列表
 * @param page 页码
 * @param tags 标签过滤配置
 * @param start_date 开始日期
 * @param end_date 结束日期
 * @param date_by 日期类型（创建时间或更新时间）
 * @param order 排序方式（0 为降序，1 为升序）
 * @param order_by 排序依据（创建时间或更新时间）
 * @returns 书签列表响应
 */
export async function fetchBookmarkList(
  page: number,
  tag_include: string[] | null = null,
  tag_exclude: string[] | null = null,
  no_tag = false,
  start_date = "",
  end_date = "",
  date_by: TimeBy = TimeBy.Create,
  order = 0,
  order_by: TimeBy = TimeBy.Create,
): Promise<BookmarkListResponse> {
  let tags: TagFilter | null = null;
  if (tag_include || tag_exclude || no_tag) {
    tags = {
      include: null,
      exclude: null,
      no_tag: false,
    };
    if (tag_include && tag_include.length > 0) {
      tags.include = tag_include;
    }
    if (tag_exclude && tag_exclude.length > 0) {
      tags.exclude = tag_exclude;
    }
    if (no_tag) {
      tags.no_tag = no_tag;
    }
  }
  const requestBody: BookmarkListRequest = {
    page,
    tags,
    start_date,
    end_date,
    date_by,
    order,
    order_by,
  };

  try {
    const response: AxiosResponse<BookmarkListResponse> = await apiClient.post(
      "/bookmark",
      requestBody,
    );
    return response.data;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error("获取书签列表失败");
  }
}
