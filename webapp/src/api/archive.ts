import { apiUrl } from "@/config/config";
import { Topic } from "@/types/topic";

// 定义请求体类型
export interface ArchiveRequest {
  /**
   * 作者url token，例如：canglimo
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
}

// 定义分页信息类型
export interface ArchivePaging {
  /**
   * 当前页码
   */
  current: number;
  /**
   * 页数
   */
  total: number;
}

// 定义响应数据的类型
export interface ArchiveResponse {
  /**
   * 总数量
   */
  count: number;
  /**
   * 分页信息
   */
  paging: ArchivePaging;
  topics: Topic[];
}

// 请求接口函数
export async function fetchArchiveTopics(
  page: number,
): Promise<ArchiveResponse> {
  const requestBody: ArchiveRequest = {
    platform: "zhihu",
    type: "answer",
    count: 10,
    page: page,
    author: "canglimo",
  };

  const response = await fetch(`${apiUrl}/archive`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(requestBody),
  });

  if (!response.ok) {
    throw new Error("Network response was not ok");
  }

  return await response.json();
}
