# zkill-bot

An EVE Online killmail monitor. It watches the [zKillboard](https://zkillboard.com) live feed, matches incoming kills against your rules, and posts notifications to Discord.

---

## Requirements

- [Go](https://go.dev) 1.25 or newer
- A `config.yaml` file — a ready-to-edit example is included

---

## 1. Install Go

### macOS

The easiest way is with [Homebrew](https://brew.sh). If you don't have Homebrew, install it first by pasting this into Terminal:

```
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

Then install Go:

```
brew install go
```

### Windows

1. Go to [go.dev/dl](https://go.dev/dl/)
2. Download the `.msi` installer for the latest version
3. Run it and follow the prompts
4. Open a new Command Prompt and verify it worked:

```
go version
```

### Linux

```
sudo apt install golang-go        # Debian / Ubuntu
sudo dnf install golang           # Fedora / RHEL
```

Or download directly from [go.dev/dl](https://go.dev/dl/) if your package manager has an older version.

---

## 2. Get the code

If you have Git installed:

```
git clone https://github.com/yourname/zkill-bot.git
cd zkill-bot
```

Or download and extract the zip from GitHub, then open a terminal in the extracted folder.

---

## 3. Configure

Open `config.yaml` in any text editor. The settings you'll most likely want to change are:

| Setting | What it does |
|---|---|
| `alert_webhook_url` | Discord webhook for startup/shutdown notifications |
| `rules` → `rules` | Your kill notification rules |

For help writing rules, see [RULES.md](RULES.md).

---

## 4. Build

From inside the project folder, run:

```
go build -o zkill-bot .
```

This creates a single executable file called `zkill-bot` (or `zkill-bot.exe` on Windows) in the current directory. You only need to rebuild when you update the code.

---

## 5. Run

```
./zkill-bot
```

On Windows:

```
zkill-bot.exe
```

The bot will start polling the live zKillboard feed immediately. You should see kill notifications printed to the terminal within a few seconds.

### Using a different config file

By default the bot looks for `config.yaml` in the same directory. To use a file somewhere else:

```
./zkill-bot -config /path/to/my-config.yaml
```

### Stopping

Press `Ctrl+C`. The bot saves its position in the feed before exiting, so it will resume from where it left off next time.

---

## 6. Keeping it running

If you want the bot to run continuously in the background, the simplest options are:

### macOS / Linux — screen

```
screen -S zkill-bot
./zkill-bot
```

Detach with `Ctrl+A` then `D`. Reattach later with `screen -r zkill-bot`.

### macOS / Linux — nohup

```
nohup ./zkill-bot > zkill-bot.log 2>&1 &
```

Logs go to `zkill-bot.log`. Find the process with `ps aux | grep zkill-bot`.

### Linux — systemd

Create `/etc/systemd/system/zkill-bot.service`:

```ini
[Unit]
Description=zkill-bot
After=network.target

[Service]
Type=simple
WorkingDirectory=/path/to/zkill-bot
ExecStart=/path/to/zkill-bot/zkill-bot
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Then enable and start it:

```
sudo systemctl enable zkill-bot
sudo systemctl start zkill-bot
sudo journalctl -u zkill-bot -f   # view logs
```

---

## Updating game data

All ship names, item names, and solar system names are compiled directly into the binary — no database file is needed at runtime. If CCP releases a patch that adds new items or systems, regenerate the data and rebuild:

```
go run ./cmd/gen-sde              # ship and item names from eve.db
go run ./cmd/gen-systems          # solar system names from ESI
go build -o zkill-bot .
```

`eve.db` is only needed to run these generators, not to run the bot itself.

---

## Running the tests

```
go test ./...
```
