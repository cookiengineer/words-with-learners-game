import { socket } from "../socket.js";
import { clear, el, flagFor, formatWhen, languageNames, statusLabel, template, toast } from "../ui.js";

export class OverviewView {
  constructor(container, context) {
    this.container = container;
    this.context = context;
    this.mounted = false;
    this.reloadTimer = null;
    this.offChanged = null;
  }

  mount() {
    clear(this.container);
    this.container.append(template("overview-template"));
    this.mounted = true;
    this.container.querySelector("#overview-user").textContent = `${this.context.profile.displayName} (@${this.context.profile.username})`;
    this.container.querySelector("#open-invite").addEventListener("click", () => this.context.openInvite());
    this.container.querySelector("#open-players").addEventListener("click", () => this.context.openPlayers());
    this.container.querySelector("#logout").addEventListener("click", () => this.context.onLogout());
    this.offChanged = socket.on("games.changed", () => this.scheduleReload());
    this.reload();
    return () => {
      this.mounted = false;
      clearTimeout(this.reloadTimer);
      if (this.offChanged) {
        this.offChanged();
      }
    };
  }

  scheduleReload() {
    clearTimeout(this.reloadTimer);
    this.reloadTimer = setTimeout(() => this.reload(), 150);
  }

  async reload() {
    try {
      const payload = await socket.request("games.list", {});
      if (!this.mounted) {
        return;
      }
      this.renderGames(payload?.games || []);
    } catch (error) {
      if (this.mounted) {
        toast(error.message);
      }
    }
  }

  renderGames(games) {
    const list = this.container.querySelector("#game-list");
    const empty = this.container.querySelector("#games-empty");
    clear(list);
    empty.hidden = games.length > 0;
    for (const game of games) {
      const players = el("div", { class: "players" }, game.players.map((player) => el("div", { class: "player-line" }, [
        el("span", { text: player.name }),
        el("strong", { text: String(player.score) }),
      ])));
      const config = game.config || {};
      const header = el("header", {}, [
        el("img", { class: "flag", src: flagFor(config.language), alt: `${languageNames[config.language]} flag` }),
        el("div", {}, [
          el("strong", { text: languageNames[config.language] || config.language }),
          el("div", { class: "muted", text: `${config.level} · up to ${config.maxWordLength} tiles` }),
        ]),
      ]);
      const footer = el("div", { class: "muted", text: `${statusLabel(game.status)} · ${formatWhen(game.updatedAt)}` });
      const card = el("button", {
        type: "button",
        class: "game-card",
        onclick: () => this.context.openGame(game.id),
      }, [header, players, footer]);
      if (game.yourTurn) {
        card.append(el("span", { class: "badge", text: "Your turn" }));
      } else if (game.status === "finished") {
        const winner = game.players.find((player) => player.id === game.winner);
        card.append(el("span", { class: "badge", text: winner ? `${winner.name} won` : "Tie game" }));
      }
      list.append(card);
    }
  }
}
