# DEPLOY — RP-Management production setup, from zero

Two provisioning paths, one shared deploy:

- **Guide A** — Ubuntu Server **VM inside a Windows PC** (current plan).
- **Guide B** — Ubuntu Server on a **dedicated PC** (bare metal).
- **Part 3** — the actual deploy. Both guides land here; every step from Part 3 on is identical.

Everything below assumes zero prior setup: no Cloudflare account, no domain, clean machines. Commands are copy-pasteable. `<angle-brackets>` mark values you substitute.

**What you need before starting**
- The Windows PC (Guide A) or a spare PC (Guide B), always-on, wired ethernet strongly preferred.
- A USB stick ≥ 4 GB (only for Guide B, or Guide A on VirtualBox).
- ~US$10/year for a domain (Part 3.7 — needed for the public URL; the app works on the shop LAN without it).
- This repo pushed to GitHub (the server clones it).

---

## Guide A — Ubuntu Server VM on a Windows PC

### A.1 Check the Windows edition

Press `Win+Pause` (or Settings → System → About). Under "Windows specifications":

- **Windows 10/11 Pro, Enterprise or Education** → use **Hyper-V** (built in, runs as a system service before login — the VM comes back after power cuts with nobody touching the PC). Continue with A.2.
- **Windows Home** → Hyper-V isn't available. Two options: upgrade to Pro (one-time license), or use VirtualBox (works, but auto-start after reboot depends on a logged-in session — configure Windows auto-login + a Task Scheduler entry, and accept it's clunkier). The VirtualBox path is the same VM specs as A.4; the rest of this guide assumes Hyper-V.

### A.2 Configure the Windows host to behave like a server

Do all of these — each one is a way the app silently dies months later:

1. **Never sleep**: Settings → System → Power → Screen and sleep → "When plugged in, put my device to sleep" = **Never**. Also Control Panel → Power Options → choose "High performance".
2. **Windows Update active hours**: Settings → Windows Update → Advanced options → Active hours = the shop's opening hours (e.g. 09:00–20:00). Windows will still reboot for updates, but at night; the whole stack auto-recovers.
3. **Auto-start after power failure**: enter the PC's BIOS/UEFI (usually `Del` or `F2` at boot) → Power settings → "Restore on AC Power Loss" = **Power On**. This is the difference between "the shop lost power at 3 AM and everything is fine" and "call Santi".
4. Optional but recommended: rename the PC to something recognizable (e.g. `RP-SERVER-HOST`).

### A.3 Enable Hyper-V

PowerShell **as Administrator**:

```powershell
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
```

Reboot when prompted. "Hyper-V Manager" now appears in the Start menu.

### A.4 Download Ubuntu Server

On the Windows PC, download the **Ubuntu Server 24.04 LTS** ISO from <https://ubuntu.com/download/server> (~2.6 GB). LTS matters: security updates until 2029, no OS upgrades to babysit.

### A.5 Create the VM

In **Hyper-V Manager**:

1. Right panel → **New → Virtual Machine…**
   - Name: `rp-server`.
   - **Generation 2**.
   - Startup memory: **4096 MB**, **uncheck** "Use Dynamic Memory" (Postgres prefers stable memory).
   - Networking: connect to the **Default Switch** for now (we fix networking in A.7).
   - Virtual hard disk: **60 GB** (the app is tiny; photos are what grows).
   - Installation options: "Install an operating system from a bootable image file" → pick the Ubuntu ISO.
