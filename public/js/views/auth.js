import * as api from "../api.js";
import { clear, template, toast } from "../ui.js";

export class AuthView {
  constructor(container, onAuthenticated) {
    this.container = container;
    this.onAuthenticated = onAuthenticated;
  }

  mount() {
    clear(this.container);
    this.container.append(template("auth-template"));
    const loginForm = this.container.querySelector("#login-form");
    const registerForm = this.container.querySelector("#register-form");
    const error = this.container.querySelector("#auth-error");
    const tabs = this.container.querySelectorAll("[data-auth-tab]");
    for (const tab of tabs) {
      tab.addEventListener("click", () => {
        for (const other of tabs) {
          other.classList.toggle("active", other === tab);
        }
        const login = tab.dataset.authTab === "login";
        loginForm.hidden = !login;
        registerForm.hidden = login;
        error.textContent = "";
      });
    }
    loginForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.textContent = "";
      const data = new FormData(loginForm);
      try {
        const profile = await api.login(data.get("username"), data.get("password"));
        this.onAuthenticated(profile);
      } catch (requestError) {
        error.textContent = requestError.message;
      }
    });
    registerForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.textContent = "";
      const data = new FormData(registerForm);
      try {
        const profile = await api.register(data.get("username"), data.get("displayName"), data.get("password"));
        this.onAuthenticated(profile);
        toast(`Welcome, ${profile.displayName}`);
      } catch (requestError) {
        error.textContent = requestError.message;
      }
    });
  }
}
