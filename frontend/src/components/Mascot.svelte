<script lang="ts" module>
  // The pixel raven, drawn as one SVG rect per pixel. It has two poses that share one frame:
  // side-on, facing left toward whatever it is saying, and front-on, for when it turns to look at you.
  // X body, W wing feathers (the blue sheen), B beak, F feet, o eye, h the glint in the eye.
  const SIDE = [
    '........XXXXX...........',
    '......XXXXXXXXX.........',
    '.....XXXXXXXXXXX........',
    '..BBBXhoXXXXXXXXX.......',
    '.BBBBBooXXXXXXXXXX......',
    'BBBBBBBXXXXXXXXXXXW.....',
    'B..BBBWXXXXWWWWWWWWW....',
    '....WXWXWXWWWWWWWWWWW...',
    '....XWXWXXWWWWWWWWWWWWW.',
    '.....XXXXXXWWWWWWWWWWWWW',
    '.....XXXXXXXWWWWWWWWWWWW',
    '......XXXXXXXXWWWWWWWWW.',
    '.......XXXXXXX....WWWWWW',
    '.........F..F......WWWW.',
    '.........F..F...........',
    '........FF.FF...........',
  ];
  const FRONT = [
    '....XXXXXXXXX....',
    '...XXXXXXXXXXX...',
    '..XXXXXXXXXXXXX..',
    '..XXhoXXXXXhoXX..',
    '..XXooXBBBXooXX..',
    '..XXXXBBBBBXXXX..',
    '...XXXBBBBBXXX...',
    '...XXXXBBBXXXX...',
    '.XX.WXWXBXWXW.XX.',
    'XWWXXWXWXWXWXXWWX',
    'XWWWXXWXXXWXXWWWX',
    'XWWWXXXXXXXXXWWWX',
    '.WWWXXXXXXXXXWWW.',
    '..WWXXXXXXXXXWW..',
    '...WXXXXXXXXXW...',
    '....XWWWWWWWX....',
    '.....FWWWWWF.....',
    '....FFFWWWFFF....',
  ];

  type Cell = { x: number; y: number; c: string };
  type Parts = Record<string, Cell[]>;

  // Splits a pose into the groups that move on their own. `pick` names a cell's group, or the body.
  function split(grid: string[], dx: number, dy: number, pick: (x: number, y: number, c: string) => string): Parts {
    const parts: Parts = {};
    grid.forEach((row, y) =>
      [...row].forEach((c, x) => {
        if (c === '.') return;
        (parts[pick(x, y, c)] ??= []).push({ x: x + dx, y: y + dy, c });
      }),
    );
    for (const k of ['body', 'eyes', 'wing', 'tail', 'wingL', 'wingR', 'legs']) parts[k] ??= [];
    return parts;
  }

  // Both poses stand on the same line; the front-on one is centred over the side-on one's feet.
  const W = SIDE[0].length;
  const H = Math.max(SIDE.length, FRONT.length);
  const side = split(SIDE, 0, H - SIDE.length, (x, y, c) => {
    if (c === 'o' || c === 'h') return 'eyes';
    if (c === 'F') return 'legs';
    if (c === 'W' && (x >= 20 || y >= 12)) return 'tail';
    if (c === 'W' && x >= 10) return 'wing';
    return 'body';
  });
  const front = split(FRONT, 3, H - FRONT.length, (x, y, c) => {
    if (c === 'o' || c === 'h') return 'eyes';
    if (y >= 16) return 'legs';
    if (y >= 8 && x < 4) return 'wingL';
    if (y >= 8 && x > 12) return 'wingR';
    return 'body';
  });

  // Room above for the hop and at the sides for the wing flaps.
  const VIEW = `-1 -3 ${W + 2} ${H + 3}`;
  const RATIO = H + 3;
  const WIDTH = W + 2;
</script>

<script lang="ts">
  // One 10 s loop, in whole pixels: breathes and blinks side-on, hops with a wing beat, flicks
  // its tail, then turns to look at you, blinks, flaps, and turns back.
  // Reduced motion is handled globally in theme.css, which leaves the raven standing side-on.
  let { size = 48 }: { size?: number } = $props();
