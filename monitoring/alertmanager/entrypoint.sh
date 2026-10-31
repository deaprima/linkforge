#!/bin/sh
# Alertmanager tidak bisa membaca env di file konfigurasinya, jadi konfigurasi dibuat saat container start.
# Hasilnya JSON (YAML yang valid) dengan nilai yang di-escape lewat esc(), supaya password SMTP berisi
# tanda kutip, backslash, `&`, `|` atau `/` tidak merusak konfigurasi (sed substitusi biasa akan rusak).
#
# ALERT_EMAIL_TO kosong  -> tanpa notifikasi: alert hanya terlihat di UI Prometheus dan Alertmanager.
# ALERT_EMAIL_TO terisi  -> email lewat SMTP (default memakai SMTP_* yang sama dengan reset password).
set -eu

esc() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'; }

OUT="${ALERT_CONFIG_OUT:-/tmp/alertmanager.yml}"
EMAIL_CONFIGS=""

if [ -n "${ALERT_EMAIL_TO:-}" ]; then
  : "${ALERT_SMTP_HOST:?ALERT_EMAIL_TO is set, so SMTP_HOST must be set too}"
  : "${ALERT_SMTP_FROM:?ALERT_EMAIL_TO is set, so SMTP_FROM must be set too}"
  AUTH=""
  if [ -n "${ALERT_SMTP_USER:-}" ]; then
    AUTH=$(printf ', "auth_username": "%s", "auth_password": "%s"' "$(esc "$ALERT_SMTP_USER")" "$(esc "${ALERT_SMTP_PASS:-}")")
  fi
  EMAIL_CONFIGS=$(printf '"email_configs": [{"to": "%s", "from": "%s", "smarthost": "%s:%s", "send_resolved": true, "require_tls": true%s}]' \
    "$(esc "$ALERT_EMAIL_TO")" "$(esc "$ALERT_SMTP_FROM")" "$(esc "$ALERT_SMTP_HOST")" "$(esc "${ALERT_SMTP_PORT:-587}")" "$AUTH")
else
  echo "alertmanager: ALERT_EMAIL_TO is not set; alerts are only visible in the Prometheus and Alertmanager UIs" >&2
fi

cat > "$OUT" <<EOF
{
  "global": { "resolve_timeout": "5m" },
  "route": {
    "receiver": "default",
    "group_by": ["alertname", "job"],
    "group_wait": "30s",
    "group_interval": "5m",
    "repeat_interval": "4h",
    "routes": [
      { "matchers": ["severity=\"warning\""], "repeat_interval": "12h" }
    ]
  },
  "inhibit_rules": [
    {
      "source_matchers": ["alertname=\"ServiceDown\""],
      "target_matchers": ["severity=\"warning\""],
      "equal": ["job"]
    }
  ],
  "receivers": [
    { "name": "default"${EMAIL_CONFIGS:+, $EMAIL_CONFIGS} }
  ]
}
EOF

# Hanya membuat konfigurasi (dipakai untuk validasi dengan amtool)
if [ "${ALERT_GENERATE_ONLY:-}" = "1" ]; then
  exit 0
fi

exec /bin/alertmanager --config.file="$OUT" --storage.path=/alertmanager "$@"
