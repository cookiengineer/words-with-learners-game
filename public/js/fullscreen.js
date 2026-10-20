function element() {
  return document.fullscreenElement || document.webkitFullscreenElement || null;
}

export function fullscreenSupported() {
  const root = document.documentElement;
  if (!root) {
    return false;
  }
  return Boolean(root.requestFullscreen || root.webkitRequestFullscreen);
}

export function isFullscreen() {
  return element() !== null;
}

export function requestFullscreen(target = document.documentElement) {
  if (target.requestFullscreen) {
    return target.requestFullscreen({ navigationUI: "hide" });
  }
  if (target.webkitRequestFullscreen) {
    target.webkitRequestFullscreen();
    return Promise.resolve();
  }
  return Promise.reject(new Error("fullscreen is not supported"));
}

export function exitFullscreen() {
  if (document.exitFullscreen) {
    return document.exitFullscreen();
  }
  if (document.webkitExitFullscreen) {
    document.webkitExitFullscreen();
    return Promise.resolve();
  }
  return Promise.reject(new Error("fullscreen is not supported"));
}

export function toggleFullscreen(target = document.documentElement) {
  return isFullscreen() ? exitFullscreen() : requestFullscreen(target);
}

export function onFullscreenChange(callback) {
  document.addEventListener("fullscreenchange", callback);
  document.addEventListener("webkitfullscreenchange", callback);
  return () => {
    document.removeEventListener("fullscreenchange", callback);
    document.removeEventListener("webkitfullscreenchange", callback);
  };
}