2. Before starting it, right-click the VM → **Settings…**:
   - **Security** → Secure Boot template = **Microsoft UEFI Certificate Authority** (Ubuntu won't boot with the default Windows template).
   - **Processor** → 2 virtual processors.
   - **SCSI Controller → Add → Hard Drive → New** → create a second disk, **100 GB**, name it `rp-backup.vhdx`. **If the PC has a second physical drive (D:), put this file there** — that's what makes the backup survive a disk failure. Single drive? Create it anyway (protects against app-level mistakes) and treat the off-site backup in Part 3.10 as mandatory.
   - **Automatic Start Action** = **Always start this virtual machine automatically**, delay 10 s.
   - **Automatic Stop Action** = **Shut down the guest operating system**.
   - **Checkpoints** → disable automatic checkpoints (they silently eat disk under a database).

### A.6 Install Ubuntu Server

Start the VM (double-click → Start). In the installer:

1. Language: **English** (error messages become googleable; the app itself is in Spanish).
2. Keyboard: **Spanish (Latin American)** — or whatever the physical keyboard is.
3. Type: **Ubuntu Server** (not minimized).
4. Network: note the IP it gets via DHCP (e.g. `172.x.x.x` on Default Switch — temporary, fixed in A.7).
5. Storage: **Use an entire disk** → pick the **60 GB** disk (NOT the 100 GB backup disk — leave it untouched; we format it in Part 3.2). Accept defaults.
6. Profile:
   - Your name: `Santi`
   - Server name: `rp-server`
   - Username: `santi`
   - A strong password (this is the SSH/sudo login — save it in a password manager).
7. **Check "Install OpenSSH server"** — everything after this happens over SSH.
8. Skip the featured snaps (Docker comes from apt in Part 3.3 — the snap version has volume quirks).
9. Reboot when the installer finishes ("Remove installation medium": Hyper-V detaches the ISO automatically — just press Enter).

### A.7 Give the VM a LAN address (External Switch)

The Default Switch NATs the VM behind the Windows host — fine for the tunnel, useless for "open `http://rp-server` from the shop's other PC". Put the VM directly on the shop LAN:

1. Hyper-V Manager → right panel → **Virtual Switch Manager…** → New virtual network switch → **External** → Create.
   - Name: `LAN`.
   - External network: pick the PC's **wired ethernet adapter**. Keep "Allow management operating system to share this network adapter" checked.
2. VM Settings → Network Adapter → Virtual switch = **LAN**.
3. Inside the VM (login at the Hyper-V console): `ip a` → it now has an address from the shop router (e.g. `192.168.0.x`).
4. **Reserve that IP in the router** (router admin page → DHCP → static leases → bind the VM's MAC to the IP). This is nicer than static IPs in Ubuntu: the network config lives in one place.

From now on, work from your own machine:

```bash
ssh santi@<vm-ip>
```

→ **Continue with Part 3.**

---

## Guide B — Ubuntu Server on a dedicated PC (bare metal)

### B.1 Make the install USB

On any PC, download **Ubuntu Server 24.04 LTS** (<https://ubuntu.com/download/server>) and **Rufus** (<https://rufus.ie>). Rufus → pick the USB stick → pick the ISO → Start (defaults are fine). On Linux: `sudo dd if=ubuntu-24.04-live-server-amd64.iso of=/dev/sdX bs=4M status=progress` with the stick's device.

### B.2 BIOS prep

Boot the server PC into BIOS/UEFI (`Del`/`F2`/`F12`):

- Boot order: USB first (just for the install).
- **"Restore on AC Power Loss" = Power On** (same reasoning as A.2.3 — non-negotiable for a shop server).
- Disable Secure Boot only if the installer refuses to boot (Ubuntu is usually signed and fine).

### B.3 Install Ubuntu

Boot from the USB and follow **exactly the steps of A.6**, with one difference in storage: if the PC has **two disks**, install Ubuntu on the first and leave the second untouched (it becomes `/mnt/backup` in Part 3.2). One disk only? Proceed — and treat the off-site backup (3.10) as mandatory.

### B.4 Server behavior

After the first boot, stop the machine from ever sleeping (a desktop-turned-server sometimes has lid/idle policies):

```bash
sudo systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target
```

Reserve the PC's IP in the router (as in A.7.4), then work over SSH:

```bash
ssh santi@<server-ip>
```

→ **Continue with Part 3.**

---

## Part 3 — The deploy (identical for VM and bare metal)

All commands run **on the server over SSH** unless labeled otherwise.

### 3.1 Base system

```bash
sudo apt update && sudo apt upgrade -y

# Argentina timezone — transaction dates and the nightly jobs depend on sane time
sudo timedatectl set-timezone America/Argentina/Buenos_Aires
timedatectl   # verify: Time zone: America/Argentina/Buenos_Aires, "System clock synchronized: yes"

# Automatic security patches (Ubuntu handles kernel/ssl CVEs by itself)
sudo apt install -y unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades   # answer "Yes"
```

### 3.2 Mount the backup disk

Identify the second (backup) disk — it's the one with no partitions:

```bash
lsblk
# NAME    SIZE  TYPE MOUNTPOINTS
# sda      60G  disk           ← Ubuntu (has partitions)
# sdb     100G  disk           ← backup disk, empty
```

Format and mount it at `/mnt/backup` (**double-check the device name** — this erases the disk):

```bash
sudo mkfs.ext4 -L rp-backup /dev/sdb
sudo mkdir -p /mnt/backup
echo 'LABEL=rp-backup /mnt/backup ext4 defaults,nofail 0 2' | sudo tee -a /etc/fstab
sudo systemctl daemon-reload && sudo mount -a
df -h /mnt/backup   # should show ~100G mounted
```

(`nofail` means the server still boots if the disk ever dies — the app keeps working, only backups stop.)

### 3.3 Install Docker

```bash
sudo apt install -y docker.io docker-compose-v2 git
sudo usermod -aG docker $USER
# apply the group without relogging:
newgrp docker
docker run --rm hello-world   # sanity check
```

### 3.4 Clone the repo and create `.env`

```bash
sudo mkdir -p /opt/rp-management && sudo chown $USER: /opt/rp-management
git clone https://github.com/santiguti/rp-management.git /opt/rp-management
cd /opt/rp-management
cp .env.example .env
```

Generate the two secrets:

```bash
head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n'; echo   # → POSTGRES_PASSWORD
head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'; echo   # → COOKIE_SECRET
```

Edit (`nano .env`) to exactly this shape (substitute the generated values):

```ini
APP_ENV=prod
HTTP_ADDR=:8080
RP_ATTACHMENTS_DIR=/app/data/attachments

DATABASE_URL=postgres://rp:<POSTGRES_PASSWORD>@postgres:5432/rp?sslmode=disable

POSTGRES_USER=rp
POSTGRES_DB=rp
POSTGRES_PASSWORD=<generated 32-hex>

COOKIE_SECRET=<generated 64-hex>

TUNNEL_TOKEN=
```

Notes: the host in `DATABASE_URL` is `postgres` (the compose service name, not localhost). The password appears twice — literally inside `DATABASE_URL` and in `POSTGRES_PASSWORD` — and must match. `TUNNEL_TOKEN` stays empty until 3.7.

### 3.5 Build and start the stack

```bash
cd /opt/rp-management/deploy
docker compose build          # first build ~5-10 min (downloads Go/Node images)
docker compose up -d
docker compose ps             # all services Up; cloudflared restarting is EXPECTED (no token yet)
```

Run the migrations (the goose binary and the migration files ship inside the api image):

```bash
docker compose exec api sh -c '/app/goose -dir /app/migrations postgres "$DATABASE_URL" up'
docker compose exec api sh -c '/app/goose -dir /app/migrations postgres "$DATABASE_URL" status'
# Expect: 0001 … 0009 all applied
```

Health check:

```bash
curl -s http://localhost/healthz    # → {"status":"ok"}   (via Caddy)
```

### 3.6 Create the users

**The owner (your friend — full access):**

```bash
docker compose exec api /app/jobs seed-owner \
  --username lucas --password '<strong-password>' --full-name 'Lucas <Apellido>'
```

**An employee** (day-to-day work; can't touch parts catalog, gastos fijos, importación, bitácora): create with `seed-owner`, then downgrade the role — v1 has no user-management UI by design:

```bash
docker compose exec api /app/jobs seed-owner \
  --username empleado1 --password '<password>' --full-name 'Nombre Empleado'
docker compose exec postgres psql -U rp -d rp \
  -c "UPDATE rp.users SET role='employee' WHERE username='empleado1';"
```

**Forgot a password** (this WILL happen):

```bash
docker compose exec api /app/jobs set-password --username lucas --password '<new-password>'
```

**Promote/demote later**: same `UPDATE rp.users SET role='owner'|'employee' WHERE username='…';` via psql.

**LAN test — do this now**: from any device on the shop network, open `http://<server-ip>` → login page → log in as the owner. This works with zero internet; it's also the fallback if Cloudflare is ever down.

### 3.7 Cloudflare Tunnel — public HTTPS access, from zero

You need this for access from outside the shop (and later for the v2 WhatsApp webhook). It requires a domain in a Cloudflare account.

**a) Create the Cloudflare account** (free plan is enough — laptop, not server):
Go to <https://dash.cloudflare.com/sign-up>, register, verify the email.

**b) Get a domain.** Two paths:

- **Easiest — buy inside Cloudflare**: dash → **Domain Registration → Register Domains** → search something like `rp-taller.com` (~US$10/year, at-cost pricing, auto-configured, zero extra steps).
- **Already have / prefer another registrar** (Namecheap, nic.ar for `.com.ar`, …): buy it there, then in Cloudflare dash → **Add a domain** → enter it → free plan → Cloudflare shows two nameservers (e.g. `ana.ns.cloudflare.com`) → set those at the registrar → wait for "Active" (minutes to hours). `.com.ar` via nic.ar is cheap in ARS but needs a CUIT/CUIL and its nameserver panel is clunkier — the US$10 `.com` is less friction.

**c) Create the tunnel**:

1. Dash → **Zero Trust** (left menu; pick any team name, free plan).
2. **Networks → Tunnels → Create a tunnel** → type **Cloudflared** → name it `rp-shop`.
3. The next screen shows install commands containing the token — **only copy the token** (the long string after `--token`, starts with `eyJ`). The container is already in our compose file.
4. On the server:

```bash
nano /opt/rp-management/.env     # TUNNEL_TOKEN=eyJ...
cd /opt/rp-management/deploy
docker compose up -d cloudflared
docker compose logs cloudflared | tail -5   # expect "Registered tunnel connection" ×4
```

5. Back in the Cloudflare wizard the connector shows **Connected**. Next → **Route tunnel**:
   - Public hostname: `rp` · domain: `<your-domain>` → `rp.rp-taller.com`
   - Service: **HTTP** → `caddy:80` *(container-to-container inside the compose network)*
   - Save.

**d) Test**: from a phone **on mobile data** (not shop wifi), open `https://rp.<your-domain>` → green-lock login page. TLS is automatic — Cloudflare's edge handles the certificate; nothing to renew, ever.

