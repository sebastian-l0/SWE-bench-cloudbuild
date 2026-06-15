import { useEffect } from 'react';

export function useSSE(url: string | undefined, onMessage: (event: MessageEvent) => void) {
  useEffect(() => {
    if (!url) return;
    const source = new EventSource(url);
    source.onmessage = onMessage;
    return () => source.close();
  }, [url, onMessage]);
}
