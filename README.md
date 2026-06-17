# mistah

> *mistah* (maya yucateco): «él que barre», del verbo *mis* (barrer).

CLI open-source para macOS que recupera los gigabytes que tu Mac acumula
con el tiempo: papelera, backups viejos de iPhone, cachés de apps,
snapshots de Time Machine, adjuntos de Mail y más. Si además eres
desarrollador, también limpia tus cachés de dev (npm, Docker, Xcode…).
Cero telemetría. Código auditable. MIT.

🌐 https://mistah.sistematlan.com

## Instalación

```sh
curl -fsSL https://mistah.sistematlan.com/install.sh | sh
```

El script detecta tu arquitectura (`arm64` o `amd64`), descarga el binario
de la última release de GitHub, y lo coloca en `/usr/local/bin/mistah`.

Lee `install.sh` antes de ejecutarlo si tienes dudas — son 200 líneas de
shell POSIX sin sorpresas.

## Uso rápido

```sh
mistah                          # wizard guiado con tres niveles
mistah scan                     # escaneo informativo por categoría, no borra nada
mistah clean --dry-run          # ver qué se borraría (solo caches dev)
mistah clean                    # interactivo, ítem por ítem
mistah clean --include-system   # también papelera, cachés de apps, snapshots, logs
mistah clean --all              # todas las categorías (= wizard Profundo)
mistah --help                   # ayuda completa
mistah --advanced --help        # mostrar todos los comandos avanzados
```

### Niveles del wizard

| Nivel | Qué incluye |
|---|---|
| **Ligera** | Cachés de apps (Spotify, Slack, navegadores), papelera, miniaturas de QuickLook, adjuntos de Mail, firmware iOS redundante, cachés seguros de dev |
| **Estándar** | + snapshots de Time Machine, logs y reportes de fallos. Para devs: Docker prune, JetBrains, artefactos de Xcode |
| **Profunda** | + datos huérfanos, candidatos en Downloads, backups de iPhone/iPad, simuladores Xcode obsoletos. Pregunta por cada archivo que pueda ser tuyo |

El wizard detecta automáticamente si tienes herramientas de desarrollo y
ajusta lo que muestra. Un usuario sin entorno de dev nunca ve "Docker prune".

### Qué detecta

**Para cualquier Mac:**

| Categoría | Detalle |
|---|---|
| 🗑️ Papelera | `~/.Trash` (vaciado, no borra la carpeta) |
| 📱 Backups de iPhone/iPad | `MobileSync/Backup` — suelen ser 4-15 GB cada uno |
| 🎵 Cachés de apps | Spotify, Slack, Discord, Telegram, Zoom, Teams, Notion, Figma, Linear, Arc |
| 🌐 Cachés de navegadores | Chrome, Safari, Firefox, Brave, Edge |
| ⏱️ Snapshots de Time Machine | snapshots locales que macOS retiene |
| 📨 Adjuntos de Mail | descargas que Mail.app vuelve a bajar |
| 💬 Adjuntos de iMessage | fotos/videos de chats >6 meses (los chats no se borran) |
| 🖼️ Miniaturas de QuickLook | cache que macOS regenera |
| 📦 Actualizaciones iOS (.ipsw) | firmware que Apple re-sirve |
| 📋 Logs y crash reports | logs viejos + reportes >30 días |

**Para desarrolladores:**

| Categoría | Detalle |
|---|---|
| 🧹 Cachés de dev | npm, pnpm, yarn, brew, pip, uv, Cargo, Go, Composer, node-gyp |
| 🐳 Docker | `docker system prune` (sin tocar volúmenes) |
| 🛠️ Xcode | DerivedData, Archives, DeviceSupport, simuladores obsoletos |
| 💡 JetBrains | cachés de IDEs |
| 🧠 Downloads | instaladores duplicados, ZIPs ya extraídos, dumps viejos, proyectos abandonados con node_modules |

## Características

- 🧹 30+ detectores entre sistema, dispositivos, apps y dev tools
- 🛡️ Doble barrera de seguridad: `SafeRoots` (solo borra dentro de `$HOME` y `/tmp`) + `OffLimits` (jamás toca Documentos, Fotos, Escritorio, iCloud, Llaveros…)
- ✋ Los datos que pueden ser tuyos (backups, papelera, dumps) requieren confirmación por ítem, incluso en el wizard
- 🌐 Bilingüe — autodetecta `$LANG` (es / en)
- 🔒 Cero telemetría, cero red, cero analytics

## Construir desde código

```sh
git clone https://github.com/sistematlan/mistah.git
cd mistah
make build
./bin/mistah
```

Requiere Go 1.26+.

## Roadmap

Ver [BACKLOG.md](BACKLOG.md). En curso:

- `mistah report --json` para integración con scripts
- Detección de duplicados (fotos, PDFs) — evaluando
- Apple notarization para que Gatekeeper no se queje al primer arranque
- Soporte Linux (largo plazo)

## Contribuir

PRs y reports bienvenidos. Antes de PRs grandes:

1. Abre un issue describiendo el cambio.
2. `make test` debe pasar.
3. Sigue el estilo Go idiomático del repo (errores explícitos, paquetes pequeños, sin globals).

## Licencia

MIT. Ver [LICENSE](LICENSE).

## Créditos

Construido por [@chrisherlan](https://github.com/chrisherlan) en México,
bajo el paraguas de [sistematlan](https://sistematlan.com).
