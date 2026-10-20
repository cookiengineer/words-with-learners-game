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
    const form = this.container.querySelector("#auth-form");
    const error = this.container.querySelector("#auth-error");
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.textContent = "";
      const data = new FormData(form);
      try {
        const profile = await api.login(data.get("username"), data.get("password"));
        this.onAuthenticated(profile);
        toast(`Welcome, ${profile.username}`);
      } catch (requestError) {
        error.textContent = requestError.message;
      }
    });
  }
}
