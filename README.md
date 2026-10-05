# Master Panel Backend API (Go-Fiber)

Backend REST API untuk **Master Panel** menggunakan framework **Go-Fiber**, dirancang untuk performa tinggi (*ultra-fast response* < 5ms), efisiensi memori RAM rendah (~15-30 MB), serta arsitektur **Dual-Database** (Admin PostgreSQL + Dynamic Tenant MySQL).

---

## 🏗️ Arsitektur & Konsep Database

Sistem ini memisahkan secara tegas akses database antara **Admin / Backoffice** dan **Player / Operasional**:

```
                       ┌─────────────────────────┐
                       │   Master Panel Client   │
                       │    (Svelte Frontend)    │
                       └────────────┬────────────┘
                                    │
                                    ▼
                       ┌─────────────────────────┐
                       │  Go-Fiber Backend API   │
                       └─────┬─────────────┬─────┘
                             │             │
              (Primary DB)   │             │  (Dynamic Multi-Tenant)
                             ▼             ▼
┌─────────────────────────────────┐   ┌─────────────────────────────────┐
│     PostgreSQL (engine-auth)    │   │      MySQL Tenant Database      │
│ ------------------------------- │   │ ------------------------------- │
│ • users (admin/backoffice users)│   │ • users (players data)          │
│ • agent_master & agent          │   │ • transactions, deposits, etc.  │
│ • agent_connection (MySQL creds)│   │                                 │
│ • master_site_configs (routing) │   │ (Koneksi dibuka dinamis lewat   │
└─────────────────────────────────┘   │  kredensial di agent_connection)│
                                      └─────────────────────────────────┘
```

1. **Admin Database (PostgreSQL - `engine-auth`)**:
   * Menyimpan akun admin, master agent, daftar agen, dan pengaturan role.
   * Tabel `agent_connection` & `agent_connection_read` menyimpan kredensial database MySQL untuk masing-masing agen (`db_host`, `db_name`, `db_user`, `db_password`, `db_port`).
   * Tabel `master_site_configs` menyimpan status domain aktif, daftar backup domain, dan status maintenance agen.

2. **Player Database (Dynamic MySQL Tenant Pool)**:
   * Menggunakan connection pool manager dinamis di Go (`database/tenant_db.go`).
   * Ketika request membutuhkan data player (contoh: saldo, total player, daftar member), backend otomatis mengambil kredensial agen dari PostgreSQL lalu membuat/memakai koneksi MySQL ber-cache thread-safe (`sync.Map`).

---

## 📁 Struktur Folder

```text
master-panel/backend/
├── cmd/
│   └── server/
│       └── main.go          # Entry point server Fiber
├── config/
│   ├── config.go            # Loader environment variables
│   └── config_test.go       # Unit test config
├── database/
│   ├── admin_db.go          # Koneksi GORM PostgreSQL (Admin DB)
│   └── tenant_db.go         # Dynamic MySQL Connection Pool Manager
├── handlers/
│   ├── agent_handler.go     # CRUD & Domain switcher per-agen
│   ├── auth_handler.go      # Login admin (Bcrypt) & info user
│   ├── player_handler.go    # Query data player ke DB MySQL agen
│   ├── worker_handler.go    # Sub-5ms API untuk Cloudflare Worker & APK
│   └── worker_handler_test.go
├── middleware/
│   ├── auth.go              # JWT Authentication
│   ├── auth_test.go         # Unit test JWT
│   ├── secret.go            # Gateway API secret verification
│   └── secret_test.go       # Unit test secret verification
├── models/
│   ├── admin_models.go      # Struct DB PostgreSQL
│   ├── models_test.go       # Unit test entity & table names
│   └── player_models.go     # Struct DB MySQL Tenant
├── routes/
│   └── routes.go            # Pendaftaran routing Fiber & CORS
├── .env                     # File environment lokal
└── go.mod & go.sum
```

---

## ⚙️ Persyaratan & Instalasi

### 1. Requirements
* **Go** versi `>= 1.23`
* Akses jaringan ke PostgreSQL Admin (`engine-auth`)

### 2. Konfigurasi `.env`
Salin template atau sesuaikan file `.env`:
```env
APP_PORT=8080
JWT_SECRET=your_access_token_secret_key
WORKER_API_SECRET=hrc-worker-secret-key-2026

# Admin PostgreSQL Database (engine-auth / backoffice)
ADMIN_DB_HOST=62.72.46.195
ADMIN_DB_PORT=2022
ADMIN_DB_USER=postgres
ADMIN_DB_PASS=postgres
ADMIN_DB_NAME=engine-auth
ADMIN_DB_SSLMODE=disable
```

### 3. Menjalankan Unit Tests
Jalankan seluruh test suite dengan:
```bash
go test -v ./...
```

### 4. Menjalankan Server Development
```bash
go run cmd/server/main.go
```

### 5. Build Binary Production
```bash
go build -o bin/server cmd/server/main.go
./bin/server
```

---

## 📡 Dokumentasi Endpoint API

