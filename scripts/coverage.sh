#!/usr/bin/env bash
# Mengukur coverage lapisan inti tiap service dan gagal kalau ada yang di bawah batas (AI-RULES.md bagian 5).
#
# Yang dihitung: handler / service / repository / delivery / middleware dsb. Yang dikecualikan:
# cmd/ (wiring main), proto/ (kode hasil generate), entity/ (struct tanpa logika), testdb/ (helper test).
# Test lintas package ikut terhitung (-coverpkg), mis. handler Gateway yang menjalankan router.
#
# Repository butuh Postgres; tanpa TEST_DSN test-nya di-skip dan coverage auth/link/analytics akan turun:
#   TEST_DSN="postgres://user:pass@localhost:5432/linkforge?sslmode=disable" scripts/coverage.sh
#
# Batas bisa diubah lewat COVERAGE_MIN (default 70). Service "shared" hanya dilaporkan, tidak di-gate,
# karena sebagian (Kafka) butuh broker yang tidak selalu ada.
set -euo pipefail

cd "$(dirname "$0")/../services"

PKGS=$(go list ./... | grep -v -e '/cmd/' -e '/proto/' -e '/entity' -e '/testdb' | paste -sd, -)
PROFILE=$(mktemp)
trap 'rm -f "$PROFILE"' EXIT

if [ -z "${TEST_DSN:-}" ]; then
  echo "WARNING: TEST_DSN is not set; repository tests are skipped and coverage will look lower than it is." >&2
fi

go test -count=1 -coverpkg="$PKGS" -coverprofile="$PROFILE" ./... > /dev/null

# Satu blok kode bisa muncul di banyak binary test; dihitung tercakup kalau salah satunya menjalankannya.
awk -v min="${COVERAGE_MIN:-70}" '
NR > 1 { n[$1] = $2; if ($3 > 0) hit[$1] = 1 }
END {
  for (k in n) {
    f = k; sub(/:.*/, "", f); sub(/^github.com\/deaprima\/linkforge\/services\//, "", f)
    split(f, p, "/"); svc = p[1]
    total[svc] += n[k]; if (hit[k]) covered[svc] += n[k]
  }
  split("gateway auth link analytics", gated, " ")
  for (i in gated) isGated[gated[i]] = 1

  failed = 0
  for (svc in total) {
    pct = 100 * covered[svc] / total[svc]
    status = "info"
    if (svc in isGated) { status = (pct >= min) ? "ok" : "FAIL"; if (pct < min) failed = 1 }
    printf "%-10s %5.1f%%  (%s)\n", svc, pct, status
  }
  if (failed) { printf "\nCoverage is below %s%% for at least one service.\n", min; exit 1 }
}' "$PROFILE" | sort
exit "${PIPESTATUS[1]}"
