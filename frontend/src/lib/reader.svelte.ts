import { api, ApiError, type Feed, type Item, type ItemDetail } from './api';

/** On a narrow screen only one pane shows at a time. */
export type Pane = 'feeds' | 'list' | 'article';

const message = (e: unknown) => (e instanceof ApiError ? e.message : 'Something went wrong.');

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

  loadingList = $state(false);
  loadingMore = $state(false);
  loadingItem = $state(false);
  refreshing = $state(false);
  /** A problem to show above the lists, or "". */
  error = $state('');
  /** Said aloud by screen readers. */
  announcement = $state('');

  totalUnread = $derived(this.feeds.reduce((n, f) => n + f.unread, 0));
  currentFeed = $derived(this.feeds.find((f) => f.id === this.feedId) ?? null);
  title = $derived(this.currentFeed ? this.currentFeed.title : 'All items');

  // Bumped by every list load, so a slow answer to an old request is ignored.
  private listToken = 0;
  private itemToken = 0;

  async init() {
    await Promise.all([this.loadFeeds(), this.loadItems()]);
  }

  async loadFeeds() {
    try {
      this.feeds = (await api.feeds()).feeds;
    } catch (e) {
      this.error = message(e);
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
      this.error = '';
    } catch (e) {
      if (token === this.listToken) this.error = message(e);
    } finally {
      if (token === this.listToken) this.loadingList = false;
    }
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
      this.error = message(e);
    } finally {
      this.loadingMore = false;
    }
  }

  async chooseFeed(id: number | null) {
    this.feedId = id;
    this.pane = 'list';
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
    this.pane = 'article';
    this.loadingItem = true;
    try {
      const { item: detail } = await api.item(item.id);
      if (token !== this.itemToken) return;
      this.selected = detail;
      this.error = '';
      if (item.unread) await this.setRead(item.id, true);
    } catch (e) {
      if (token === this.itemToken) this.error = message(e);
    } finally {
      if (token === this.itemToken) this.loadingItem = false;
    }
  }

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
      this.error = message(e);
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
    try {
      const { marked } = await api.readAll(this.feedId);
      this.announcement = marked === 1 ? '1 article marked read' : `${marked} articles marked read`;
      await Promise.all([this.loadFeeds(), this.loadItems()]);
      if (this.selected) this.selected.unread = false;
    } catch (e) {
      this.error = message(e);
    }
  }

  /** Fetches every feed in the background, then reloads what the page shows. */
  async refresh() {
    if (this.refreshing) return;
    this.refreshing = true;
    this.announcement = 'Refreshing feeds';
    try {
      await api.refreshAll();
      // The server works in the background: look again a few times.
      for (const wait of [2000, 4000, 8000]) {
        await new Promise((r) => setTimeout(r, wait));
        await Promise.all([this.loadFeeds(), this.loadItems()]);
      }
      this.announcement = 'Feeds refreshed';
    } catch (e) {
      this.error = message(e);
    } finally {
      this.refreshing = false;
    }
  }

  /** Subscribes to a feed. Throws an ApiError with a message for the form to show. */
  async addFeed(url: string) {
    const { feed } = await api.addFeed(url);
    await Promise.all([this.loadFeeds(), this.loadItems()]);
    this.announcement = `Added ${feed.title}`;
    return feed;
  }

  async removeFeed(id: number) {
    try {
      await api.deleteFeed(id);
      if (this.feedId === id) this.feedId = null;
      if (this.selected?.feedId === id) {
        this.selected = null;
        this.selectedId = null;
      }
      await Promise.all([this.loadFeeds(), this.loadItems()]);
    } catch (e) {
      this.error = message(e);
    }
  }

  /** Called after an OPML import: new feeds appear as they are fetched. */
  async reloadSoon() {
    for (const wait of [1500, 5000, 10000]) {
      await new Promise((r) => setTimeout(r, wait));
      await Promise.all([this.loadFeeds(), this.loadItems()]);
    }
  }
}

export const reader = new Reader();
