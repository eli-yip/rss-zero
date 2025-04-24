import type { ContentTypeV2 } from "@/api/client";

interface Author {
  id: string;
  nickname: string;
}

export interface Topic {
  id: string;
  original_url: string;
  archive_url: string;
  platform: string;
  type: ContentTypeV2;
  title: string;
  created_at: string;
  body: string;
  author: Author;
  custom: Custom | null;
}

export type Custom = {
  bookmark: boolean;
  bookmark_id: string;
  like: number;
  tags: string[];
  comment: string;
  note: string;
};
