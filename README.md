# NabungYuk 💰

**Kelola uang, capai impian.**

NabungYuk adalah aplikasi web personal finance untuk mencatat transaksi, mengatur target tabungan, memantau laporan keuangan, dan mendapatkan pengingat menabung — lengkap dengan fitur scan struk otomatis (OCR) langsung dari browser.

## ✨ Fitur

- **Autentikasi** — Register & login dengan JWT, rate limiting pada endpoint login.
- **Dashboard** — Ringkasan saldo, pemasukan/pengeluaran, dan grafik interaktif.
- **Transaksi** — Catat pemasukan & pengeluaran, kategori, catatan, serta lampiran struk.
- **Scan Struk (OCR)** — Upload foto struk dan sistem otomatis mendeteksi nominal, tanggal, dan kategori menggunakan Tesseract.js langsung di browser (tanpa dikirim ke server pihak ketiga).
- **Target Tabungan** — Buat target tabungan dengan deadline, ikon, dan riwayat setoran.
- **Pengingat** — Atur pengingat menabung harian/mingguan/bulanan via email.
- **Laporan** — Rekap keuangan dan visualisasi grafik per periode.
- **Responsive UI** — Tampilan menyesuaikan desktop, tablet, dan mobile.

## 🛠️ Tech Stack

