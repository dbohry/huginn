const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/** A short age such as "now", "12m", "3h" or "2d"; older than a month, a date. */
export function ago(unixSeconds: number, now = Date.now() / 1000): string {
  const s = Math.max(0, now - unixSeconds);
  if (s < MINUTE) return 'now';
  if (s < HOUR) return `${Math.floor(s / MINUTE)}m`;
  if (s < DAY) return `${Math.floor(s / HOUR)}h`;
  if (s < 30 * DAY) return `${Math.floor(s / DAY)}d`;
  return new Date(unixSeconds * 1000).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
}

export function when(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

/** "Updated just now", "Updated 12m ago" or a date: a label followed by how long ago something happened. */
export function since(label: string, unixSeconds: number, now = Date.now() / 1000): string {
  const age = ago(unixSeconds, now);
  if (age === 'now') return `${label} just now`;
  return /^\d+[mhd]$/.test(age) ? `${label} ${age} ago` : `${label} ${age}`;
}

/** "Updated just now", "Updated 12m ago" or "Not fetched yet", for a feed's last fetch. */
export function updated(unixSeconds: number | null, now = Date.now() / 1000): string {
  return unixSeconds === null ? 'Not fetched yet' : since('Updated', unixSeconds, now);
}
