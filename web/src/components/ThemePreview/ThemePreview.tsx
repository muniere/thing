import { useLayoutEffect, useRef, useState } from "react";
import s from "./ThemePreview.module.css";

// The mock is laid out at the width a real board would be and then scaled down as
// a whole, rather than rebuilt at small sizes. Every padding, radius, and font
// size below is the value the real pane uses, so the miniature's density and
// proportions are the board's rather than an approximation of them.
const BOARD_WIDTH = 1200;
const BOARD_HEIGHT = 560;

// useScaleToFit reports the factor that fits BOARD_WIDTH into the element it is
// given, tracking resizes. The frame sizes itself by aspect ratio rather than
// from this factor: a transform does not change layout, so the unscaled board
// would otherwise claim its full height on the first pass and the frame would
// shrink once measured — which, in a vertically centered modal, looks like the
// dialog sliding down.
function useScaleToFit(ref: React.RefObject<HTMLDivElement | null>): number {
  const [scale, setScale] = useState(0);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => setScale(el.clientWidth / BOARD_WIDTH);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref]);
  return scale;
}

// ThemePreview renders a miniature of the board in the selected theme: the filter
// sidebar, the tree of railed epic cards, and a node's detail pane. It shows what
// a palette actually feels like, which a row of color chips cannot — most of the
// character is in how the surfaces, rails, and washes sit against each other.
//
// The miniature is not a description of the palette but the palette itself: it
// carries data-theme, so every value in it is read from the theme's own tokens,
// down to the rails going through the same [data-status] map the tree's rows use.
// An empty theme leaves the attribute off, which inherits the picker's tokens —
// the default palette, which is what "Default" resolves to.
//
// The text is placeholder on purpose: this is about color, and real titles would
// invite reading rather than looking.
//
// It does not load the theme's stylesheet: the dialog around it already loads the
// whole set it offers, since it colors every choice in the list too.
export function ThemePreview({ theme }: { theme: string }) {
  const frame = useRef<HTMLDivElement>(null);
  const scale = useScaleToFit(frame);

  return (
    <div
      ref={frame}
      className={s.frame}
      style={{ aspectRatio: `${BOARD_WIDTH} / ${BOARD_HEIGHT}` }}
      aria-hidden="true"
    >
      <div
        className={s.board}
        data-theme={theme || undefined}
        style={{ width: BOARD_WIDTH, height: BOARD_HEIGHT, transform: `scale(${scale})` }}
      >
        <div className={s.side}>
          <div className={s.group}>
            <span className={s.heading}>Search</span>
            <div className={s.input}>lorem, ipsum, dolor…</div>
          </div>
          <div className={s.group}>
            <span className={s.heading}>Status</span>
            {[
              ["todo", "12"],
              ["doing", "5"],
              ["done", "34"],
              ["paused", "2"],
            ].map(([name, count]) => (
              <div key={name} className={s.facet}>
                <span className={s.check} />
                <span className={s.dot} data-status={name} />
                <span className={s.facetName}>{name}</span>
                <span className={s.count}>{count}</span>
              </div>
            ))}
          </div>
          <div className={s.group}>
            <span className={s.heading}>Priority</span>
            {[
              ["high", "3", s.high],
              ["medium", "6", s.medium],
              ["low", "4", s.low],
            ].map(([name, count, tone]) => (
              <div key={name} className={s.facet}>
                <span className={s.check} />
                <span className={`${s.facetName} ${tone}`}>{name}</span>
                <span className={s.count}>{count}</span>
              </div>
            ))}
          </div>
          <div className={s.group}>
            <span className={s.heading}>Category</span>
            <div className={s.select}>all categories</div>
          </div>
        </div>

        <div className={s.treePane}>
          <div className={s.path}>/path/to/project/.thing</div>
          <div className={s.groupLabel}>Lorem</div>

          <div className={s.epicCard} data-status="doing">
            <div className={s.row}>
              <span className={s.caret}>▾</span>
              <span className={s.rowDot} data-status="doing" />
              <span className={s.epicTitle}>Lorem ipsum dolor sit amet</span>
              <span className={s.badge}>3</span>
              <span className={s.medium}>medium</span>
            </div>
            <div className={s.row}>
              <span className={s.caret}>▸</span>
              <span className={s.rowDot} data-status="doing" />
              <span className={s.rowTitle}>Consectetur adipiscing elit</span>
              <span className={s.badge}>12</span>
              <span className={s.high}>high</span>
            </div>
            <div className={s.row} data-status="done" data-selected>
              <span className={s.caret}>▸</span>
              <span className={s.rowDot} data-status="done" />
              <span className={s.rowTitle}>Sed do eiusmod tempor</span>
              <span className={s.badge}>7</span>
            </div>
          </div>

          <div className={s.epicCard} data-status="todo">
            <div className={s.row}>
              <span className={s.caret}>▾</span>
              <span className={s.rowDot} data-status="todo" />
              <span className={s.epicTitle}>Incididunt ut labore et dolore</span>
              <span className={s.badge}>2</span>
              <span className={s.low}>low</span>
            </div>
            <div className={s.row}>
              <span className={s.caret}>▸</span>
              <span className={s.rowDot} data-status="paused" />
              <span className={s.taskTitle}>magna/aliqua-ut-enim</span>
            </div>
          </div>
        </div>

        <div className={s.detailPane}>
          <div className={s.detailHead}>
            <span className={s.detailTitle}>Lorem ipsum dolor sit amet</span>
            <span className={s.edit}>edit</span>
          </div>
          <div className={s.detailRef}>lorem-ipsum/dolor-sit-amet</div>
          <div className={s.chips}>
            <span className={s.statusChip} data-status="doing">
              doing
            </span>
            <span className={s.priorityChip}>priority</span>
            <span className={s.updated}>updated 2026-01-01</span>
          </div>
          <div className={s.sectionLabel}>body</div>
          <div className={s.bodyPanel}>
            Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque
            laudantium, totam rem aperiam eaque ipsa quae ab illo inventore veritatis.
          </div>
          <div className={s.sectionLabel}>tasks · 3</div>
          {[
            ["done", "Nemo enim ipsam voluptatem"],
            ["doing", "Quia voluptas sit aspernatur"],
            ["todo", "Aut odit aut fugit sed quia"],
          ].map(([status, title]) => (
            <div key={title} className={s.childRow} data-status={status}>
              {title}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
