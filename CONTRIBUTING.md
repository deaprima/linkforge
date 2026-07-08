# Contributing Guide — LinkForge

## Commit Message Convention

Project ini menggunakan standar **[Conventional Commits](https://www.conventionalcommits.org/)**.
Setiap commit message harus mengikuti format berikut:

```
<type>(<scope>): <deskripsi singkat>

[body opsional — jelaskan APA dan KENAPA, bukan BAGAIMANA]

[footer opsional — misal: BREAKING CHANGE atau referensi issue]
```

---

### Type (Wajib)

| Type | Kapan dipakai |
|---|---|
| `feat` | Menambahkan fitur baru |
| `fix` | Memperbaiki bug |
| `refactor` | Refactor kode tanpa menambah fitur atau memperbaiki bug |
| `test` | Menambah atau memperbaiki test |
| `docs` | Perubahan dokumentasi saja |
| `chore` | Update dependency, konfigurasi build, atau file setup |
| `ci` | Perubahan file CI/CD (GitHub Actions) |
| `perf` | Optimasi performa |
| `style` | Perubahan formatting, whitespace (tidak mengubah logika) |
| `revert` | Membatalkan commit sebelumnya |

---

### Scope (Opsional tapi Dianjurkan)

Scope membantu menunjukkan **bagian mana dari project** yang berubah.

| Scope | Area |
|---|---|
| `gateway` | Gateway Service |
| `auth` | Auth Service |
| `link` | Link Service |
| `analytics` | Analytics Service |
| `shared` | Kode shared (breaker, logger, kafka) |
| `proto` | File Protobuf |
| `web` | Frontend |
| `infra` | Docker, docker-compose, konfigurasi infra |
| `ci` | GitHub Actions workflow |
| `deps` | Update dependency |

---

### Contoh Commit Message

**✅ Benar:**

```
feat(auth): add JWT refresh token rotation

Implement refresh token rotation mechanism to prevent token reuse.
Old refresh tokens are invalidated immediately after use.
```

```
feat(link): add custom alias validation

Validate alias contains only alphanumeric chars and hyphens.
Reject reserved aliases (api, admin, health, etc.).
```

```
fix(gateway): circuit breaker not resetting after timeout

The timeout value was set in milliseconds instead of seconds.
Fixes interval reset causing breaker to stay open indefinitely.
```

```
chore(infra): add dual listener config to Kafka docker-compose

INTERNAL listener for container-to-container communication.
EXTERNAL listener for host machine access on port 9092.
```

```
docs: add API contract draft v1
```

```
ci: add GitHub Actions workflow for linting and testing
```

---

**❌ Salah (jangan seperti ini):**

```
update code          ← terlalu vague
fix bug              ← bug apa?
WIP                  ← jangan commit WIP ke main
asdfgh               ← tidak bermakna
```

---

### Breaking Changes

Jika perubahan Anda **tidak backward compatible**, tambahkan `!` setelah type/scope
dan sertakan keterangan `BREAKING CHANGE:` di footer:

```
feat(auth)!: change token response from body to HttpOnly cookie

BREAKING CHANGE: access_token dan refresh_token tidak lagi
dikembalikan di response body. Keduanya kini di-set via
HttpOnly Cookie secara otomatis.
```

---

### Aturan Tambahan

1. **Deskripsi**: gunakan kalimat imperatif, huruf kecil, tanpa titik di akhir.
   - ✅ `add circuit breaker to gateway`
   - ❌ `Added circuit breaker to gateway.`
2. **Panjang deskripsi**: maksimal 72 karakter.
3. **Bahasa**: gunakan **Bahasa Inggris** untuk konsistensi dan supaya bisa dibaca secara internasional (ini portofolio publik!).
4. **Satu commit = satu tujuan**: jangan campur refactor dengan fitur baru dalam satu commit.


## Branch Strategy
Project ini menggunakan **Git Flow**:
| Branch | Fungsi |
|---|---|
| `main` | Production only. Hanya menerima merge dari `develop`. |
| `develop` | Integration branch. Semua feature branch merge ke sini. |
| `feat/*` | Fitur baru |
| `fix/*` | Bug fix |
| `docs/*` | Perubahan dokumentasi |
| `chore/*` | Konfigurasi, setup, dependency |
| `ci/*` | CI/CD workflow |
| `refactor/*` | Refactoring tanpa perubahan perilaku |

### Flow Kerja
1. Checkout dari `develop` → buat branch baru
2. Kerjakan perubahan → commit dengan Conventional Commits
3. Push branch → buat Pull Request ke `develop`
4. Merge ke `develop` setelah review
5. `develop` → `main` hanya saat siap release/deploy