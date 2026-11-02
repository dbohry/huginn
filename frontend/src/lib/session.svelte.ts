import { api, ApiError, setUnauthorizedHandler, type User } from './api';

type Phase = 'loading' | 'setup' | 'anonymous' | 'authed';

/** Who is using the app. Screens are chosen from this. */
class Session {
  phase = $state<Phase>('loading');
  user = $state<User | null>(null);
  version = $state('');

  /** Works out whether setup is needed and whether a session already exists. */
  async init() {
    api.health().then((h) => (this.version = h.version)).catch(() => {});
    try {
      const { needsSetup } = await api.setupStatus();
      if (needsSetup) {
        this.phase = 'setup';
        return;
      }
      const { user } = await api.me();
      this.signedIn(user);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        this.phase = 'anonymous';
        return;
      }
      throw e;
    }
  }

  async setup(username: string, password: string) {
    const { user } = await api.setup(username, password);
    this.signedIn(user);
  }

  async login(username: string, password: string, remember: boolean) {
    const { user } = await api.login(username, password, remember);
    this.signedIn(user);
  }

  async logout() {
    await api.logout().catch(() => {});
    this.user = null;
    this.phase = 'anonymous';
  }

  async changePassword(current: string, next: string) {
    await api.changePassword(current, next);
    if (this.user) this.user = { ...this.user, mustChangePassword: false };
  }

  private signedIn(user: User) {
    this.user = user;
    this.phase = 'authed';
  }
}

export const session = new Session();

// If the server stops recognising the session while someone is using the app,
// show the sign-in screen instead of a broken page.
setUnauthorizedHandler(() => {
  if (session.phase === 'authed') {
    session.user = null;
    session.phase = 'anonymous';
  }
});
