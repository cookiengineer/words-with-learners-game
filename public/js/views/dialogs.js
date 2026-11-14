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
    this.inviteSection = document.getElementById("invite-players-section");
    this.inviteHeading = this.inviteSection?.querySelector("h3");
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
    const count = Number(this.players.value);
    const solo = count === 1;
    const needed = count - 1;
    if (solo && (this.picker.selected || []).length > 0) {
      this.picker.setSelected([]);
    }
    this.picker.setMax(Math.max(1, needed));
    if (this.inviteSection) {
      if (this.inviteHeading) {
        this.inviteHeading.hidden = solo;
      }
      this.selectedNode.hidden = solo;
      this.picker.hidden = solo;
    }
    const selected = this.picker.selected || [];
    this.selectedNode.replaceChildren(...selected.map((profile) => el("span", { class: "chip" }, [
      profile.username,
      el("button", {
        type: "button",
        "aria-label": `Remove ${profile.username}`,
        text: "×",
        onclick: () => this.picker.remove(profile.id),
      }),
    ])));
    if (solo) {
      this.hint.textContent = "Training game — no opponents needed.";
    } else if (needed === 1) {
      this.hint.textContent = "Select 1 player for a 2 player game.";
    } else {
      this.hint.textContent = `Select ${needed} players for a ${this.players.value} player game.`;
    }
    this.createButton.disabled = this.busy || (!solo && selected.length !== needed);
  }

  async submit(event) {
    event.preventDefault();
    if (this.busy) {
      return;
    }
    const count = Number(this.players.value);
    const needed = count - 1;
    const selected = count === 1 ? [] : this.picker.selected || [];
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
        playerCount: count,
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

function inviteHost() {
  const advertised = globalThis.WWL_ADVERTISE;
  if (typeof advertised === "string") {
    const value = advertised.trim().replace(/^https?:\/\//i, "").replace(/\/+$/, "");
    if (value) {
      return value;
    }
  }
  return location.host;
}

function isLoopbackHost(host) {
  const value = host.toLowerCase();
  return value === "localhost" ||
    value.startsWith("localhost:") ||
    value === "[::1]" ||
    value.startsWith("[::1]:") ||
    value.startsWith("127.");
}

async function copyText(text) {
  const bridge = globalThis.WWLAndroid;
  if (bridge && typeof bridge.copyText === "function") {
    bridge.copyText(text);
    return;
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      console.debug("Clipboard API unavailable");
    }
  }
  const area = document.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.top = "-1000px";
  area.style.opacity = "0";
  document.body.append(area);
  area.select();
  area.setSelectionRange(0, text.length);
  const copied = document.execCommand("copy");
  area.remove();
  if (!copied) {
    throw new Error("Could not copy invite message");
  }
}

export class ShareInviteDialog {
  constructor({ getProfile }) {
    this.getProfile = getProfile;
    this.dialog = document.getElementById("share-invite-dialog");
    this.messageNode = document.getElementById("share-invite-message");
    this.note = document.getElementById("share-invite-note");
    this.copyButton = document.getElementById("share-invite-copy");
    this.message = "";
    this.bind();
  }

  bind() {
    for (const button of this.dialog.querySelectorAll("[data-close-share-invite]")) {
      button.addEventListener("click", () => this.dialog.close());
    }
    this.copyButton.addEventListener("click", () => this.copy());
  }

  open() {
    const username = this.getProfile()?.username || "a player";
    const host = inviteHost();
    this.message = `You've been invited by ${username} to play Words with Learners together.\n\nGo to http://${host} in your Web Browser to join!`;
    this.messageNode.textContent = this.message;
    this.note.hidden = !isLoopbackHost(host);
    this.note.textContent = "No LAN address detected. Connect to Wi-Fi or Ethernet, or open this server through its network address, to invite friends.";
    if (!this.dialog.open) {
      this.dialog.showModal();
    }
  }

  async copy() {
    try {
      await copyText(this.message);
      toast("Invite message copied");
    } catch (error) {
      toast(error.message || "Could not copy invite message");
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
