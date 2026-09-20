# itsanla/bot

Platform background worker deterministik berbasis **Golang + SQLite + Docker** untuk mengotomasi notifikasi dan job terjadwal secara modular, hemat resource (~15MB RAM), dan 100% self-contained.

---

## 🏗 Arsitektur

```text
bot/
├── cmd/
│   └── bot/
│       └── main.go           # Entry point daemon & graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go         # Konfigurasi dari .env / environment variables
│   ├── db/
│   │   └── db.go             # SQLite engine (WAL mode, state tracking, deduplication)
│   ├── telegram/
│   │   └── client.go         # Telegram Bot API client (HTML formatting, auto-retry)
│   └── service/              # Modular service layer
│       ├── service.go        # Interface Service: Name(), Interval(), Run(ctx)
│       ├── manager.go        # Concurrent runner untuk semua service
│       └── activecollab/     # Service 1: ActiveCollab notification poller
│           ├── client.go     # HTTP client ke ActiveCollab API (WAF Cloudflare safe)
│           ├── models.go     # Struct JSON ActiveCollab
│           └── service.go    # Polling, parser event, dan dispatcher notifikasi
├── Dockerfile                # Multi-stage build (image ~20MB)
├── docker-compose.yml        # Docker compose dengan persistent volume ./data:/app/data
├── Makefile                  # Perintah build & run cepat
└── .env.example
```

---

## 🧩 Menambah Service Notifikasi Baru di Masa Depan

Setiap service baru cukup mengimplementasikan interface `service.Service`:

```go
type Service interface {
    Name() string
    Interval() time.Duration
    Run(ctx context.Context) error
}
```

Langkah:
1. Buat folder baru di `internal/service/<nama_service>/`.
2. Implementasikan logika polling / fetch data dan kirim via Telegram client.
3. Daftarkan di `cmd/bot/main.go`:
   ```go
   manager.Register(namaservice.NewService(...))
   ```
4. Service baru akan otomatis berjalan secara paralel di goroutine-nya sendiri tanpa mengganggu service lain.

---

## 🚀 Menjalankan Aplikasi

### Opsi 1: Menggunakan Docker Compose (Direkomendasikan di VPS)

```bash
# 1. Siapkan .env
cp .env.example .env
# Edit token & konfigurasi di .env

# 2. Build & jalankan di background
docker compose up -d --build

# 3. Cek log
docker compose logs -f
```

### Opsi 2: Menjalankan Binary Native (Lokal)

```bash
# Build binary
make build

# Jalankan
make run
```

---

## 🗄 Penyimpanan Data (SQLite)

Database disimpan di `./data/bot.db` (atau `/app/data/bot.db` di dalam container):
* `service_states`: Menyimpan checkpoint event terakhir per service.
* `notification_logs`: Mencegah duplikasi pesan (*deduplication*).
* `service_runs`: Riwayat eksekusi dan durasi tiap service.
