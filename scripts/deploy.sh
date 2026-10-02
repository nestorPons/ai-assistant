#!/usr/bin/env bash
# Despliega al servidor de .vscode/sftp.json (mismo destino que la extensión SFTP).
# Uso: deploy.sh [<tree-ish>]
#   - con argumento: despliega el estado exacto de ese commit (lo que usa el
#     hook pre-push al pushear main: en el servidor solo hay código de main).
#   - sin argumento: despliega los archivos versionados del árbol de trabajo.
# Usa tar sobre SSH (no requiere rsync en el servidor) y mantiene un manifest
# remoto para eliminar archivos que dejan de existir.
# Nunca toca los archivos protegidos (compose de prod, .env, cursor local),
# que se gestionan a mano en el servidor.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="$REPO_ROOT/.vscode/sftp.json"

[[ -f "$CONFIG" ]] || { echo "error: no existe $CONFIG" >&2; exit 1; }

HOST="$(jq -r '.host' "$CONFIG")"
PORT="$(jq -r '.port // 22' "$CONFIG")"
USER="$(jq -r '.username' "$CONFIG")"
REMOTE_PATH="$(jq -r '.remotePath' "$CONFIG")"
KEY="$(jq -r '.privateKeyPath // empty' "$CONFIG")"

[[ -n "$HOST" && -n "$USER" && -n "$REMOTE_PATH" ]] || {
  echo "error: .vscode/sftp.json incompleto (host, username, remotePath)" >&2
  exit 1
}

KEY="${KEY/#\~/$HOME}"
SSH_ARGS=(-p "$PORT")
[[ -n "$KEY" ]] && SSH_ARGS+=(-i "$KEY")
TARGET="${USER}@${HOST}"
MANIFEST=".tskhub-deploy-manifest"
REMOTE_CMD="mkdir -p '$REMOTE_PATH' && tar -xzf - -C '$REMOTE_PATH'"
TREE="${1:-}"

# Archivos gestionados a mano en el servidor: nunca se suben ni se borran.
# (docker-compose*.yml de prod va por detrás de Traefik; .env y el cursor
# local son configuración de cada entorno.) En línea con los --exclude del
# workflow .github/workflows/deploy.yml.
PROTECTED_REGEX='^(docker-compose\.yml|docker-compose\.override\.yml|\.env|\.gmail_cursor\.json)$'

FILES_FILE="$(mktemp)"
MANIFEST_LOCAL="$(mktemp)"
MANIFEST_REMOTE="$(mktemp)"
TMP_ARCH="$(mktemp)"
trap 'rm -f "$FILES_FILE" "$MANIFEST_LOCAL" "$MANIFEST_REMOTE" "$TMP_ARCH"' EXIT

if [[ -n "$TREE" ]]; then
  git -C "$REPO_ROOT" cat-file -e "${TREE}^{commit}" 2>/dev/null \
    || { echo "error: commit inválido: $TREE" >&2; exit 1; }
  echo "→ ${TARGET}:${REMOTE_PATH} (main @ ${TREE:0:7})"
  git -C "$REPO_ROOT" ls-tree -r --name-only -z "$TREE" | grep -zvE "$PROTECTED_REGEX" > "$FILES_FILE" || test "${PIPESTATUS[0]}" -eq 0
  git -C "$REPO_ROOT" ls-tree -r --name-only "$TREE" | grep -vE "$PROTECTED_REGEX" | sort > "$MANIFEST_LOCAL"
  echo "· subiendo archivos (sin compose/.env)…"
  git -C "$REPO_ROOT" archive "$TREE" > "$TMP_ARCH"
  for m in docker-compose.yml docker-compose.override.yml .env .gmail_cursor.json; do
    tar --delete -f "$TMP_ARCH" "$m" 2>/dev/null || true
  done
  gzip -c "$TMP_ARCH" | \
    ssh "${SSH_ARGS[@]}" "$TARGET" "$REMOTE_CMD"
else
  # Solo archivos bajo control de git; los no rastreados (.env, vendor/,
  # node_modules/) no se tocan: el servidor conserva su configuración.
  echo "→ ${TARGET}:${REMOTE_PATH} (árbol de trabajo)"
  git -C "$REPO_ROOT" ls-files -z | grep -zvE "$PROTECTED_REGEX" > "$FILES_FILE" || test "${PIPESTATUS[0]}" -eq 0
  git -C "$REPO_ROOT" ls-files | grep -vE "$PROTECTED_REGEX" | sort > "$MANIFEST_LOCAL"
  echo "· subiendo archivos (sin compose/.env)…"
  tar --null -T "$FILES_FILE" -czf - | \
    ssh "${SSH_ARGS[@]}" "$TARGET" "$REMOTE_CMD"
fi

# Eliminar en remoto los archivos que ya no existen en el despliegue.
# Los protegidos nunca se borran aunque sigan en un manifest antiguo.
if ssh "${SSH_ARGS[@]}" "$TARGET" "test -f '$REMOTE_PATH/$MANIFEST'"; then
  ssh "${SSH_ARGS[@]}" "$TARGET" "cat '$REMOTE_PATH/$MANIFEST'" | sort > "$MANIFEST_REMOTE"
  TO_DELETE="$(comm -23 "$MANIFEST_REMOTE" "$MANIFEST_LOCAL" | grep -vE "$PROTECTED_REGEX" || true)"
  if [[ -n "$TO_DELETE" ]]; then
    printf '%s\n' "$TO_DELETE" | sed 's/^/· borrando /'
    printf '%s\n' "$TO_DELETE" | \
      ssh "${SSH_ARGS[@]}" "$TARGET" "while IFS= read -r f; do rm -f '$REMOTE_PATH/'\$f; done"
  fi
fi

# Guardar el manifest nuevo.
ssh "${SSH_ARGS[@]}" "$TARGET" "cat > '$REMOTE_PATH/$MANIFEST'" < "$MANIFEST_LOCAL"

echo "✓ despliegue completado"
