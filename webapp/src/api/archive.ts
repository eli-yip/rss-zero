import { apiUrl } from "@/config/config";
import { Topic } from "@/types/topic";

export enum ContentType {
  Answer = "answer",
  Pin = "pin",
  Article = "article",
}

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
  type: ContentType;
  /**
   * 开始日期
   */
  start_date: string;
  /**
   * 结束日期
   */
  end_date: string;
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

export interface ArchiveResponse {
  count: number;
  paging: ArchivePaging;
  topics: Topic[];
}

// 请求接口函数
export async function fetchArchiveTopics(
  page: number,
  start_date: string = "",
  end_date: string = "",
  type: ContentType = ContentType.Answer,
): Promise<ArchiveResponse> {
  const requestBody: ArchiveRequest = {
    platform: "zhihu",
    type: type,
    count: 10,
    page: page,
    author: "canglimo",
    start_date: start_date,
    end_date: end_date,
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
