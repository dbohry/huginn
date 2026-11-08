export type User = {
  id: number;
  username: string;
  isAdmin: boolean;
  mustChangePassword: boolean;
  starterDone: boolean;
};

export type Feed = {
  id: number;
  url: string;
  title: string;
  siteUrl: string;
  unread: number;
  /** Unix seconds, or null if the feed has not been fetched yet. */
  lastFetchedAt: number | null;
  error: string;
  fullText: boolean;
  category: string;
  /** "fetching" while the server fetches the feed, "queued" while it waits its turn in a refresh of every feed. */
  refresh: '' | 'queued' | 'fetching';
};

export type StarterFeed = { id: number; title: string; url: string; followed: boolean };
export type StarterTopic = { name: string; feeds: StarterFeed[] };

export type Item = {
  id: number;
  feedId: number;
  feedTitle: string;
  title: string;
  /** The address of the original article, or "" if the feed gave none. */
  url: string;
  author: string;
  publishedAt: number;
  snippet: string;
  unread: boolean;
};

/** An article opened for reading. `content` is sanitized HTML. */
export type ItemDetail = Item & {
  content: string;
  /** True when `content` is the full article loaded from its own page, not what the feed gave. */
  fullText: boolean;
  summaryOnly: boolean;
};

export type UserSummary = {
  id: number;
  username: string;
  isAdmin: boolean;
  mustChangePassword: boolean;
  /** Unix seconds, or null if the user has never signed in. */
  lastSeenAt: number | null;
};

export type BlockedWord = { id: number; word: string };

export type ImportResult = { added: number; existing: number; invalid: number };

/** An error response from the server. `message` is safe to show to the user. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly code?: string,
  ) {
    super(message);
  }
}

let onUnauthorized: () => void = () => {};

/** Called when a request finds that the session is gone (expired or signed out elsewhere). */
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

// Sign-in, setup and the first "who am I" check expect to be told "not signed in".
const expectsUnauthorized = (path: string) =>
  path === '/api/me' || path === '/api/login' || path === '/api/setup';

type Body = { json: unknown } | { xml: string };

async function request<T>(method: string, path: string, body?: Body): Promise<T> {
  let init: RequestInit = { method, credentials: 'same-origin' };
  if (body && 'json' in body) {
    init = { ...init, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body.json) };
  } else if (body) {
    init = { ...init, headers: { 'Content-Type': 'text/xml' }, body: body.xml };
  }

  let res: Response;
  try {
    res = await fetch(path, init);
  } catch {
    throw new ApiError(0, 'Cannot reach the server.');
  }

  if (res.status === 204) return undefined as T;

  const data = await res.json().catch(() => ({}));
  if (res.status === 401 && !expectsUnauthorized(path)) onUnauthorized();
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? 'Something went wrong.', data.code);
  }
  return data as T;
}

const json = (value: unknown): Body => ({ json: value });

export type ItemQuery = { feedId?: number | null; unreadOnly?: boolean; after?: string; limit?: number };

export const api = {
  health: () => request<{ status: string; version: string }>('GET', '/healthz'),
  setupStatus: () => request<{ needsSetup: boolean }>('GET', '/api/setup/status'),
  setup: (username: string, password: string) =>
    request<{ user: User }>('POST', '/api/setup', json({ username, password })),
  login: (username: string, password: string, remember: boolean) =>
    request<{ user: User }>('POST', '/api/login', json({ username, password, remember })),
  logout: () => request<void>('POST', '/api/logout'),
  me: () => request<{ user: User }>('GET', '/api/me'),
  changePassword: (current: string, next: string) =>
    request<void>('POST', '/api/me/password', json({ current, new: next })),

  users: () => request<{ users: UserSummary[] }>('GET', '/api/users'),
  createUser: (username: string, isAdmin: boolean) =>
    request<{ user: UserSummary; temporaryPassword: string }>('POST', '/api/users', json({ username, isAdmin })),
  resetUserPassword: (id: number) =>
    request<{ temporaryPassword: string }>('POST', `/api/users/${id}/reset-password`),
  deleteUser: (id: number) => request<void>('DELETE', `/api/users/${id}`),

  feeds: () => request<{ feeds: Feed[] }>('GET', '/api/feeds'),
  addFeed: (url: string) => request<{ feed: Feed }>('POST', '/api/feeds', json({ url })),
  deleteFeed: (id: number) => request<void>('DELETE', `/api/feeds/${id}`),
  setFeedFullText: (id: number, on: boolean) => request<{ feed: Feed }>('PATCH', `/api/feeds/${id}`, json({ fullText: on })),
  setFeedCategory: (id: number, category: string) => request<{ feed: Feed }>('PATCH', `/api/feeds/${id}`, json({ category })),
  refreshFeed: (id: number) => request<void>('POST', `/api/feeds/${id}/refresh`),
  /** Starts fetching every feed in the background. */
  refreshAll: () => request<{ feeds: number }>('POST', '/api/refresh'),

  items: (q: ItemQuery = {}) => {
    const p = new URLSearchParams();
    if (q.feedId) p.set('feed', String(q.feedId));
    if (q.unreadOnly) p.set('unread', '1');
    if (q.after) p.set('after', q.after);
    if (q.limit) p.set('limit', String(q.limit));
    const qs = p.toString();
    return request<{ items: Item[]; next: string }>('GET', `/api/items${qs ? `?${qs}` : ''}`);
  },
  item: (id: number) => request<{ item: ItemDetail }>('GET', `/api/items/${id}`),
  /** Fetches the article's own page and keeps the readable text. */
  loadFullText: (id: number) => request<{ item: ItemDetail }>('POST', `/api/items/${id}/full`),
  unloadFullText: (id: number) => request<{ item: ItemDetail }>('DELETE', `/api/items/${id}/full`),
  setRead: (id: number, read: boolean) => request<void>('PUT', `/api/items/${id}/read`, json({ read })),
  readAll: (feedId?: number | null) =>
    request<{ marked: number }>('POST', '/api/read-all', json(feedId ? { feedId } : {})),

  blockedWords: () => request<{ words: BlockedWord[] }>('GET', '/api/blocked-words'),
  addBlockedWord: (word: string) => request<{ word: BlockedWord }>('POST', '/api/blocked-words', json({ word })),
  deleteBlockedWord: (id: number) => request<void>('DELETE', `/api/blocked-words/${id}`),

  importOpml: (xml: string) => request<ImportResult>('POST', '/api/opml', { xml }),

  starter: () => request<{ topics: StarterTopic[] }>('GET', '/api/starter'),
  subscribeStarter: (ids: number[]) => request<{ added: number; existing: number }>('POST', '/api/starter/subscribe', json({ ids })),
  dismissStarter: () => request<void>('POST', '/api/starter/dismiss'),
  // Administrators edit the list.
  addStarterFeed: (category: string, title: string, url: string) =>
    request<{ feed: StarterFeed }>('POST', '/api/starter/feeds', json({ category, title, url })),
  deleteStarterFeed: (id: number) => request<void>('DELETE', `/api/starter/feeds/${id}`),
  resetStarter: () => request<void>('POST', '/api/starter/reset'),
  importStarterOpml: (xml: string) => request<{ feeds: number; skipped: number }>('POST', '/api/starter/opml', { xml }),
};
