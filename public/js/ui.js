
export function uniqueID() {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes);
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256);
    }
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = [...bytes].map((value) => value.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

export const languageNames = {
  nl: "Dutch",
  en: "English",
  de: "German",
  es: "Spanish",
  fr: "French",
  it: "Italian",
  ru: "Russian",
};

export function flagFor(language) {
  return `/flags/${language}.svg`;
}

export function template(id) {
  const source = document.getElementById(id);
  if (!source) {
    throw new Error(`Missing template ${id}`);
  }
  return source.content.cloneNode(true);
}

export function el(tag, attributes = {}, children = []) {
  const element = document.createElement(tag);
  for (const [key, value] of Object.entries(attributes)) {
    if (key === "class") {
      element.className = value;
    } else if (key === "text") {
      element.textContent = value;
    } else if (key === "dataset") {
      Object.assign(element.dataset, value);
    } else if (key.startsWith("on") && typeof value === "function") {
      element.addEventListener(key.slice(2).toLowerCase(), value);
    } else if (value === false) {
      continue;
    } else if (value === true) {
      element.setAttribute(key, "");
    } else if (value !== undefined && value !== null) {
      element.setAttribute(key, value);
    }
  }
  for (const child of children) {
    if (child === null || child === undefined) {
      continue;
    }
    element.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return element;
}

const toastQueue = [];
let toastActive = false;
let toastTimer = null;
let toastCooldown = null;
let activeToastKey = "";

function showNextToast() {
  const target = document.getElementById("toast");
  if (!target) {
    toastQueue.length = 0;
    toastActive = false;
    activeToastKey = "";
    return;
  }
  if (toastQueue.length === 0) {
    toastActive = false;
    activeToastKey = "";
    return;
  }
  toastActive = true;
  const entry = toastQueue.shift();
  activeToastKey = entry.key;
  const bottomBar = document.querySelector(".game-bottom");
  if (bottomBar) {
    target.style.setProperty("--toast-lift", `${Math.ceil(bottomBar.getBoundingClientRect().height) + 12}px`);
    target.classList.add("game-toast");
  } else {
    target.classList.remove("game-toast");
    target.style.removeProperty("--toast-lift");
  }
  target.textContent = entry.message;
  target.classList.add("visible");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    target.classList.remove("visible");
    clearTimeout(toastCooldown);
    toastCooldown = setTimeout(showNextToast, 280);
  }, 3000);
}

export function toast(message, key = "") {
  if (!message) {
    return;
  }
  const target = document.getElementById("toast");
  if (key && toastActive && activeToastKey === key && target?.classList.contains("visible")) {
    target.textContent = message;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      target.classList.remove("visible");
      clearTimeout(toastCooldown);
      toastCooldown = setTimeout(showNextToast, 280);
    }, 3000);
    return;
  }
  if (key) {
    const queued = toastQueue.find((entry) => entry.key === key);
    if (queued) {
      queued.message = message;
      return;
    }
  }
  if (toastQueue.length >= 5) {
    toastQueue.shift();
  }
  toastQueue.push({ message, key });
  if (!toastActive) {
    showNextToast();
  }
}

export function clear(node) {
  while (node.firstChild) {
    node.removeChild(node.firstChild);
  }
}

export function formatWhen(value) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

export function statusLabel(status) {
  if (status === "finished") {
    return "Finished";
  }
  return "Playing";
}

export function cloneProfile(profile) {
  return { id: profile.id, username: profile.username };
}

export function clearToasts() {
  toastQueue.length = 0;
  clearTimeout(toastTimer);
  clearTimeout(toastCooldown);
  toastActive = false;
  activeToastKey = "";
  const target = document.getElementById("toast");
  if (target) {
    target.classList.remove("visible", "game-toast");
    target.style.removeProperty("--toast-lift");
  }
}
