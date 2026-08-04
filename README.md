# Home Server Backend Daemon

## 🧠 App Overview & Function
The Home Server Backend Daemon is a lightweight, high-performance service written in Go designed to run on Linux servers (such as Ubuntu Server). Its primary purpose is to act as a secure, centralized control bridge that allows you to manage and monitor your home server remotely via a companion mobile app or client interface.

### Key Capabilities:
* **Live System Monitoring:** Streams real-time updates for CPU usage, RAM utilization, and disk storage metrics using Server-Sent Events (SSE).
* **Comprehensive File Management:** Provides a full API suite to browse directory structures, view file metadata, read or edit text files, create/delete directories and files, and handle multipart file uploads.
* **Inter-Server File Transfers:** Enables direct server-to-server file push operations across your private network topology.
* **Secure Networking:** Integrates with Tailscale to ensure encrypted, hassle-free remote connectivity without exposing your home network to public security risks.

---

## 🚀 Automated Installation

You can install, configure, and run the backend daemon on your Linux server with a single automated command. The install script automatically handles system dependencies, sets up Go, configures Tailscale, clones the backend repository into `/opt/homeserver`, builds the binary, and registers/starts it as a background `systemd` service.

Run the following command on your server:

```bash
curl -fsSL [https://raw.githubusercontent.com/thedumbguy1234567891011-svg/home-server-backend/main/install.sh](https://gist.github.com/thedumbguy1234567891011-svg/dfcaa5cd5aa150bfa32d50be3fdfff8f) | sudo bash
```

---

## 📡 API Endpoints

Once running, the server listens on port `8080`:

* **`GET /api/status/live`** — Server-Sent Events (SSE) stream providing real-time JSON metrics (`os`, `cpu_usage`, `ram_usage`, `disk_usage`).
* **`GET /api/files?path=/`** — JSON directory listing and file metadata.
* **`POST /api/files/create-dir?path=/...`** — Creates a new directory.
* **`DELETE /api/files/delete?path=/...`** — Deletes a file or folder.
* **`GET/POST /api/files/content?path=/...`** — Reads or modifies text file contents.
* **`POST /api/files/upload?path=/...`** — Handles multipart file uploads.
* **`POST /api/files/transfer`** — Triggers a direct server-to-server file push instruction payload.
