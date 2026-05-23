# SBC Hosting Guide

This guide is for running `mamacord` on small single-board computers:

- Raspberry Pi
- Orange Pi
- ODROID

This version is intentionally repetitive.

You should never have to guess:

- which machine a command runs on
- which directory a command runs in
- which file should exist before the command
- which file should exist after the command

## Fast Answer

If you want the shortest safe answer:

- weak boards should usually **run** the bot, not **build** it
- strong boards can build and run locally
- if you are already on the board and the repo is cloned there, use [Local Build On The Device](#local-build-on-the-device)
- if you are on your stronger laptop/desktop building for the board, use [Cross-Build On Another Machine](#cross-build-on-another-machine)
- if `/opt/mamacord` does not exist yet, use [First Install On A Fresh Device](#first-install-on-a-fresh-device)
- if `/opt/mamacord` and `mamacord.service` already exist, use [Update An Existing Install](#update-an-existing-install)

## Path + Machine Legend

These words mean exact things in this guide.

- `BUILD HOST`: the machine that compiles the binary
- `TARGET DEVICE`: the SBC that runs the bot
- `REPO CHECKOUT`: your git clone, for example `~/go-mamacord`
- `INSTALLED APP DIR`: `/opt/mamacord`
- `INSTALLED BINARY`: `/opt/mamacord/mamacord`

Hard rules:

- do not mix `REPO CHECKOUT` commands with `INSTALLED APP DIR` commands
- do not mix `BUILD HOST` commands with `TARGET DEVICE` commands
- do not assume `./dist/mamacord` and `./dist/mamacord-linux-arm64` are the same file

## Table Of Contents

- [Fast Answer](#fast-answer)
- [Path + Machine Legend](#path--machine-legend)
- [Pick Your Deployment Profile](#pick-your-deployment-profile)
- [Board Matrix](#board-matrix)
- [Stop Here If This Is Your Situation](#stop-here-if-this-is-your-situation)
- [Golden Path A: Weak Board, Build Elsewhere](#golden-path-a-weak-board-build-elsewhere)
- [Golden Path B: Stronger Board, Build Locally](#golden-path-b-stronger-board-build-locally)
- [Local Build On The Device](#local-build-on-the-device)
- [Cross-Build On Another Machine](#cross-build-on-another-machine)
- [First Install On A Fresh Device](#first-install-on-a-fresh-device)
- [Update An Existing Install](#update-an-existing-install)
- [Environment Examples](#environment-examples)
- [Production Plugin Signing](#production-plugin-signing)
- [Dashboard Build Guidance](#dashboard-build-guidance)
- [Troubleshooting](#troubleshooting)

## Pick Your Deployment Profile

### Profile A: Bot Only On Device

Host on device:

- Discord bot
- SQLite

Do not host on device:

- admin API
- dashboard

Best for:

- weakest boards
- lowest setup complexity
- lowest RAM use

### Profile B: Bot + Admin API On Device, Dashboard Elsewhere

Host on device:

- Discord bot
- SQLite
- admin API

Host elsewhere:

- static dashboard site

Best for:

- weak or mid-range boards
- users who want dashboard access without asking the board to host frontend files

### Profile C: Full Stack On Device

Host on device:

- Discord bot
- SQLite
- admin API
- built `apps/dashboard/dist`

Best for:

- stronger boards
- one-box LAN or homelab setups

Important:

- this means built frontend files only
- this does not mean normal Vite dev-server usage

## Board Matrix

Use this as the main pick table.

| Board / Class         | Target arch                                          | RAM class                 | Native build                        | Best profiles | Dashboard build advice                     |
| --------------------- | ---------------------------------------------------- | ------------------------- | ----------------------------------- | ------------- | ------------------------------------------ |
| Raspberry Pi Zero 2 W | `linux/arm64` on 64-bit OS, `linux/arm` on 32-bit OS | 512MB                     | painful                             | A, B          | prebuild elsewhere                         |
| Orange Pi Zero 2W     | `linux/arm64`                                        | 1GB / 1.5GB / 2GB / 4GB   | weak-to-acceptable depending on RAM | A, B          | prebuild elsewhere                         |
| Raspberry Pi 3        | `linux/arm` or `linux/arm64`                         | 1GB                       | acceptable but slow                 | A, B          | prebuild elsewhere                         |
| ODROID M1S            | `linux/arm64`                                        | 4GB / 8GB                 | good                                | A, B, C       | local build is fine; prebuild still faster |
| Raspberry Pi 4        | `linux/arm64` preferred                              | 2GB / 4GB / 8GB           | good                                | A, B, C       | local build is fine                        |
| Raspberry Pi 5        | `linux/arm64`                                        | 4GB / 8GB                 | very good                           | A, B, C       | local build is fine                        |
| Orange Pi 5 family    | `linux/arm64`                                        | 4GB / 8GB / 16GB / higher | very good                           | A, B, C       | local build is fine                        |

Rule for unmapped boards:

- if it feels like Zero-class hardware, treat it like Zero 2 / Zero 2W
- if it feels like Pi 3 / RK3566 class hardware, treat it like Pi 3 / ODROID M1S
- if it has 4GB+ and a modern 64-bit SoC, treat it like Pi 4/5 or Orange Pi 5 class

Representative vendor pages:

- Raspberry Pi Zero 2 W:
  <https://www.raspberrypi.com/products/raspberry-pi-zero-2-w/>
- Orange Pi Zero 2W:
  <https://www.orangepi.org/orangepiwiki/index.php/Orange_Pi_Zero_2W>
- Orange Pi 5:
  <https://www.orangepi.org/html/hardWare/computerAndMicrocontrollers/details/Orange-Pi-5.html>
- ODROID M1S:
  <https://www.hardkernel.com/blog-2/odroid-m1s/>

## Stop Here If This Is Your Situation

- If you are already on the board and the repo is cloned there:
  go to [Local Build On The Device](#local-build-on-the-device)
- If you are on your laptop or desktop building for the board:
  go to [Cross-Build On Another Machine](#cross-build-on-another-machine)
- If `/opt/mamacord` does not exist yet:
  go to [First Install On A Fresh Device](#first-install-on-a-fresh-device)
- If `/opt/mamacord` already exists and the service already works:
  go to [Update An Existing Install](#update-an-existing-install)

## Golden Path A: Weak Board, Build Elsewhere

Use this for:

- Pi Zero 2 W
- Orange Pi Zero 2W
- any board where local builds are miserable

Shape:

- `BUILD HOST`: stronger x86_64 or arm64 machine
- `TARGET DEVICE`: weak SBC
- profile: usually A or B

### A1. Build The Binary

Run on `BUILD HOST`, inside `REPO CHECKOUT`.

Input expected before command:

- repo is cloned
- you are in that repo

Output after command:

- `./dist/mamacord-linux-arm64` for 64-bit targets
- or `./dist/mamacord-linux-armv7` for 32-bit targets

64-bit target:

```bash
GOOS=linux GOARCH=arm64 ./scripts/build-release.sh dist/mamacord-linux-arm64
```

32-bit target:

```bash
GOOS=linux GOARCH=arm GOARM=7 ./scripts/build-release.sh dist/mamacord-linux-armv7
```

### A2. Copy The Built Binary To The Board

Placeholder used below:

- `USER@TARGET_HOST`

Real example:

- `krystian@mamaberry.local`

Run on `BUILD HOST`, inside `REPO CHECKOUT`.

Input expected before command:

- build output exists in `./dist/`
- the target board is reachable over SSH

Output after command:

- `~/mamacord` exists on the `TARGET DEVICE`

64-bit target:

```bash
rsync -a ./dist/mamacord-linux-arm64 USER@TARGET_HOST:~/mamacord
```

32-bit target:

```bash
rsync -a ./dist/mamacord-linux-armv7 USER@TARGET_HOST:~/mamacord
```

### A3. Install It On The Board

Run on `TARGET DEVICE`.

Input expected before command:

- `~/mamacord` exists on the device
- `/opt/mamacord` already exists if this is an update

Output after command:

- `INSTALLED BINARY` exists

```bash
sudo install -Dm755 ~/mamacord /opt/mamacord/mamacord
```

### A4. Restart And Verify

Run on `TARGET DEVICE`.

Output after command:

- service restarted
- `doctor` reports the installed production config

```bash
sudo systemctl restart mamacord
/opt/mamacord/mamacord doctor
```

## Golden Path B: Stronger Board, Build Locally

Use this for:

- Raspberry Pi 4
- Raspberry Pi 5
- ODROID M1S
- Orange Pi 5 family

Shape:

- `TARGET DEVICE` is also the build machine
- repo is cloned on the board

### B1. Build On The Board

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Input expected before command:

- repo is cloned on the board

Output after command:

- `./dist/mamacord`

```bash
./scripts/build-release.sh
```

### B2. Install The Built Binary

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Input expected before command:

- `./dist/mamacord` exists

Output after command:

- `INSTALLED BINARY` exists

```bash
sudo install -Dm755 ./dist/mamacord /opt/mamacord/mamacord
```

### B3. Restart And Verify

Run on `TARGET DEVICE`.

```bash
sudo systemctl restart mamacord
/opt/mamacord/mamacord doctor
```

## Local Build On The Device

Use this section only if both of these are true:

- you are already on the `TARGET DEVICE`
- the repo is cloned on that device

Do not use this section if you are building on another machine.

### Confirm Where You Are

Run on `TARGET DEVICE`.

Goal:

- confirm you are inside `REPO CHECKOUT`

```bash
pwd
ls -la
```

You should see repo files such as:

- `go.mod`
- `scripts/build-release.sh`
- `migrations/`
- `locales/`

### Build

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Output after command:

- `./dist/mamacord`

```bash
./scripts/build-release.sh
```

### Confirm The Build Output Exists

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Expected output:

- a file at `./dist/mamacord`

```bash
ls -la ./dist
find ./dist -maxdepth 1 -type f -name 'mamacord*'
```

### Install The Binary

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Input expected before command:

- `./dist/mamacord` exists

Output after command:

- `INSTALLED BINARY` exists

```bash
sudo install -Dm755 ./dist/mamacord /opt/mamacord/mamacord
```

### Install Repo Assets

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Input expected before command:

- `./migrations/`
- `./locales/`
- `./plugins/`
- `./config/`
- optionally `./apps/dashboard/dist/` for Profile C

Output after command:

- those assets exist under `INSTALLED APP DIR`

```bash
sudo rsync -a ./migrations/ /opt/mamacord/migrations/
sudo rsync -a ./locales/ /opt/mamacord/locales/
sudo rsync -a ./plugins/ /opt/mamacord/plugins/
sudo rsync -a ./config/ /opt/mamacord/config/
```

For Profile C only:

```bash
sudo rsync -a ./apps/dashboard/dist/ /opt/mamacord/apps/dashboard/dist/
```

### Install `.env.prod`

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Input expected before command:

- `./.env.prod` exists in the repo checkout

Output after command:

- `/opt/mamacord/.env.prod`

```bash
sudo install -Dm600 ./.env.prod /opt/mamacord/.env.prod
sudo chown -R mamacord:mamacord /opt/mamacord
```

### Restart And Verify

Run on `TARGET DEVICE`.

```bash
sudo systemctl restart mamacord
/opt/mamacord/mamacord doctor
```

## Cross-Build On Another Machine

Use this section only if both of these are true:

- the repo checkout is on your stronger computer
- the board is a different machine

Do not use this section if you are already on the board.

### Pick The Target Architecture

Use these rules:

- `linux/arm64` for 64-bit Pi 4/5, Orange Pi 5, ODROID M1S, and similar boards
- `linux/arm` plus `GOARM=7` for 32-bit Raspberry Pi OS on Pi 3 / Zero 2 W

### Build For 64-bit Targets

Run on `BUILD HOST`, inside `REPO CHECKOUT`.

Output after command:

- `./dist/mamacord-linux-arm64`

```bash
GOOS=linux GOARCH=arm64 ./scripts/build-release.sh dist/mamacord-linux-arm64
```

### Build For 32-bit Targets

Run on `BUILD HOST`, inside `REPO CHECKOUT`.

Output after command:

- `./dist/mamacord-linux-armv7`

```bash
GOOS=linux GOARCH=arm GOARM=7 ./scripts/build-release.sh dist/mamacord-linux-armv7
```

### Confirm The Output Exists

Run on `BUILD HOST`, inside `REPO CHECKOUT`.

```bash
ls -la ./dist
find ./dist -maxdepth 1 -type f -name 'mamacord*'
```

### Copy To The Target Device

Placeholder used below:

- `USER@TARGET_HOST`

Real example:

- `krystian@mamaberry.local`

Run on `BUILD HOST`, inside `REPO CHECKOUT`.

Input expected before command:

- build output exists in `./dist/`

Output after command:

- `~/mamacord` exists on the target board

64-bit target:

```bash
rsync -a ./dist/mamacord-linux-arm64 USER@TARGET_HOST:~/mamacord
```

32-bit target:

```bash
rsync -a ./dist/mamacord-linux-armv7 USER@TARGET_HOST:~/mamacord
```

### Install The Copied Binary

Run on `TARGET DEVICE`.

Input expected before command:

- `~/mamacord` exists

Output after command:

- `INSTALLED BINARY` exists

```bash
sudo install -Dm755 ~/mamacord /opt/mamacord/mamacord
```

### Restart And Verify

Run on `TARGET DEVICE`.

```bash
sudo systemctl restart mamacord
/opt/mamacord/mamacord doctor
```

## First Install On A Fresh Device

Use this only if the target device does not already have:

- `/opt/mamacord`
- `/etc/systemd/system/mamacord.service`

This section is about the initial bootstrap.

### 1. Create The Service User

Run on `TARGET DEVICE`.

Output after command:

- system user `mamacord` exists

```bash
sudo useradd --system --home /opt/mamacord --shell /usr/sbin/nologin mamacord || true
```

### 2. Create The Install Tree

Run on `TARGET DEVICE`.

Output after command:

- `/opt/mamacord`
- `/opt/mamacord/data`

```bash
sudo install -d -o mamacord -g mamacord /opt/mamacord
sudo install -d -o mamacord -g mamacord /opt/mamacord/data
```

### 3. Install The Binary

Pick one:

- if you built on the device, go back to [Local Build On The Device](#local-build-on-the-device)
- if you built elsewhere, go back to [Cross-Build On Another Machine](#cross-build-on-another-machine)

After this step, you must have:

- `/opt/mamacord/mamacord`

### 4. Install Repo Assets

If the repo checkout exists on the board, run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

Output after command:

- `/opt/mamacord/migrations/`
- `/opt/mamacord/locales/`
- `/opt/mamacord/plugins/`
- `/opt/mamacord/config/`

```bash
sudo rsync -a ./migrations/ /opt/mamacord/migrations/
sudo rsync -a ./locales/ /opt/mamacord/locales/
sudo rsync -a ./plugins/ /opt/mamacord/plugins/
sudo rsync -a ./config/ /opt/mamacord/config/
```

If you do not have a repo checkout on the board, copy these directories there first from the build host, then install them into `/opt/mamacord/`.

### 5. Install `.env.prod`

Run on `TARGET DEVICE`.

Input expected before command:

- you already created `.env.prod`

Output after command:

- `/opt/mamacord/.env.prod`

If `.env.prod` is already on the target in your current directory:

```bash
sudo install -Dm600 ./.env.prod /opt/mamacord/.env.prod
```

If `.env.prod` is in your home directory:

```bash
sudo install -Dm600 ~/.env.prod /opt/mamacord/.env.prod
```

### 5.5 Production Plugin Signing

If your `.env.prod` contains both of these:

- `MAMACORD_PROD_MODE=1`
- `MAMACORD_ALLOW_UNSIGNED_PLUGINS=0`

then you must finish the signing setup before the service will boot.

Use [Production Plugin Signing](#production-plugin-signing) before you start the service.

### 6. Install The Service

Run on `TARGET DEVICE`.

Create `/etc/systemd/system/mamacord.service` with:

```ini
[Unit]
Description=mamacord
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=mamacord
Group=mamacord
WorkingDirectory=/opt/mamacord
ExecStart=/opt/mamacord/mamacord
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

### 7. Start The Service

Run on `TARGET DEVICE`.

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now mamacord
sudo systemctl status mamacord --no-pager
```

### 8. Fix Ownership

Run on `TARGET DEVICE`.

```bash
sudo chown -R mamacord:mamacord /opt/mamacord
```

### 9. Verify With `doctor`

Run on `TARGET DEVICE`.

```bash
/opt/mamacord/mamacord doctor
```

You want to see:

- `env_file_loaded: .env.prod`
- `discord_token: true`
- `prod_mode: true`
- `trusted_keys_file_exists: true`

## Update An Existing Install

Use this only if both of these are already true:

- `/opt/mamacord` exists
- `mamacord.service` already exists

### Update Path 1: Build On The Board

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`.

```bash
git pull --ff-only
diff -u .env.prod.example .env.prod
./scripts/build-release.sh
sudo install -Dm755 ./dist/mamacord /opt/mamacord/mamacord
sudo rsync -a ./migrations/ /opt/mamacord/migrations/
sudo rsync -a ./locales/ /opt/mamacord/locales/
sudo rsync -a ./plugins/ /opt/mamacord/plugins/
sudo rsync -a ./config/ /opt/mamacord/config/
sudo install -Dm600 ./.env.prod /opt/mamacord/.env.prod
sudo systemctl restart mamacord
/opt/mamacord/mamacord doctor
```

### Update Path 2: Build Elsewhere

Run on `BUILD HOST`, inside `REPO CHECKOUT`.

64-bit target:

```bash
git pull --ff-only
GOOS=linux GOARCH=arm64 ./scripts/build-release.sh dist/mamacord-linux-arm64
rsync -a ./dist/mamacord-linux-arm64 USER@TARGET_HOST:~/mamacord
```

32-bit target:

```bash
git pull --ff-only
GOOS=linux GOARCH=arm GOARM=7 ./scripts/build-release.sh dist/mamacord-linux-armv7
rsync -a ./dist/mamacord-linux-armv7 USER@TARGET_HOST:~/mamacord
```

Run on `TARGET DEVICE`.

```bash
sudo install -Dm755 ~/mamacord /opt/mamacord/mamacord
sudo systemctl restart mamacord
/opt/mamacord/mamacord doctor
```

## Environment Examples

Only these env filenames are supported:

- `.env.dev`
- `.env.prod`

### Profile A

```dotenv
DISCORD_TOKEN=your-token-here

MAMACORD_PROD_MODE=1
MAMACORD_ALLOW_UNSIGNED_PLUGINS=0
# MAMACORD_TRUSTED_KEYS_FILE=./config/trusted_keys.json
```

### Profile B

```dotenv
DISCORD_TOKEN=your-token-here

MAMACORD_PROD_MODE=1
MAMACORD_ALLOW_UNSIGNED_PLUGINS=0
# MAMACORD_TRUSTED_KEYS_FILE=./config/trusted_keys.json

MAMACORD_ADMIN_ADDR=0.0.0.0:8081
MAMACORD_DASHBOARD_CLIENT_ID=your-discord-client-id
MAMACORD_DASHBOARD_CLIENT_SECRET=your-discord-client-secret
MAMACORD_DASHBOARD_SESSION_SECRET=use-at-least-32-characters-here

MAMACORD_PUBLIC_DASHBOARD_ORIGIN=https://example.com
MAMACORD_PUBLIC_API_ORIGIN=https://api.example.com
MAMACORD_DASHBOARD_ALLOWED_ORIGINS=https://example.com
```

### Profile C

```dotenv
DISCORD_TOKEN=your-token-here

MAMACORD_PROD_MODE=1
MAMACORD_ALLOW_UNSIGNED_PLUGINS=0
# MAMACORD_TRUSTED_KEYS_FILE=./config/trusted_keys.json

MAMACORD_ADMIN_ADDR=0.0.0.0:8081
MAMACORD_DASHBOARD_CLIENT_ID=your-discord-client-id
MAMACORD_DASHBOARD_CLIENT_SECRET=your-discord-client-secret
MAMACORD_DASHBOARD_SESSION_SECRET=use-at-least-32-characters-here

MAMACORD_PUBLIC_DASHBOARD_ORIGIN=https://device-or-domain.example
MAMACORD_PUBLIC_API_ORIGIN=https://device-or-domain.example
MAMACORD_DASHBOARD_ALLOWED_ORIGINS=https://device-or-domain.example
```

## Production Plugin Signing

This section matters whenever all of these are true:

- you are using `.env.prod`
- `MAMACORD_PROD_MODE=1`
- `MAMACORD_ALLOW_UNSIGNED_PLUGINS=0`

If that is your setup, the service will not boot unless trusted signer setup is complete.

### Case 1: You Only Want The Bundled Official Plugins

Good news:

- the bundled plugins are already signed
- the matching trusted public key file is `./config/trusted_keys.json`

What must exist on the installed machine:

- `/opt/mamacord/config/trusted_keys.json`
- `/opt/mamacord/plugins/<plugin>/signature.json` for the bundled plugins

Run on `TARGET DEVICE`:

```bash
ls -la /opt/mamacord/config/trusted_keys.json
find /opt/mamacord/plugins -maxdepth 2 -name signature.json -print | sort
/opt/mamacord/mamacord doctor
```

You want `doctor` to show:

- `prod_mode: true`
- `allow_unsigned_plugins: false`
- `trusted_keys_file_exists: true`
- `trusted_keys_count_file: 1` or higher

If `config/trusted_keys.json` is missing from the installed machine, copy it from the repo checkout:

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`:

```bash
sudo install -Dm644 ./config/trusted_keys.json /opt/mamacord/config/trusted_keys.json
sudo chown mamacord:mamacord /opt/mamacord/config/trusted_keys.json
```

### Case 2: You Want To Sign Your Own Plugins

Run on the machine where your repo checkout lives.

Generate a signer:

```bash
go run ./cmd/mamacord gen-signing-key --key-id your-key-id
```

Default outputs:

- private key: `./data/keys/your-key-id.key`
- trusted public key entry: `./config/trusted_keys.json`

Sign your plugin:

```bash
go run ./cmd/mamacord sign-plugin --dir ./plugins/<id> --key-id your-key-id --private-key-file ./data/keys/your-key-id.key
```

That creates:

- `./plugins/<id>/signature.json`

Then install or copy these onto the target device:

- `config/trusted_keys.json`
- `plugins/<id>/signature.json`
- the rest of the plugin directory

You do not need to copy the private key to the target unless you want the dashboard to sign plugins there.

### Optional: Let The Dashboard Sign Plugins Too

If you want plugin scaffolding in the dashboard to sign automatically, set these in `.env.prod`:

```dotenv
MAMACORD_DASHBOARD_SIGNING_KEY_ID=your-key-id
MAMACORD_DASHBOARD_SIGNING_KEY_FILE=./data/keys/your-key-id.key
```

That private key file must exist on the installed machine if you enable dashboard signing.

## Dashboard Build Guidance

This only matters for Profile C.

Main rule:

- weak boards should not be your normal frontend build machine
- build the dashboard elsewhere when possible
- copy built files onto the board

Run on the machine that has the repo checkout you want to build from.

Input expected before command:

- `apps/dashboard/` exists

Output after command:

- `apps/dashboard/dist/`

```bash
cd apps/dashboard
bun install
bun run build
```

At runtime:

- the admin API serves `apps/dashboard/dist` automatically if it exists
- Bun is not needed to serve the production dashboard

## Troubleshooting

### `install: cannot stat './dist/mamacord'`

That means one of these is true:

- the build did not finish successfully
- you are not in the repo checkout you think you are
- `./dist/mamacord` was never created

Run on the machine where you expect the build output:

```bash
pwd
ls -la
ls -la ./dist
find . -maxdepth 3 -type f -name 'mamacord*'
```

If you built locally on the board with `./scripts/build-release.sh`, the expected file is:

- `./dist/mamacord`

If you cross-built with an explicit output path, the expected file is:

- exactly the filename you passed to `./scripts/build-release.sh`

### `rsync: change_dir ... ~/migrations failed`

That means you used a home-directory path in a repo-local workflow.

If you are already on the board and inside the repo checkout, use:

- `./migrations/`
- `./locales/`
- `./plugins/`
- `./config/`

Do not use:

- `~/migrations/`
- `~/locales/`

unless you explicitly copied those directories into your home directory first.

### `rsync` To `/opt/mamacord/...` Fails

That usually means one of these is true:

- `/opt/mamacord` does not exist yet
- your SSH user cannot write there directly

Safe rule:

- copy to a user-writable path first
- then use `sudo install` or `sudo rsync` on the target device

### `systemctl restart mamacord` Says Unit Not Found

That means the first-install service setup was never completed.

You still need to:

- create `/etc/systemd/system/mamacord.service`
- run `sudo systemctl daemon-reload`
- run `sudo systemctl enable --now mamacord`

### `doctor` Says `discord_token: false`

Check these in order:

- does `/opt/mamacord/.env.prod` exist
- are you running `/opt/mamacord/mamacord doctor`
- does `.env.prod` actually contain `DISCORD_TOKEN=...`
- did you install `.env.prod` into `/opt/mamacord/.env.prod`

The installed production check is:

```bash
/opt/mamacord/mamacord doctor
```

### Prod Mode Says Trusted Signers Are Missing

If logs say the service needs a trusted signer:

- run `/opt/mamacord/mamacord doctor`
- check `trusted_keys_file`
- check `trusted_keys_file_exists`
- make sure `/opt/mamacord/config/trusted_keys.json` exists

If you only use the bundled plugins, the usual fix is:

Run on `TARGET DEVICE`, inside `REPO CHECKOUT`:

```bash
sudo install -Dm644 ./config/trusted_keys.json /opt/mamacord/config/trusted_keys.json
sudo chown mamacord:mamacord /opt/mamacord/config/trusted_keys.json
sudo systemctl restart mamacord
```

If you are signing your own plugins, use [Production Plugin Signing](#production-plugin-signing).

### `compile: signal: killed`

That usually means the board ran out of memory during compile.

Most common fix:

- stop native release building on that board
- cross-build on a stronger machine instead

For boards like Pi Zero 2 W, that should be your normal expectation.

### `exec format error`

You built for the wrong target architecture.

Check whether the device OS is:

- `linux/arm64`
- `linux/arm`

### Dashboard Does Not Load

If you use Profile C, make sure this exists on the target:

- `/opt/mamacord/apps/dashboard/dist/index.html`

### Admin API In Prod Fails At Startup

If `MAMACORD_ADMIN_ADDR` is set in prod mode, make sure the required dashboard OAuth, session, and public-origin vars are all set.
