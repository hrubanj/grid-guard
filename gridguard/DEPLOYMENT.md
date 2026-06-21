# grid-guard — operating a deployed instance

How to reach a deployed instance, what the on-host layout looks like, and how to
redeploy. (Build/deploy mechanics and cloud setup are in `README.md`; this file is
the day-to-day operations runbook.)

Throughout, replace the placeholders with your own values:

| Placeholder | Meaning |
|-------------|---------|
| `<HOST>` | the SSH host alias or `user@ip` of your VM |
| `<SSH_USER>` | the login user (e.g. `opc` on Oracle Linux, `ubuntu` on Ubuntu) |
| `<DEPLOY_PATH>` | a writable staging dir on the host, e.g. `/home/<SSH_USER>/grid-guard` |

## Role: run exactly one instance

grid-guard is meant to be the **sole** controller for a given inverter. It runs on
a schedule and reconciles the inverter each time. Do **not** enable a second timer
elsewhere against the same inverter — a real switch would then double-send Telegram
messages (the inverter state itself is stateless-safe, so the hardware won't be
harmed, but you'll get duplicate alerts).

## Access

Set up an `~/.ssh/config` entry on your operator machine so `ssh <HOST>` just works:

```
Host <HOST>
    HostName <YOUR_VM_IP>
    User <SSH_USER>
    IdentityFile ~/.ssh/<YOUR_KEY>
    IdentitiesOnly yes
```

> On many free-tier clouds the public IP is **ephemeral**: it survives reboots but
> changes if the instance is *stopped/started*. To make it permanent, convert it to
> a **reserved/static** public IP in your provider's console and update `HostName`.

## Install layout (on the VM)

The recommended layout that `deploy.sh push` and the systemd units assume:

| Path | What |
|------|------|
| `/usr/local/bin/grid-guard` | the static binary |
| `/etc/grid-guard/config.json` | config + secrets, mode `0600`, root-owned |
| `/etc/systemd/system/grid-guard.{service,timer}` | the oneshot service + timer |

The timer fires at `:00/:15/:30/:45 +1s` (15-min OTE segment boundaries). If your VM
clock is UTC, those boundaries still align with whole-hour-offset zones such as
`Europe/Prague`; all price logic uses the `timezone` from config regardless of the
host clock.

## Redeploy a new build

From `gridguard/` on your operator machine:
```bash
DEPLOY_TARGET=<HOST> DEPLOY_PATH=<DEPLOY_PATH> ./deploy.sh push
ssh <HOST> 'sudo install -m755 <DEPLOY_PATH>/grid-guard /usr/local/bin/grid-guard'
```
(The `service`/`timer` units rarely change; reinstall them from
`<DEPLOY_PATH>/deploy/` if they do, then `sudo systemctl daemon-reload`.)

## Operate / observe

```bash
ssh <HOST>
sudo /usr/local/bin/grid-guard -config /etc/grid-guard/config.json prices   # read-only
sudo /usr/local/bin/grid-guard -config /etc/grid-guard/config.json state    # read-only
sudo systemctl start grid-guard.service          # trigger one run now
systemctl list-timers grid-guard.timer           # next scheduled run
journalctl -u grid-guard -n 30 -f                # logs
```

## Gotchas learned in the field

- **Solax monitoring is region-bound.** `getRealtimeInfo` via `www.solaxcloud.com`
  geo-routes by requester IP, so an EU token returns `"no auth!"` from a non-EU host
  (e.g. a US VM). The code uses `global.solaxcloud.com:9443`, which works anywhere.
  The control API (login/paramInit/paramSet) is not affected.
- **Idle reclaim:** some free-tier accounts (e.g. un-upgraded Oracle Free Tier) can
  reclaim idle Always-Free instances. A 15-min timer keeps the VM active; upgrading to
  a Pay-As-You-Go account (still free for Always-Free resources) removes the risk.
