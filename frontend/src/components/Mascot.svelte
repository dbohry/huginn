<script lang="ts" module>
  // The pixel raven, drawn from the same grid as docs/mascot/generate.py.
  // X body, W wing feathers, B beak, o eye (left empty so the background shows through).
  const GRID = [
    '....XXXXXXXXX....',
    '...XXXXXXXXXXX...',
    '..XXXXXXXXXXXXX..',
    '..XXXoXXXXXoXXX..',
    '..XXXXXXBXXXXXX..',
    '..XXXXXBBBXXXXX..',
    '...XXXXBBBXXXX...',
    '....XXXXBXXXX....',
    '.XX.XXXXXXXXX.XX.',
    'XWWXXXXXXXXXXXWWX',
    'XWWWXXXXXXXXXWWWX',
    'XWWWXXXXXXXXXWWWX',
    '.WWWXXXXXXXXXWWW.',
    '..WWXXXXXXXXXWW..',
    '...WXXXXXXXXXW...',
    '....XXXXXXXXX....',
    '.....XX...XX.....',
    '....XXX...XXX....',
  ];
  const WING_ROW = 8;
  const BODY_L = 4;
  const BODY_R = 12;
  const LEG_ROW = 16;

  type Cell = { x: number; y: number; c: string };
  const parts: Record<'upper' | 'wingL' | 'wingR' | 'legs' | 'eyes', Cell[]> = {
    upper: [],
    wingL: [],
    wingR: [],
    legs: [],
    eyes: [],
  };
  GRID.forEach((row, y) =>
    [...row].forEach((c, x) => {
      if (c === '.') return;
      const cell = { x, y, c };
      if (c === 'o') parts.eyes.push(cell);
      else if (y >= LEG_ROW) parts.legs.push(cell);
      else if (y >= WING_ROW && x < BODY_L) parts.wingL.push(cell);
      else if (y >= WING_ROW && x > BODY_R) parts.wingR.push(cell);
      else parts.upper.push(cell);
    }),
  );
  // Room above for the hop and at the sides for the wing flaps.
  const VIEW = `-1 -3 ${GRID[0].length + 2} ${GRID.length + 3}`;
  const RATIO = GRID.length + 3;
  const WIDTH = GRID[0].length + 2;
</script>

<script lang="ts">
  // One 5 s loop: breathes, blinks twice, then hops and flaps. Movements are whole pixels.
  // Reduced motion is handled globally in theme.css, which leaves the raven standing still.
  let { size = 48 }: { size?: number } = $props();
</script>

{#snippet cells(list: Cell[])}
  {#each list as { x, y, c }, i (i)}<rect {x} {y} width="1" height="1" class={c} />{/each}
{/snippet}

<svg
  class="mascot"
  width={(size * WIDTH) / RATIO}
  height={size}
  viewBox={VIEW}
  shape-rendering="crispEdges"
  aria-hidden="true"
>
  <g class="hop">
    <g class="upper">
      {@render cells(parts.upper)}
      <g class="lids">
        {#each parts.eyes as { x, y }, i (i)}<rect {x} {y} width="1" height="1" class="X" />{/each}
      </g>
      <g class="wing-l">{@render cells(parts.wingL)}</g>
      <g class="wing-r">{@render cells(parts.wingR)}</g>
    </g>
    {@render cells(parts.legs)}
  </g>
</svg>

<style>
  .mascot {
    display: block;
    overflow: visible;
  }
  .X {
    fill: var(--accent);
  }
  .W {
    fill: var(--raven-wing);
  }
  .B {
    fill: var(--raven-beak);
  }
  g {
    animation-duration: 5s;
    animation-iteration-count: infinite;
    animation-timing-function: step-end;
  }
  .hop {
    animation-name: hop;
  }
  .upper {
    animation-name: breathe;
  }
  .lids {
    opacity: 0;
    animation-name: blink;
  }
  .wing-l {
    animation-name: flap-l;
  }
  .wing-r {
    animation-name: flap-r;
  }
  @keyframes breathe {
    0%, 20%, 40%, 100% { transform: none; }
    10%, 30% { transform: translateY(1px); }
  }
  @keyframes blink {
    0%, 46%, 49%, 100% { opacity: 0; }
    44%, 47.5% { opacity: 1; }
  }
  @keyframes hop {
    0%, 72%, 100% { transform: none; }
    60%, 70% { transform: translateY(-1px); }
    62%, 68% { transform: translateY(-2px); }
  }
  @keyframes flap-l {
    0%, 58%, 64%, 70%, 100% { transform: none; }
    61%, 67% { transform: translate(-1px, -1px); }
  }
  @keyframes flap-r {
    0%, 58%, 64%, 70%, 100% { transform: none; }
    61%, 67% { transform: translate(1px, -1px); }
  }
</style>
