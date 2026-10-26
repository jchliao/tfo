'use client';

import { useCallback, useRef, useState } from 'react';
import { MAX_IMAGE_SIZE, uploadImage } from '@/lib/api';

export type ImageUploadErrorReason = 'type' | 'size' | 'upload';

export interface UseImageUploadOptions {
  /** 目标文本框，图片 Markdown 会插入到光标处 */
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  /** 读取文本框当前内容 */
  getValue: () => string;
  /** 写回文本框内容 */
  setValue: (value: string) => void;
  onError?: (reason: ImageUploadErrorReason) => void;
}

export interface UseImageUploadResult {
  /** 是否有图片正在上传 */
  uploading: boolean;
  /** 是否有图片正被拖到文本框上方（用于高亮提示） */
  dragging: boolean;
  /** 打开系统文件选择框 */
  openFilePicker: () => void;
  /** 隐藏的 <input type="file">，由调用方渲染 */
  fileInputProps: {
    ref: React.RefObject<HTMLInputElement | null>;
    type: 'file';
    accept: string;
    multiple: boolean;
    hidden: true;
    onChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
  };
  handlePaste: (event: React.ClipboardEvent<HTMLElement>) => void;
  handleDragOver: (event: React.DragEvent<HTMLElement>) => void;
  handleDragLeave: (event: React.DragEvent<HTMLElement>) => void;
  handleDrop: (event: React.DragEvent<HTMLElement>) => void;
}

/** 从 clipboard / drag 事件里挑出图片文件 */
function pickImageFiles(list: FileList | null | undefined): File[] {
  if (!list) return [];
  return Array.from(list).filter(file => file.type.startsWith('image/'));
}

/**
 * 把 Markdown 片段插入到文本框光标处，并自动补齐换行，避免图片和文字挤在一行。
 */
function insertAtCursor(
  el: HTMLTextAreaElement | null,
  current: string,
  snippet: string,
): { value: string; caret: number } {
  if (!el) {
    const value = current ? `${current.replace(/\s+$/, '')}\n${snippet}` : snippet;
    return { value, caret: value.length };
  }

  const start = el.selectionStart ?? current.length;
  const end = el.selectionEnd ?? start;
  const before = current.slice(0, start);
  const after = current.slice(end);

  const leading = before.length > 0 && !before.endsWith('\n') ? '\n' : '';
  const trailing = after.length > 0 && !after.startsWith('\n') ? '\n' : '';
  const inserted = `${leading}${snippet}${trailing}`;

  return { value: `${before}${inserted}${after}`, caret: before.length + inserted.length };
}

/**
 * 统一处理"粘贴图片 / 拖拽图片 / 选择文件"三种上传入口。
 */
export function useImageUpload({ textareaRef, getValue, setValue, onError }: UseImageUploadOptions): UseImageUploadResult {
  const [uploading, setUploading] = useState(false);
  const [dragging, setDragging] = useState(false);
  const pendingRef = useRef(0);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const insertText = useCallback((snippet: string) => {
    const el = textareaRef.current;
    const { value, caret } = insertAtCursor(el, getValue(), snippet);
    setValue(value);

    if (el) {
      requestAnimationFrame(() => {
        el.focus();
        el.setSelectionRange(caret, caret);
      });
    }
  }, [getValue, setValue, textareaRef]);

  const handleFiles = useCallback(async (files: File[]) => {
    if (files.length === 0) return;

    if (files.some(file => file.size > MAX_IMAGE_SIZE)) {
      onError?.('size');
      return;
    }

    pendingRef.current += files.length;
    setUploading(true);
    try {
      const snippets: string[] = [];
      // 串行上传，保证正文里图片顺序与选择顺序一致
      for (const file of files) {
        const uploaded = await uploadImage(file);
        snippets.push(uploaded.markdown);
      }
      insertText(snippets.join('\n'));
    } catch (err) {
      console.error('Image upload failed:', err);
      onError?.('upload');
    } finally {
      pendingRef.current = Math.max(0, pendingRef.current - files.length);
      if (pendingRef.current === 0) setUploading(false);
    }
  }, [insertText, onError]);

  const handlePaste = useCallback((event: React.ClipboardEvent<HTMLElement>) => {
    const images = pickImageFiles(event.clipboardData?.files);
    if (images.length === 0) return;
    event.preventDefault();
    void handleFiles(images);
  }, [handleFiles]);

  const handleDragOver = useCallback((event: React.DragEvent<HTMLElement>) => {
    if (!Array.from(event.dataTransfer?.types ?? []).includes('Files')) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'copy';
    setDragging(true);
  }, []);

  const handleDragLeave = useCallback(() => setDragging(false), []);

  const handleDrop = useCallback((event: React.DragEvent<HTMLElement>) => {
    const images = pickImageFiles(event.dataTransfer?.files);
    if (images.length === 0) {
      setDragging(false);
      return;
    }
    event.preventDefault();
    setDragging(false);
    void handleFiles(images);
  }, [handleFiles]);

  const openFilePicker = useCallback(() => fileInputRef.current?.click(), []);

  const onFileInputChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const images = pickImageFiles(event.target.files);
    event.target.value = '';
    void handleFiles(images);
  }, [handleFiles]);

  return {
    uploading,
    dragging,
    openFilePicker,
    fileInputProps: {
      ref: fileInputRef,
      type: 'file',
      accept: 'image/png,image/jpeg,image/gif,image/webp,image/bmp',
      multiple: true,
      hidden: true,
      onChange: onFileInputChange,
    },
    handlePaste,
    handleDragOver,
    handleDragLeave,
    handleDrop,
  };
}