**e) Optional hardening**: Zero Trust → Access → Applications can put a Cloudflare login wall in front of the whole hostname. Skip for v1 — the app has its own auth and rate-limited login — but know it exists.

### 3.8 Scheduled jobs (cron)

```bash
sudo crontab -e
```

```cron
# RP-Management (paths assume /opt/rp-management)
0  2 * * * cd /opt/rp-management/deploy && docker compose exec -T api /app/jobs run-recurring    >> /var/log/rp-jobs.log 2>&1
10 2 * * * cd /opt/rp-management/deploy && docker compose exec -T api /app/jobs cleanup-sessions >> /var/log/rp-jobs.log 2>&1
30 2 * * * /opt/rp-management/deploy/backup.sh                                                   >> /var/log/rp-backup.log 2>&1
```

Dry-run the backup right now instead of discovering a typo at 02:30:

```bash
sudo /opt/rp-management/deploy/backup.sh
ls -lh /mnt/backup/rp/$(date +%F)/    # db.pgcustom (>1 KiB) + attachments.tar.gz
```

### 3.9 Reboot drill — the most important test

```bash
sudo reboot
```

**Guide A**: also do a full drill — shut down Windows itself, power it back on, **touch nothing**. Within ~3 minutes of the Windows desktop appearing: `http://<server-ip>` works (Hyper-V auto-started the VM, systemd started Docker, `restart: unless-stopped` brought every container back, cloudflared re-established the tunnel). If this drill passes, power cuts are a non-event.