Base URL: `http://localhost:8080`

### 1. Public Gateway & Health

#### • Health Check
* **Endpoint**: `GET /health`
* **Response**:
```json
{
  "service": "master-panel-api",
  "status": "healthy"
}
```

#### • Worker & APK Gateway Config (Sub-5ms)
* **Endpoint**: `GET /api/v1/gateway/config?agent=win`
* **Headers**: `X-Master-Secret: hrc-worker-secret-key-2026`
* **Deskripsi**: Digunakan oleh Cloudflare Worker & dynamic branding APK Android untuk mengambil domain aktif dan status pemeliharaan.
* **Response**:
```json
{
  "success": true,
  "agent_code": "win",
  "app_name": "Win Gaming",
  "active_domain": "https://active-domain.com",
  "backup_domains": [
    "https://backup-domain-1.com",
    "https://backup-domain-2.com"
  ],
  "is_maintenance": false,
  "maintenance_message": "Kami sedang melakukan pemeliharaan rutin. Silakan coba kembali nanti.",
  "logo_url": "https://assets.com/logo.png",
  "apk_download_url": "/app/master-release.apk",
  "timestamp": 1790703735
}
```

---

### 2. Authentication (Admin Master)

#### • Admin Login
* **Endpoint**: `POST /api/v1/auth/login`
* **Request Body**:
```json
{
  "email": "admin@example.com",
  "password": "yourpassword"
}
```
* **Response**:
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1Ni...",
  "user": {
    "uuid": "0401d4a5-...",
    "name": "Super Admin",
    "email": "admin@example.com",
    "is_master": true
  }
}
```

#### • Get Current User Profile
* **Endpoint**: `GET /api/v1/auth/me`
* **Headers**: `Authorization: Bearer <TOKEN>`

---

### 3. Agent Management & Domain Switcher

Semua endpoint berikut mewajibkan header: `Authorization: Bearer <TOKEN>`

#### • List Semua Agen
* **Endpoint**: `GET /api/v1/agents?status=active` (atau `?status=inactive`, `?status=maintenance`)
* **Lazy-Loading & Query Scope**:
  * `?status=active`: Hanya mengambil agen dengan status **`is_active = true`** (dipakai saat initial load dashboard agar query sangat ringan).
  * `?status=inactive`: Hanya mengambil agen dengan status **`is_active = false`** saat tombol Inactive diklik.
  * Dilengkapi **caching (Redis / Memory)** ber-TTL 3 menit, di-invalidate otomatis saat ada perubahan domain atau maintenance.
* **Hak Akses & Scoping**:
  * User dengan `master_agent_id = 1` (atau `is_master = true`) dapat melihat **seluruh daftar agen** yang ada di database.
  * User dengan `master_agent_id != 1` hanya dapat melihat daftar agen yang berada di bawah `master_agent_id` miliknya sendiri.
* **Response**: Array agen beserta konfigurasi domain dan koneksi database.

#### • Detail Agen
* **Endpoint**: `GET /api/v1/agents/:uuid`

#### • Ganti Domain Instan (Instant Domain Switch)
* **Endpoint**: `POST /api/v1/agents/:agent_code/switch-domain`
* **Request Body**:
```json
{
  "domain": "https://domain-baru.com"
}
```

#### • 1-Click Toggle Maintenance Mode
* **Endpoint**: `POST /api/v1/agents/:agent_code/maintenance`
* **Request Body**:
```json
{
  "is_maintenance": true,
  "message": "Situs sedang dalam maintenance terjadwal hingga pukul 03:00 WIB"
}
```

#### • Bulk Maintenance (All Database / Selected Agents)
* **Endpoint**: `POST /api/v1/agents/bulk-maintenance`
* **Hak Akses & Scoping**:
  * User dengan `master_agent_id = 1` (atau `is_master = true`) dapat melakukan bulk maintenance terhadap **seluruh agen di database**.
  * User dengan `master_agent_id != 1` hanya dapat mengeksekusi bulk maintenance untuk agen-agen dalam lingkup `master_agent_id` miliknya.
* **Request Body**:
```json
{
  "is_maintenance": true,
  "message": "Seluruh server sedang dalam pemeliharaan darurat.",
  "agent_codes": [] // Kosongkan atau omit untuk menerapkan ke SEMUA agen dalam scope
}
```
* **Response**:
```json
{
  "success": true,
  "message": "Status pemeliharaan berhasil diaktifkan (Maintenance Mode) untuk 12 agen",
  "affected_count": 12,
  "is_maintenance": true
}
```


#### • Update Konfigurasi Lengkap Agen
* **Endpoint**: `POST /api/v1/agents/:agent_code/config`
* **Request Body**:
```json
{
  "active_domain": "https://main-win.com",
  "backup_domains": "https://win1.com,https://win2.com",
  "is_maintenance": false,
  "maintenance_message": "Website under maintenance",
  "app_name": "Win Gaming VIP",
  "logo_url": "https://domain.com/logo.png",
  "apk_download_url": "/app/master-release.apk"
}
```

---

### 4. Tenant Player Database (MySQL)

Endpoint ini mengakses database **MySQL player agen** secara dinamis:

#### • Ringkasan Player Agen
* **Endpoint**: `GET /api/v1/agents/:uuid/player-summary`
* **Headers**: `Authorization: Bearer <TOKEN>`
* **Response**:
```json
{
  "success": true,
  "data": {
    "total_players": 1420,
    "total_saldo": 258900000.50,
    "active_today": 0,
    "deposit_today": 0,
    "withdraw_today": 0
  }
}
```

#### • Daftar Member / Player Agen
* **Endpoint**: `GET /api/v1/agents/:uuid/players?page=1&limit=20&search=johndoe`
* **Headers**: `Authorization: Bearer <TOKEN>`
* **Response**:
```json
{
  "success": true,
  "data": [
    {
      "id": 105,
      "username": "johndoe",
      "extplayer": "WIN_105",
      "acc_name": "John Doe",
      "acc_number": "1234567890",
      "bank": "BCA",
      "saldo": 500000.00,
      "created_at": "2026-01-15T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1
  }
}
```

---

### 5. Laporan Winlose Multi-Agen (Lazy Loading)

Fitur ini dirancang khusus untuk memonitor **Turnover, Game Winlose (Profit), Deposit, dan Withdraw** semua tenant dengan arsitektur **Lazy-Loading per agen** agar koneksi database client/tenant tidak terbebani secara bersamaan.

#### • Daftar Agen Aktif untuk Laporan (dengan Pagination & Search)
* **Endpoint**: `GET /api/v1/reports/agents?page=1&limit=10&search=win`
* **Filter Status**: Otomatis memfilter hanya agen dengan status **`is_active = true`**.
* **Headers**: `Authorization: Bearer <TOKEN>`
* **Response**:
```json
{
  "success": true,
  "data": [
    {
      "uuid": "0401d4a5-...",
      "agent_code": "win",
      "name": "Win Gaming",
      "master_agent_id": 1,
      "is_active": true
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 1,
    "total_pages": 1
  }
}
```

#### • Laporan Winlose per Agen
* **Endpoint**: `GET /api/v1/reports/winlose/agent/:uuid?start_date=2026-09-01&end_date=2026-09-30`
* **Headers**: `Authorization: Bearer <TOKEN>`
* **Response**:
```json
{
  "success": true,
  "data": {
    "agent_uuid": "0401d4a5-...",
    "agent_code": "win",
    "agent_name": "Win Gaming",
    "start_date": "2026-09-01",
    "end_date": "2026-09-30",
    "total_deposit": 150000000,
    "deposit_count": 420,
    "total_withdraw": 85000000,
    "withdraw_count": 130,
    "financial_winlose": 65000000,
    "total_turnover": 540000000,
    "total_win": 510000000,
    "game_winlose": 30000000,
    "total_betround": 12500,
    "active_players_count": 340,
    "category_breakdown": [
      {
        "game_category": "slots",
        "total_betround": 10000,
        "turnover": 400000000,
        "win": 380000000,
        "profit": 20000000
      }
    ],
    "status": "ok",
    "cached": true
  }
}
```

#### • Laporan Pemain Aktif Bermain per Agen (Drill-Down)
* **Endpoint**: `GET /api/v1/reports/winlose/agent/:uuid/players?start_date=2026-09-01&end_date=2026-09-30&page=1&limit=15&search=johndoe&refresh=false`
* **Query Params**:
  * `start_date`, `end_date`: Format YYYY-MM-DD.
  * `page`, `limit`: Pagination untuk database tenant.
  * `search`: Filter username atau extplayer.
  * `refresh=true`: Bypass cache untuk langsung fetch data terbaru dari database tenant.
* **Headers**: `Authorization: Bearer <TOKEN>`
* **Response**:
```json
{
  "success": true,
  "data": {
    "agent_uuid": "0401d4a5-...",
    "agent_code": "win",
    "agent_name": "Win Gaming",
    "start_date": "2026-09-01",
    "end_date": "2026-09-30",
    "page": 1,
    "limit": 15,
    "total_items": 340,
    "total_pages": 23,
    "players": [
      {
        "user_id": 1001,
        "username": "johndoe",
        "extplayer": "WIN_1001",
        "saldo": 250000.00,
        "total_betround": 120,
        "turnover": 5000000.00,
        "win": 4600000.00,
        "profit": 400000.00
      }
    ],
    "cached": false
  }
}
```

#### • Strategi Caching (Redis + In-Memory Fallback)
* Dilengkapi dengan hybrid caching:
  * **Redis Server**: Dikonfigurasi via `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`.
  * **In-Memory Fallback (`sync.Map`)**: Otomatis aktif jika Redis tidak tersedia tanpa memutus alur aplikasi.
* **TTL Caching**:
  * Data Hari Ini (live): TTL 5 menit.
  * Data Historis (kemarin / masa lalu): TTL 2 jam.
* Force refresh tersedia lewat tombol reload atau query `refresh=true`.

```

