import { useEffect, useRef } from 'react';
import 'asciinema-player/dist/bundle/asciinema-player.css';

// Cast plays one recorded `memdoor tui` session (an asciinema v2 file under
// web/public). A still until clicked, so a page with several of them stays
// quiet. In the docs, `![caption](/demo-x.cast)` renders one.
export function Cast({ src, poster, caption }: { src: string; poster?: string; caption?: string }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = host.current;
    if (!el) return;
    let player: { dispose(): void } | undefined;
    let cancelled = false;
    import('asciinema-player').then((ap) => {
      if (cancelled) return;
      player = ap.create(src, el, {
        autoPlay: false,
        loop: false,
        preload: false,
        idleTimeLimit: 1.2,
        speed: 1.5,
        fit: 'width',
        controls: true,
        theme: 'asciinema',
        poster: poster ?? 'npt:0:10',
      });
    });
    return () => {
      cancelled = true;
      player?.dispose();
    };
  }, [src, poster]);
  return (
    <figure className="my-6">
      <div ref={host} className="overflow-hidden rounded-xl border border-neutral-700 shadow-lg" />
      {caption ? <figcaption className="mt-2 text-center text-xs text-neutral-500">{caption}</figcaption> : null}
    </figure>
  );
}
