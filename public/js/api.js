export class ApiError extends Error {
  constructor(message, code, status) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

export async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  const text = await response.text();
  let payload = null;
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = null;
    }
  }
  if (!response.ok) {
    const code = payload?.code || "request_failed";
    const message = payload?.message || `Request failed with status ${response.status}`;
    throw new ApiError(message, code, response.status);
  }
  return payload;
}

export function register(username, displayName, password) {
  return api("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ username, displayName, password }),
  });
}

export function login(username, password) {
  return api("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export function logout() {
  return api("/api/auth/logout", { method: "POST", body: "{}" });
}

export function me() {
  return api("/api/auth/me");
}
