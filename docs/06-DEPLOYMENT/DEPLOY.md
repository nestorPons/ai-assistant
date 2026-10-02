# Despliegue — Push de main a SFTP

## Objetivo
- Al hacer `git push` de la rama **main**, subir ese estado exacto a `/root/projects/tskhub` en `tsk.nestorpons.com`.
- En el servidor solo hay código de main; otras ramas nunca despliegan.
- Mismo destino que la extensión SFTP de VSCode (`.vscode/sftp.json`).

## Componentes
- `.vscode/sftp.json`: host, usuario, puerto, `remotePath`, clave SSH.
- `scripts/deploy.sh [<commit>]`: sube el árbol exacto del commit (vía `git archive` + `tar` sobre SSH); sin argumento sube el árbol de trabajo.
- `.githooks/pre-push`: solo despliega si el destino es `refs/heads/main`.

## Comportamiento
- Solo archivos bajo control de git; no rastreados (`.env`, `vendor/`, `node_modules/`) no se tocan.
- Mantiene `.tskhub-deploy-manifest` en remoto para borrar archivos eliminados del repo.
- Si el despliegue falla, el push se aborta.
- Sin cambios que pushear, el hook no se ejecuta.

## Instalación (una vez por clon)
```bash
git config core.hooksPath .githooks   # ya aplicado en este clon
```

## Uso
```bash
git push                  # despliega solo si pushea main
./scripts/deploy.sh       # despliegue manual (árbol de trabajo)
./scripts/deploy.sh main  # despliegue manual del estado de main
SKIP_DEPLOY=1 git push    # push sin desplegar
```

## Requisitos
- `jq`, `tar`, `ssh` en local; `tar` en remoto.
- Clave SSH con acceso (`~/.ssh/id_rsa` → `root@tsk.nestorpons.com`).
- Host en `~/.ssh/known_hosts`.

## Producción (Traefik)
- El `docker-compose.yml` del servidor se gestiona **a mano** (tras Traefik,
  `Host(tsk.nestorpons.com)`, red `traefik-net`): el deploy **nunca** lo sube
  ni lo borra (protegido en `scripts/deploy.sh` y excluido en el workflow,
  junto a `docker-compose.override.yml`, `.env` y `.gmail_cursor.json`).
- `web-dashboard` tras Traefik (puerto 80); resto solo en red `internal`.
- En el servidor:
```bash
cd /root/projects/tskhub
cp .env.example .env   # rellenar secretos (solo primera vez)
docker compose up -d --build
docker compose logs -f
```
