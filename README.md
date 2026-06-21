# sun-commander

**[`grid-guard`](gridguard/)** — a tiny, stateless Go cronjob that stops a Solax
inverter from selling to the grid while the OTE spot price is ≤ 0 (selling at a
loss), and posts a Telegram message with a day-price chart whenever it switches.

- Tool, usage, build & deploy: [`gridguard/README.md`](gridguard/README.md)
- Operations runbook: [`gridguard/DEPLOYMENT.md`](gridguard/DEPLOYMENT.md)
- License: [MIT](LICENSE)

<sub>History: this repo began as a larger Python solar-optimization prototype, which
was removed in favour of grid-guard. The prototype remains in the git history.</sub>
