/** Flipped by `main.ts`: ready once the server listens, not ready again when shutdown starts. */
export class Readiness {
  #ready = false;

  get isReady(): boolean {
    return this.#ready;
  }

  markReady(): void {
    this.#ready = true;
  }

  markNotReady(): void {
    this.#ready = false;
  }
}
