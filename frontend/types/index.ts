export interface Fragment {
  id: string;
  title: string;
  content: string;
  date: string;
  tags: string[];
  highlights?: { start: number; end: number }[];
  filePath?: string;
  /** 正文引用的图片（Markdown 原文路径，如 ../attachments/2026-10/x.png） */
  images?: string[];
}