</script>

{#snippet cells(list: Cell[])}
  {#each list as { x, y, c }, i (i)}<rect {x} {y} width="1" height="1" class={c} />{/each}
{/snippet}

{#snippet lids(list: Cell[])}
  <g class="lids">
    {#each list as { x, y }, i (i)}<rect {x} {y} width="1" height="1" class="X" />{/each}
  </g>
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
    <g class="side">
      <g class="upper">
        {@render cells(side.body)}
        {@render cells(side.eyes)}
        {@render lids(side.eyes)}
        <g class="wing">{@render cells(side.wing)}</g>
        <g class="tail">{@render cells(side.tail)}</g>
      </g>
      {@render cells(side.legs)}
    </g>
    <g class="front">
      <g class="upper">
        {@render cells(front.body)}
        {@render cells(front.eyes)}
        {@render lids(front.eyes)}
        <g class="wing-l">{@render cells(front.wingL)}</g>
        <g class="wing-r">{@render cells(front.wingR)}</g>
      </g>
      {@render cells(front.legs)}
    </g>
  </g>
</svg>

<style>
  .mascot {
    display: block;
    overflow: visible;
  }
  .X {
    fill: var(--raven-body);
  }
  .W {
    fill: var(--raven-wing);
  }
  .B,
  .F {
    fill: var(--raven-beak);
  }
  .o {
    fill: var(--raven-eye);
  }
  .h {
    fill: var(--raven-glint);
  }
  g {
    animation-duration: 10s;
    animation-iteration-count: infinite;
    animation-timing-function: step-end;
  }
  .lids {
    opacity: 0;
  }
  /* Without animation (reduced motion) only the side-on pose shows. */
  .front {
    opacity: 0;
    animation-name: show-front;
  }
  .side {
    animation-name: show-side;
  }
  .hop {
    animation-name: hop;
  }
  .side .upper {
    animation-name: side-breathe;
  }
  .side .lids {
    animation-name: side-blink;
  }
  .side .wing {
    animation-name: wing-beat;
  }
  .side .tail {
    animation-name: tail-flick;
  }
  .front .upper {
    animation-name: front-breathe;
  }
  .front .lids {
    animation-name: front-blink;
  }
  .wing-l {
    animation-name: flap-l;
  }
  .wing-r {
    animation-name: flap-r;
  }

  /* Side-on: breathes, blinks twice, hops with two wing beats, flicks its tail. */
  @keyframes side-breathe {
    0%, 8%, 16%, 100% { transform: none; }
    4%, 12% { transform: translateY(1px); }
  }
  @keyframes side-blink {
    0%, 23%, 26%, 100% { opacity: 0; }
    22%, 25% { opacity: 1; }
  }
  @keyframes hop {
    0%, 42%, 100% { transform: none; }
    34%, 40% { transform: translateY(-1px); }
    36%, 38% { transform: translateY(-2px); }
  }
  @keyframes wing-beat {
    0%, 37%, 41%, 100% { transform: none; }
    35%, 39% { transform: translateY(-1px); }
  }
  @keyframes tail-flick {
    0%, 47%, 89%, 100% { transform: none; }
    45%, 87% { transform: translateY(-1px); }
  }

  /* The turn: front-on from 54% to 74% of the loop. */
  @keyframes show-side {
    0%, 74%, 100% { opacity: 1; }
    54% { opacity: 0; }
  }
  @keyframes show-front {
    0%, 74%, 100% { opacity: 0; }
    54% { opacity: 1; }
  }

  /* Front-on: breathes, blinks, flaps once. */
  @keyframes front-breathe {
    0%, 60%, 100% { transform: none; }
    58% { transform: translateY(1px); }
  }
  @keyframes front-blink {
    0%, 63%, 100% { opacity: 0; }
    62% { opacity: 1; }
  }
  @keyframes flap-l {
    0%, 67%, 69%, 71%, 100% { transform: none; }
    66%, 70% { transform: translate(-1px, -1px); }
  }
  @keyframes flap-r {
    0%, 67%, 69%, 71%, 100% { transform: none; }
    66%, 70% { transform: translate(1px, -1px); }
  }
</style>
