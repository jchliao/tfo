'use client';

import React, { useEffect } from 'react';
import { X } from 'lucide-react';
import { resolveImageSrc } from '@/lib/images';

interface ImageLightboxProps {
  src: string;
  alt?: string;
  onClose: () => void;
}

/** 点击正文图片后的大图预览层，支持 Esc / 点击遮罩关闭。 */
export function ImageLightbox({ src, alt, onClose }: ImageLightboxProps) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-6 backdrop-blur-sm"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <button
        type="button"
        onClick={onClose}
        className="absolute right-5 top-5 rounded-full bg-white/10 p-2 text-white transition-colors hover:bg-white/20"
        aria-label="Close"
      >
        <X size={18} />
      </button>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={resolveImageSrc(src)}
        alt={alt ?? ''}
        className="max-h-full max-w-full rounded-lg shadow-2xl"
        onClick={event => event.stopPropagation()}
      />
    </div>
  );
}
