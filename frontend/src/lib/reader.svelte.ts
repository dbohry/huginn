import { api, ApiError, type Feed, type Item, type ItemDetail } from './api';
import { ui } from './ui.svelte';

/** On a narrow screen only one pane shows at a time. */
export type Pane = 'feeds' | 'list' | 'article';

/** A short message that appears for a few seconds after something happens. */
export type Toast = { id: number; text: string; kind: 'info' | 'error' };

const message = (e: unknown) => (e instanceof ApiError ? e.message : 'Something went wrong.');
const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/** Everything the reading screen shows and does. */
class Reader {
  feeds = $state<Feed[]>([]);
  items = $state<Item[]>([]);
  next = $state('');
  /** The feed being read, or null for every feed. */
  feedId = $state<number | null>(null);
  unreadOnly = $state(false);
  selectedId = $state<number | null>(null);
  selected = $state<ItemDetail | null>(null);
  pane = $state<Pane>('list');
  /** The "Add a feed" dialog. It lives here so the first-run screen can open it too. */
  addOpen = $state(false);
  /** The starter feeds picker. It opens by itself at a new user's first sign-in, and from the sidebar. */
  starterOpen = $state(false);

  loadingList = $state(false);
  loadingMore = $state(false);
  loadingItem = $state(false);
  /** The full article is being fetched from the article's own page. */
  loadingFull = $state(false);
  refreshing = $state(false);
  /** The id of the one feed being refreshed, if any. */
  refreshingFeedId = $state<number | null>(null);
  /** Why the list could not be loaded, or "". Offered with a Retry button. */
  listError = $state('');
  toast = $state<Toast | null>(null);
  /** Said aloud by screen readers. */
  announcement = $state('');

  totalUnread = $derived(this.feeds.reduce((n, f) => n + f.unread, 0));
  currentFeed = $derived(this.feeds.find((f) => f.id === this.feedId) ?? null);
  title = $derived(this.currentFeed ? this.currentFeed.title : 'All items');
  selectedIndex = $derived(this.items.findIndex((i) => i.id === this.selectedId));
  canGoPrevious = $derived(this.selectedIndex > 0);
  canGoNext = $derived(this.selectedIndex >= 0 && (this.selectedIndex < this.items.length - 1 || this.next !== ''));

