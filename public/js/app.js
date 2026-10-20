import * as api from "./api.js";
import { socket } from "./socket.js";
import { clear, toast } from "./ui.js";
import { AuthView } from "./views/auth.js";
import { OverviewView } from "./views/overview.js";
import { GameView } from "./views/game.js";
import { InviteDialog, PlayersDialog } from "./views/dialogs.js";
import "./components/player-picker.js";

const app = document.getElementById("app");
const state = {
  profile: null,
  cleanup: null,
  route: "",
};

const context = {
  get profile() {
    return state.profile;
  },
  openInvite(preselected = []) {
    inviteDialog.open(preselected);
  },
  openPlayers() {
    playersDialog.open();
  },
  openGame(gameId) {
    location.hash = `#/game/${gameId}`;
  },
  onLeave() {
    location.hash = "#/games";
  },
  onLogout,
};

const inviteDialog = new InviteDialog({
  onCreate(game) {
    context.openGame(game.id);
  },
});

const playersDialog = new PlayersDialog({
  onInvite(profiles) {
    inviteDialog.open(profiles);
  },
});

async function onLogout() {
  try {
    await api.logout();
  } catch {
    // ignore logout failures
  }
  socket.close();
  state.profile = null;
  inviteDialog.dialog.close();
  playersDialog.dialog.close();
  location.hash = "#/auth";
  route();
}

function mount(view) {
  if (state.cleanup) {
    state.cleanup();
    state.cleanup = null;
  }
  state.cleanup = view.mount() || null;
}

function showAuth() {
  mount(new AuthView(app, async (profile) => {
    state.profile = profile;
    await socket.connect().catch((error) => toast(error.message));
    location.hash = "#/games";
    route();
  }));
}

function showOverview() {
  mount(new OverviewView(app, context));
}

function showGame(gameId) {
  mount(new GameView(app, context, gameId));
}

function route() {
  if (!state.profile) {
    showAuth();
    return;
  }
  const hash = location.hash || "#/games";
  const gameMatch = hash.match(/^#\/game\/([a-zA-Z0-9_-]+)$/);
  if (gameMatch) {
    showGame(gameMatch[1]);
  } else {
    showOverview();
  }
}

window.addEventListener("hashchange", route);

socket.addEventListener("connected", () => {
  if (state.profile) {
    socket.dispatchEvent(new CustomEvent("games.changed"));
  }
});

socket.addEventListener("disconnected", () => {
  if (state.profile) {
    toast("Connection lost. Reconnecting…");
  }
});

async function boot() {
  clear(app);
  try {
    const profile = await api.me();
    state.profile = profile;
    socket.connect().catch((error) => toast(error.message));
    route();
  } catch {
    state.profile = null;
    if (!location.hash || location.hash === "#/games") {
      location.hash = "#/auth";
    }
    route();
  }
}

boot();
