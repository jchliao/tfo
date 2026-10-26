// 图片地址解析：笔记正文里保存的是相对笔记文件的路径（如 ../attachments/2026-10/xxx.png），
// 这样用其他 Markdown 编辑器直接打开数据目录时图片也能正常显示；
// 在 Web 界面里渲染时需要把这些相对路径映射到后端的 /api/attachments/ 路由。

const EXTERNAL_SRC = /^(?:[a-z][a-z0-9+.-]*:|\/\/|\/)/i;

/**
 * 把 Markdown 中的图片地址转换为浏览器可访问的地址。
 * - http(s):// 、data: 、blob: 、以 / 开头的地址原样返回
 * - ../attachments/... 、attachments/... 映射为 /api/attachments/...
 * - 其他相对路径原样返回（交给浏览器按当前页面解析）
 */
export function resolveImageSrc(src?: string | null): string {
  const value = (src ?? '').trim();
  if (!value) return '';
  if (EXTERNAL_SRC.test(value)) return value;

  const stripped = value.replace(/^(?:\.\.?\/)+/, '');
  if (stripped.startsWith('attachments/')) {
    return `/api/${stripped}`;
  }
  return value;
}

/** 是否是本应用内的附件（可用于判断要不要显示"已在本地"之类的提示） */
export function isLocalAttachment(src?: string | null): boolean {
  const value = (src ?? '').trim();
  if (!value) return false;
  return /^(?:\.\.\/)*attachments\//.test(value) || value.startsWith('/api/attachments/');
}
