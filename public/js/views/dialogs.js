import { socket } from "../socket.js";
import { el, toast } from "../ui.js";

export class InviteDialog {
  constructor({ onCreate }) {
    this.onCreate = onCreate;
    this.dialog = document.getElementById("invite-dialog");
    this.form = document.getElementById("invite-form");
    this.language = document.getElementById("invite-language");
    this.level = document.getElementById("invite-level");
    this.length = document.getElementById("invite-length");
    this.lengthOutput = document.getElementById("invite-length-output");
    this.players = document.getElementById("invite-players");
    this.playersOutput = document.getElementById("invite-players-output");
    this.picker = document.getElementById("invite-picker");
    this.selectedNode = document.getElementById("invite-selected");
    this.hint = document.getElementById("invite-hint");
    this.createButton = document.getElementById("invite-create");
    this.busy = false;
    this.bind();
  }

  bind() {
    for (const button of this.dialog.querySelectorAll("[data-close-invite]")) {
      button.addEventListener("click", () => this.dialog.close());
    }
    this.length.addEventListener("input", () => {
      this.lengthOutput.textContent = this.length.value;
    });
    this.players.addEventListener("input", () => {
      this.playersOutput.textContent = this.players.value;
      if (this.picker.initialized) {
        this.picker.setMax(Number(this.players.value) - 1);
      }
      this.update();
    });
    this.picker.addEventListener("selectionchange", () => this.update());
    this.form.addEventListener("submit", (event) => this.submit(event));
  }

  open(preselected = []) {
    const minimum = Math.max(2, Math.min(4, preselected.length + 1));
    this.players.value = String(minimum);
    this.playersOutput.textContent = this.players.value;
    this.lengthOutput.textContent = this.length.value;
    this.picker.setMax(Number(this.players.value) - 1);
    this.picker.setSelected(preselected);
    this.update();
    if (!this.dialog.open) {
      this.dialog.showModal();
    }
  }

  update() {
    const needed = Number(this.players.value) - 1;
    this.picker.setMax(needed);
    const selected = this.picker.selected || [];
    this.selectedNode.replaceChildren(...selected.map((profile) => el("span", { class: "chip" }, [
      profile.displayName,
      el("button", {
        type: "button",
        "aria-label": `Remove ${profile.displayName}`,
        text: "×",
        onclick: () => this.picker.remove(profile.id),
      }),
    ])));
    this.hint.textContent = needed === 1
      ? "Select 1 player for a 2 player game."
      : `Select ${needed} players for a ${this.players.value} player game.`;
    this.createButton.disabled = this.busy || selected.length !== needed;
  }

  async submit(event) {
    event.preventDefault();
    if (this.busy) {
      return;
    }
    const needed = Number(this.players.value) - 1;
    const selected = this.picker.selected || [];
    if (selected.length !== needed) {
      toast(`Select exactly ${needed} player${needed === 1 ? "" : "s"}`);
      return;
    }
    this.busy = true;
    this.createButton.disabled = true;
    const original = this.createButton.textContent;
    this.createButton.textContent = "Creating…";
    try {
      const payload = await socket.request("game.create", {
        language: this.language.value,
        level: this.level.value,
        maxWordLength: Number(this.length.value),
        playerCount: Number(this.players.value),
        opponents: selected.map((profile) => profile.id),
      });
      this.dialog.close();
      this.onCreate(payload);
    } catch (error) {
      toast(error.message);
    } finally {
      this.busy = false;
      this.createButton.textContent = original;
      this.update();
    }
  }
}

export class PlayersDialog {
  constructor({ onInvite }) {
    this.onInvite = onInvite;
    this.dialog = document.getElementById("players-dialog");
    this.picker = document.getElementById("players-picker");
    this.inviteButton = document.getElementById("players-invite");
    this.bind();
  }

  bind() {
    for (const button of this.dialog.querySelectorAll("[data-close-players]")) {
      button.addEventListener("click", () => this.dialog.close());
    }
    this.picker.addEventListener("selectionchange", () => {
      this.inviteButton.disabled = (this.picker.selected || []).length === 0;
    });
    this.inviteButton.addEventListener("click", () => {
      const selected = (this.picker.selected || []).map((profile) => ({ ...profile }));
      this.dialog.close();
      this.onInvite(selected);
    });
  }

  open() {
    this.picker.setMax(3);
    this.picker.setSelected([]);
    this.inviteButton.disabled = true;
    if (!this.dialog.open) {
      this.dialog.showModal();
    }
  }
}