### 3.10 Off-site backup (strongly recommended — do it the first week)

The backup disk protects against app mistakes and (if on a second physical drive) one disk dying. It does not protect against theft, fire, or ransomware on the Windows host. Backblaze B2's free tier (10 GB) covers years of this app's dumps:

1. Create a B2 account → one private bucket (`rp-backups`) → an app key.
2. `sudo apt install -y rclone && rclone config` (remote type `b2`).
3. Append to the backup script or add a 4th cron line:
   ```cron
   45 2 * * * rclone sync /mnt/backup/rp b2:rp-backups/rp >> /var/log/rp-backup.log 2>&1
   ```

### 3.11 Final checklist

- [ ] `docker compose ps` → 5 services Up (postgres, api, web, caddy, cloudflared)
- [ ] `curl http://localhost/healthz` → `{"status":"ok"}`
- [ ] `http://<server-ip>` from another shop device → login works (LAN path)
- [ ] `https://rp.<domain>` on mobile data → login works (tunnel path)
- [ ] Owner + employee users exist; employee sees no "Ajustes" section
- [ ] `goose status` shows 0001–0009 applied
- [ ] `/mnt/backup/rp/<today>/` has a dump > 1 KiB and the attachments tarball
- [ ] Reboot drill passed (3.9) — including the full Windows power-off drill on Guide A
- [ ] BIOS "Restore on AC Power Loss" = Power On (both guides)

---

## Runbook — operating it after day one

**Update the app** (after Santi pushes to GitHub):
```bash
cd /opt/rp-management && git pull
cd deploy && docker compose build && docker compose up -d
docker compose exec api sh -c '/app/goose -dir /app/migrations postgres "$DATABASE_URL" up'
```
~30 s of downtime; sessions survive (they live in Postgres).

**Logs**: `docker compose logs -f api` (or `caddy`, `cloudflared`, `postgres`).

**Restore from backup** (disaster recovery, onto a freshly deployed stack):
```bash
cd /opt/rp-management/deploy
docker compose exec -T postgres pg_restore -U rp -d rp --clean --if-exists < /mnt/backup/rp/<date>/db.pgcustom
docker run --rm -v rp_attachments:/data -v /mnt/backup/rp/<date>:/backup alpine \
  tar -xzf /backup/attachments.tar.gz -C /data
docker compose restart api
```

**Common failures**:
| Symptom | Fix |
|---|---|
| Public URL down, LAN fine | `docker compose logs cloudflared` — internet cut or token revoked; LAN keeps the shop working meanwhile |
| Everything down after a storm | Windows PC off → BIOS power setting (A.2.3/B.2); VM didn't start → Hyper-V Automatic Start Action (A.5) |
| "Stock insuficiente" that looks wrong | inventory drifted from reality → register an `Ajuste` movement on the part |
| Disk filling up | `docker system prune -f` (old build layers); check `/mnt/backup` retention ran |
| Forgot the owner password | `set-password` (3.6) |
