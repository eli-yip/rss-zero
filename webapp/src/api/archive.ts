import { apiUrl } from "@/config/config";
import type { Topic } from "@/types/topic";

export enum ContentType {
  Answer = "answer",
  Pin = "pin",
  Article = "article",
}

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
  type: ContentType;
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

export async function fetchArchiveTopics(
  page: number,
  start_date = "",
  end_date = "",
  type: ContentType = ContentType.Answer,
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
