/** One counter per app instance (a module-level counter would leak between tests). */
export class Metrics {
  #requests = 0;

  countRequest(): void {
    this.#requests++;
  }

  render(): string {
    return `requests_total ${this.#requests}\n`;
  }
}
