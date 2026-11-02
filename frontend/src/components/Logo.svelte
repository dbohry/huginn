<script lang="ts">
  let { size = 32 }: { size?: number } = $props();
</script>

<!-- The faceted raven. The tile and the eye take the accent colour; the bird takes the text-on-accent colour. -->
<svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
  <rect width="64" height="64" rx="16" fill="var(--accent)" />
  <g transform="translate(6.4 6.4) scale(0.8)">
    <polygon points="56,30 40,22 28,12 12,22 8,40 22,54 34,44 40,36" fill="var(--on-accent)" />
    <polygon points="28,12 40,22 34,30 18,26" fill="var(--accent)" fill-opacity="0.3" />
    <circle cx="30" cy="27" r="2.6" fill="var(--accent)" />
  </g>
</svg>
