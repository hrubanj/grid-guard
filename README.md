# grid-guard

A tiny, stateless Go cronjob that stops a Solax inverter from selling to the grid
while the OTE spot price is ≤ 0 (selling at a loss), and posts a Telegram message
with a day-price chart whenever it switches. The tool lives in [`gridguard/`](gridguard/).

- Tool, usage, build & deploy: [`gridguard/README.md`](gridguard/README.md)
- Operations runbook: [`gridguard/DEPLOYMENT.md`](gridguard/DEPLOYMENT.md)
- License: [MIT](LICENSE)
