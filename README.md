# Go Custom Cloudflare DDNS 🚀

A lightning-fast, zero-dependency Dynamic DNS (DDNS) updater for Cloudflare, written purely in Go. 

It comes with a built-in interactive setup (TUI) and an automated `systemd` installer, making it perfect for Debian-based servers. Just run the one-line install command, pick your Cloudflare zone, and let it run quietly in the background every 5 minutes.

## ✨ Features
* **Zero Dependencies:** Runs purely on the Go standard library, resulting in a single, lightweight binary.
* **Interactive Setup:** Built-in CLI tool to easily select your Cloudflare zones without manually hunting for Zone IDs.
* **Systemd Native:** Automatically configures a `oneshot` service and timer (no messy cron jobs!).
* **Smart Updates:** Caches your old IP locally and only pings the Cloudflare API when your IP actually changes.
* **Multi-Arch:** Automatically built for Linux (amd64/arm64), macOS, and Windows.

## 🚀 One-Line Installation

You can install and configure the DDNS updater in seconds. This script will fetch the latest binary, launch the interactive setup, and configure the background timer.

```bash
curl -sSL https://raw.githubusercontent.com/vuthanhtrung2010/go-custom-ddns/master/install.sh | sudo bash
```