**Backend**
- [Go](https://go.dev/) 1.25
- [Gin](https://github.com/gin-gonic/gin) — HTTP web framework
- [GORM](https://gorm.io/) + MySQL driver — ORM & database
- [JWT (golang-jwt/jwt)](https://github.com/golang-jwt/jwt) — autentikasi
- [godotenv](https://github.com/joho/godotenv) — konfigurasi environment

**Frontend**
- HTML, JavaScript (vanilla)
- [Tailwind CSS](https://tailwindcss.com/) (via CDN)
- [Chart.js](https://www.chartjs.org/) — grafik dashboard & laporan
- [Tesseract.js](https://tesseract.projectnaptha.com/) — OCR scan struk di sisi browser

**Database**
- MySQL

## 📁 Struktur Proyek

```
.
├── config/          # Koneksi database & loader environment variable
├── controllers/      # Handler untuk auth, dashboard, transaksi, tabungan, reminder, laporan
├── middleware/        # Auth middleware (JWT) & rate limiter
├── models/            # Struct GORM: User, Transaction, SavingGoal, SavingDeposit, Reminder
├── routes/            # Definisi routing API
├── services/          # Email service & scheduler pengingat
├── migrations/        # SQL schema manual (opsional, GORM AutoMigrate juga berjalan)
├── static/            # Halaman frontend (HTML/CSS/JS)
├── uploads/receipts/  # Penyimpanan file struk yang diunggah
└── main.go            # Entry point aplikasi
```

## 🚀 Menjalankan Secara Lokal

### Prasyarat
- Go 1.25+
- MySQL 8+
- (Opsional) SMTP account untuk fitur email reminder, mis. Gmail App Password

### Langkah instalasi

1. **Clone repository**
   ```bash
   git clone https://github.com/<username>/nabungyuk.git
   cd nabungyuk
   ```

2. **Siapkan database MySQL**
   ```sql
   CREATE DATABASE nabungyuk;
   ```
   Skema tabel akan otomatis dibuat oleh GORM AutoMigrate saat aplikasi pertama kali dijalankan. Bila ingin membuat skema secara manual, jalankan file di `migrations/2024_01_01_create_tables.sql`.

3. **Konfigurasi environment**

   Salin file contoh lalu sesuaikan nilainya:
   ```bash
   cp .env.example .env
   ```

   Variabel yang tersedia:

   | Variabel | Keterangan | Default |
   |---|---|---|
   | `SERVER_PORT` | Port server | `8081` |
   | `DB_HOST` | Host MySQL | `localhost` |
   | `DB_PORT` | Port MySQL | `3306` |
   | `DB_USER` | User MySQL | `root` |
   | `DB_PASSWORD` | Password MySQL | - |
   | `DB_NAME` | Nama database | `nabungyuk` |
   | `JWT_SECRET` | Secret key JWT (min. 32 karakter) | - |
   | `CORS_ALLOWED_ORIGINS` | Daftar origin frontend yang diizinkan (pisahkan koma) | `http://localhost:3000,http://127.0.0.1:3000` |
   | `REMINDER_CHECK_INTERVAL` | Interval pengecekan pengingat (`disabled` untuk mematikan) | `30s` |
   | `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` | Konfigurasi email untuk fitur reminder | - |
   | `TRUSTED_PROXIES` | IP/CIDR reverse proxy yang dipercaya (opsional) | - |

4. **Install dependencies**
   ```bash
   go mod download
   ```

5. **Jalankan server**
   ```bash
   go run main.go
   ```

   Server akan berjalan di `http://localhost:8081` (atau sesuai `SERVER_PORT`).

6. **Buka aplikasi**

   Akses `http://localhost:8081` di browser untuk halaman login, atau langsung ke:
   - `/register.html` — Registrasi akun
   - `/dashboard.html` — Dashboard utama
   - `/transactions.html` — Kelola transaksi (termasuk scan struk)
   - `/savings.html` — Target tabungan
   - `/reminders.html` — Pengingat
   - `/reports.html` — Laporan keuangan

## 🔌 API Endpoints

### Publik
| Method | Endpoint | Deskripsi |
|---|---|---|
| POST | `/api/register` | Registrasi user baru |
| POST | `/api/login` | Login (rate limited) |

### Terproteksi (memerlukan JWT)
| Method | Endpoint | Deskripsi |
|---|---|---|
| POST | `/api/logout` | Logout |
| GET | `/api/profile` | Profil user |
| GET | `/api/dashboard` | Ringkasan dashboard |
| GET | `/api/dashboard/chart` | Data grafik dashboard |
| GET | `/api/transactions` | Daftar transaksi |
| POST | `/api/transactions` | Tambah transaksi |
| GET | `/api/transactions/:id` | Detail transaksi |
| GET | `/api/transactions/:id/receipt` | Lihat struk transaksi |
| PUT | `/api/transactions/:id` | Update transaksi |
| DELETE | `/api/transactions/:id` | Hapus transaksi |
| GET | `/api/savings` | Daftar target tabungan |
| POST | `/api/savings` | Buat target tabungan |
| GET | `/api/savings/:id` | Detail target tabungan |
| PUT | `/api/savings/:id` | Update target tabungan |
| DELETE | `/api/savings/:id` | Hapus target tabungan |
| POST | `/api/savings/:id/deposit` | Tambah setoran |
| GET | `/api/savings/:id/deposits` | Riwayat setoran |
| GET | `/api/reminders` | Daftar pengingat |
| POST | `/api/reminders` | Buat pengingat |
| GET | `/api/reminders/:id` | Detail pengingat |
| PUT | `/api/reminders/:id` | Update pengingat |
| DELETE | `/api/reminders/:id` | Hapus pengingat |
| GET | `/api/reports` | Data laporan |
| GET | `/api/reports/chart` | Data grafik laporan |

Health check tersedia di `GET /health`.

## 🗃️ Skema Database

- **users** — data akun (nama, email, password ter-hash)
- **transactions** — transaksi income/expense, kategori, nominal, lampiran struk
- **saving_goals** — target tabungan (nama, nominal target, deadline, ikon)
- **saving_deposits** — riwayat setoran per target tabungan
- **reminders** — jadwal pengingat menabung (harian/mingguan/bulanan)

Detail lengkap ada di [`migrations/2024_01_01_create_tables.sql`](migrations/2024_01_01_create_tables.sql).

## 🔒 Keamanan

- Password di-hash dengan bcrypt saat registrasi.
- Autentikasi berbasis JWT dengan secret minimal 32 karakter.
- Rate limiting pada endpoint login.
- CORS dibatasi sesuai whitelist origin dari environment variable.
- `TRUSTED_PROXIES` opsional untuk mencegah spoofing header `X-Forwarded-For` bila tidak di belakang reverse proxy.
