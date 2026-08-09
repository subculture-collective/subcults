import { useEffect } from 'react';

const defaultDescription = 'Find underground shows, tour stops, festivals, artists, and local scenes without sharing your exact location.';

export function PageMeta({ title, description = defaultDescription }: { title: string; description?: string }) {
  useEffect(() => {
    document.title = `${title} | Subcult`;
    let meta = document.querySelector<HTMLMetaElement>('meta[name="description"]');
    if (!meta) {
      meta = document.createElement('meta');
      meta.name = 'description';
      document.head.append(meta);
    }
    meta.content = description;
  }, [title, description]);
  return null;
}
