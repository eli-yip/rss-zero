export interface Author {
  id: string;
  nickname: string;
}

export interface Topic {
  id: string;
  original_url: string;
  archive_url: string;
  platform: string;
  title: string;
  created_at: string;
  body: string;
  author: Author;
}