  // Bumped by every list or article load, so a slow answer to an old request is ignored.
  private listToken = 0;
  private itemToken = 0;
  // Bumped by reset(), so work started for a user who has since signed out stops.
  private generation = 0;
  private toastId = 0;
  private toastTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    // The phone's Back button, and a swipe back, move between the panes.
    if (typeof window !== 'undefined') {
      addEventListener('popstate', (e) => {
        this.pane = (e.state as { pane?: Pane } | null)?.pane ?? 'list';
      });
    }
  }

  async init() {
    // After a reload the browser may still hold a pane from an earlier visit.
    if (!this.selected && (history.state as { pane?: Pane } | null)?.pane) {
      history.replaceState(null, '', location.pathname);
      this.pane = 'list';
    }
    await Promise.all([this.loadFeeds(), this.loadItems()]);
  }

  /** Forgets everything. Called when the user signs out, so the next person sees none of it. */
  reset() {
    this.generation++;
    this.listToken++;
    this.itemToken++;
    clearTimeout(this.toastTimer);
    this.feeds = [];
    this.items = [];
    this.next = '';
    this.feedId = null;
    this.unreadOnly = false;
    this.selectedId = null;
    this.selected = null;
    this.pane = 'list';
    this.addOpen = false;
    this.starterOpen = false;
    this.loadingList = this.loadingMore = this.loadingItem = this.loadingFull = this.refreshing = false;
    this.refreshingFeedId = null;
    this.listError = '';
    this.toast = null;
    this.announcement = '';
  }

  // --- messages ---

  notify(text: string, kind: Toast['kind'] = 'info') {
    clearTimeout(this.toastTimer);
    this.toast = { id: ++this.toastId, text, kind };
    this.toastTimer = setTimeout(() => (this.toast = null), kind === 'error' ? 7000 : 4000);
  }

  dismissToast() {
    clearTimeout(this.toastTimer);
    this.toast = null;
  }

  // --- loading ---

  async loadFeeds() {
    try {
      this.feeds = (await api.feeds()).feeds;
    } catch (e) {
      this.listError = message(e);
    }
  }

  /** Starts the list again from the newest article. */
  async loadItems() {
    const token = ++this.listToken;
    this.loadingList = true;
    try {
      const page = await api.items({ feedId: this.feedId, unreadOnly: this.unreadOnly });
      if (token !== this.listToken) return;
      this.items = page.items;
      this.next = page.next;
      this.listError = '';
    } catch (e) {
      if (token === this.listToken) this.listError = message(e);
    } finally {
      if (token === this.listToken) this.loadingList = false;
    }
  }

  async retry() {
    this.listError = '';
    await Promise.all([this.loadFeeds(), this.loadItems()]);
  }

  async more() {
    if (!this.next || this.loadingMore) return;
    const token = this.listToken;
    this.loadingMore = true;
    try {
      const page = await api.items({ feedId: this.feedId, unreadOnly: this.unreadOnly, after: this.next });
      if (token !== this.listToken) return;
      this.items = [...this.items, ...page.items];
      this.next = page.next;
    } catch (e) {
      this.notify(message(e), 'error');
    } finally {
      this.loadingMore = false;
    }
  }

  // --- moving around ---

  private pushPane(pane: Pane) {
    this.pane = pane;
    if (ui.narrow) history.pushState({ pane }, '', location.pathname);
  }

  /** Shows the list. On a phone this goes back a step, so the Back button and this agree. */
  goList() {
    if (ui.narrow && (history.state as { pane?: Pane } | null)?.pane) history.back();
    else this.pane = 'list';
  }

  showFeeds() {
    this.pushPane('feeds');
  }

  async chooseFeed(id: number | null) {
    this.feedId = id;
    this.goList();
    await this.loadItems();
  }

  async setUnreadOnly(value: boolean) {
    this.unreadOnly = value;
    await this.loadItems();
  }

  /** Opens an article and marks it read. */
  async open(item: Item) {
    const token = ++this.itemToken;
    this.selectedId = item.id;
    if (this.pane !== 'article') this.pushPane('article');
    this.loadingItem = true;
    try {
      const { item: detail } = await api.item(item.id);
      if (token !== this.itemToken) return;
      this.selected = detail;
      // A feed set to load full articles does it as soon as one is opened.
      if (this.feeds.find((f) => f.id === detail.feedId)?.fullText) void this.loadFull();
      if (item.unread) await this.setRead(item.id, true);
    } catch (e) {
      if (token === this.itemToken) this.notify(message(e), 'error');
    } finally {
      if (token === this.itemToken) this.loadingItem = false;
    }
  }

  /** Loads the open article's full text from its own page, once. A failure is shown and the summary stays. */
  async loadFull() {
    const current = this.selected;
    if (!current || current.fullText || !current.url || this.loadingFull) return;
    const id = current.id;
    const token = this.itemToken;
    this.loadingFull = true;
    this.announcement = 'Loading the full article';
    try {
      const { item: detail } = await api.loadFullText(id);
      // Only if the same article is still open, and keep a read or unread change made meanwhile.
      if (token === this.itemToken && this.selected?.id === id) {
        this.selected = { ...detail, unread: this.selected.unread };
        this.announcement = 'The full article is loaded';
      }
    } catch (e) {
      if (this.selected?.id === id) this.notify(message(e), 'error');
    } finally {
      this.loadingFull = false;
    }
  }

  /** Turns loading full articles automatically on or off for a feed. */
  async setFeedFullText(id: number, on: boolean) {
    const feed = this.feeds.find((f) => f.id === id);
    if (!feed) return;
    try {
      const { feed: updated } = await api.setFeedFullText(id, on);
      feed.fullText = updated.fullText;
      this.notify(on ? `Full articles will load by themselves in ${feed.title}` : `Stopped loading full articles in ${feed.title}`);
      if (on && this.selected?.feedId === id) void this.loadFull();
    } catch (e) {
      this.notify(message(e), 'error');
    }
  }

  /** Opens the next or previous article in the list, loading more when the list runs out. */
  async step(delta: 1 | -1) {
    if (this.pane === 'feeds') return;
    let from = this.selectedIndex;
    if (from === -1) from = delta > 0 ? -1 : this.items.length;
    let to = from + delta;
    if (to >= this.items.length && this.next) {
      await this.more();
    }
    if (to < 0 || to >= this.items.length) return;
    await this.open(this.items[to]);
  }

  // --- read state ---

  async toggleRead() {
    const current = this.items.find((i) => i.id === this.selectedId) ?? this.selected;
    if (!current) return;
    await this.setRead(current.id, current.unread);
    this.announcement = current.unread ? 'Marked read' : 'Marked unread';
  }

  /** Marks one article read or unread, here and on the server. */
  async setRead(id: number, read: boolean) {
    const was = this.items.find((i) => i.id === id)?.unread ?? this.selected?.unread;
    if (was === undefined || was === !read) return;
    this.applyRead(id, read);
    try {
      await api.setRead(id, read);
    } catch (e) {
      this.applyRead(id, !read);
      this.notify(message(e), 'error');
    }
  }

  private applyRead(id: number, read: boolean) {
    const item = this.items.find((i) => i.id === id);
    if (item) {
      item.unread = !read;
      const feed = this.feeds.find((f) => f.id === item.feedId);
      if (feed) feed.unread = Math.max(0, feed.unread + (read ? -1 : 1));
    }
    if (this.selected?.id === id) this.selected.unread = !read;
  }

  async markAllRead() {
    const scope = this.currentFeed ? this.currentFeed.title : 'all feeds';
    try {
      const { marked } = await api.readAll(this.feedId);
      this.notify(marked === 0 ? `Nothing unread in ${scope}` : `${plural(marked, 'article', 'articles')} marked read in ${scope}`);
      await Promise.all([this.loadFeeds(), this.loadItems()]);
      if (this.selected && (this.feedId === null || this.selected.feedId === this.feedId)) this.selected.unread = false;
    } catch (e) {
      this.notify(message(e), 'error');
    }
  }

  // --- fetching feeds ---

  /** Fetches every feed in the background, then reloads what the page shows. */
  async refresh() {
    if (this.refreshing) return;
    const generation = this.generation;
    this.refreshing = true;
    this.announcement = 'Refreshing feeds';
    try {
      await api.refreshAll();
      // The server works in the background: look again a few times.
      for (const wait of [2000, 4000, 8000]) {
        await sleep(wait);
        if (generation !== this.generation) return;
        await Promise.all([this.loadFeeds(), this.loadItems()]);
      }
      this.notify('Feeds refreshed');
    } catch (e) {
      this.notify(message(e), 'error');
    } finally {
      if (generation === this.generation) this.refreshing = false;
    }
  }

  /** Fetches one feed now, then reloads what the page shows. A failure is shown and also kept on the feed. */
  async refreshFeed(id: number) {
    if (this.refreshingFeedId !== null) return;
    const generation = this.generation;
    this.refreshingFeedId = id;
    this.announcement = 'Refreshing this feed';
    let failure = '';
    try {
      await api.refreshFeed(id);
    } catch (e) {
      failure = message(e);
    }
    await Promise.all([this.loadFeeds(), this.loadItems()]);
    if (generation !== this.generation) return;
    this.refreshingFeedId = null;
    if (failure) this.notify(failure, 'error');
    else this.notify('Feed refreshed');
  }

  /** Subscribes to a feed. Throws an ApiError with a message for the form to show. */
  async addFeed(url: string) {
    const { feed } = await api.addFeed(url);
    await Promise.all([this.loadFeeds(), this.loadItems()]);
    this.notify(`Following ${feed.title}`);
    return feed;
  }

  async removeFeed(id: number) {
    const title = this.feeds.find((f) => f.id === id)?.title ?? 'the feed';
    try {
      await api.deleteFeed(id);
      if (this.feedId === id) this.feedId = null;
      if (this.selected?.feedId === id) {
        this.selected = null;
        this.selectedId = null;
      }
      await Promise.all([this.loadFeeds(), this.loadItems()]);
      this.notify(`Unfollowed ${title}`);
    } catch (e) {
      this.notify(message(e), 'error');
    }
  }

  /** Files a feed under a topic ("" for none). */
  async setFeedCategory(id: number, category: string) {
    const feed = this.feeds.find((f) => f.id === id);
    if (!feed || feed.category === category.trim()) return;
    try {
      const { feed: updated } = await api.setFeedCategory(id, category);
      feed.category = updated.category;
      this.notify(updated.category ? `${feed.title} is now under ${updated.category}` : `${feed.title} has no topic now`);
    } catch (e) {
      this.notify(message(e), 'error');
    }
  }

  /** The topics in use, for suggesting when filing a feed. */
  topics = $derived([...new Set(this.feeds.map((f) => f.category).filter(Boolean))].sort((a, b) => a.localeCompare(b)));

  /** Called after an OPML import: new feeds appear as they are fetched. */
  async reloadSoon() {
    const generation = this.generation;
    for (const wait of [1500, 5000, 10000]) {
      await sleep(wait);
      if (generation !== this.generation) return;
      await Promise.all([this.loadFeeds(), this.loadItems()]);
    }
  }
}

export const reader = new Reader();
