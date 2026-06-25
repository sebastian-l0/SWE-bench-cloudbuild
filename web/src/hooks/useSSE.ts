import { useEffect, useRef } from 'react';

// useSSE subscribes to a Server-Sent Events endpoint and invokes onMessage for
// every event. The browser's EventSource auto-reconnects on transient drops; on
// reconnect the server resumes from Last-Event-ID. The latest onMessage is kept
// in a ref so the subscription is not torn down on each render.
export function useSSE(url: string | undefined, onMessage: (event: MessageEvent) => void) {
  const handler = useRef(onMessage);
  handler.current = onMessage;

  useEffect(() => {
    if (!url) return;
    const source = new EventSource(url);
    const listener = (event: MessageEvent) => handler.current(event);
    source.onmessage = listener;
    // Named events from the backend (run_phase, image_status, ...).
    for (const type of ['run_phase', 'run_status', 'image_status']) {
      source.addEventListener(type, listener as EventListener);
    }
    return () => source.close();
  }, [url]);
}
