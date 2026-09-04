# mistah

> *mistah* (maya yucateco): «él que barre», del verbo *mis* (barrer).

CLI open-source para macOS y Windows que recupera los gigabytes que tu
equipo acumula con el tiempo: papelera/Recycle Bin, cachés de apps,
navegadores, y (en macOS) backups viejos de iPhone, snapshots de Time
Machine, adjuntos de Mail. Si además eres desarrollador, también limpia
tus cachés de dev (npm, Docker, Xcode/JetBrains…). Cero telemetría.
Código auditable. MIT.

🌐 https://mistah.sistematlan.com

## Instalación

### macOS

```sh
curl -fsSL https://mistah.sistematlan.com/install.sh | sh
```

El script detecta tu arquitectura (`arm64` o `amd64`), descarga el binario
de la última release de GitHub, y lo coloca en `/usr/local/bin/mistah`.

Lee `install.sh` antes de ejecutarlo si tienes dudas — son 200 líneas de
shell POSIX sin sorpresas.

### Windows

Descarga `mistah_<versión>_windows_amd64.zip` desde
[GitHub Releases](https://github.com/sistematlan/mistah/releases),
extrae `mistah.exe` y colócalo donde prefieras (o en una carpeta que
esté en tu `PATH`, ej. `C:\Windows` o una carpeta propia agregada al
PATH del sistema).

No hay instalador ni script de una línea todavía — es un binario suelto,
sin firma de código (verás el aviso de SmartScreen "editor desconocido"
la primera vez; puedes revisar el código fuente antes de confiar en él).

## Uso rápido

```sh
mistah                          # wizard guiado con tres niveles
mistah scan                     # escaneo informativo por categoría, no borra nada
mistah clean --dry-run          # ver qué se borraría (solo caches dev)
mistah clean                    # interactivo, ítem por ítem
mistah clean --include-system   # también papelera/Recycle Bin, cachés de apps, snapshots, logs
mistah clean --all              # todas las categorías (= wizard Profundo)
mistah --help                   # ayuda completa
mistah --advanced --help        # mostrar todos los comandos avanzados
```

### Niveles del wizard

| Nivel | Qué incluye |
|---|---|
| **Ligera** | Cachés de apps (Spotify, Slack, navegadores), papelera/Recycle Bin, miniaturas (QuickLook en macOS / thumbcache en Windows), adjuntos de Mail (macOS), firmware iOS redundante, cachés seguros de dev |
| **Estándar** | + snapshots de Time Machine (macOS), logs y reportes de fallos (macOS), archivos temporales de %TEMP% (Windows). Para devs: Docker prune, JetBrains, artefactos de Xcode (macOS) |
| **Profunda** | + datos huérfanos, candidatos en Downloads, backups de iPhone/iPad, simuladores Xcode obsoletos (macOS). Pregunta por cada archivo que pueda ser tuyo |

El wizard detecta automáticamente si tienes herramientas de desarrollo y
ajusta lo que muestra. Un usuario sin entorno de dev nunca ve "Docker prune".

### Qué detecta

**Para cualquier Mac o PC Windows:**

| Categoría | macOS | Windows |
|---|---|---|
| 🗑️ Papelera | `~/.Trash` (vaciado, no borra la carpeta) | Recycle Bin (vía `SHEmptyRecycleBinW`) |
| 📱 Backups de iPhone/iPad | `MobileSync/Backup` — suelen ser 4-15 GB cada uno | Igual, bajo la ruta de iTunes/Apple Devices en Windows |
| 🎵 Cachés de apps | Spotify, Slack, Discord, Telegram, Zoom, Teams, Notion, Figma | Spotify, Slack, Discord, Telegram, Zoom, Teams, Notion, Figma (rutas `%LOCALAPPDATA%`/`%APPDATA%`) |
| 🌐 Cachés de navegadores | Chrome, Safari, Firefox, Brave, Edge | Chrome, Edge, Brave |
| ⏱️ Snapshots de Time Machine | snapshots locales que macOS retiene | *(sin equivalente; VSS requiere permisos de administrador)* |
| 📨 Adjuntos de Mail | descargas que Mail.app vuelve a bajar | *(sin equivalente directo con Outlook aún)* |
| 💬 Adjuntos de iMessage | fotos/videos de chats >6 meses (los chats no se borran) | *(iMessage no existe en Windows)* |
| 🖼️ Miniaturas | caché de QuickLook | caché de miniaturas del Explorador (`thumbcache_*.db`) |
| 📦 Actualizaciones iOS (.ipsw) | firmware que Apple re-sirve | Igual |
| 📋 Logs y crash reports | logs viejos + reportes >30 días | *(sin detector dedicado aún; ver BACKLOG.md)* |
| 🧹 Archivos temporales | — | `%TEMP%` (>7 días) |

**Para desarrolladores (ambas plataformas):**

| Categoría | Detalle |
|---|---|
| 🧹 Cachés de dev | npm, pnpm, yarn, pip, uv, Cargo, Go, Composer, node-gyp — más NuGet en Windows |
| 🐳 Docker | `docker system prune` (sin tocar volúmenes) — CLI idéntica en ambas plataformas |
| 🛠️ Xcode | DerivedData, Archives, DeviceSupport, simuladores obsoletos — *macOS únicamente* |
| 💡 JetBrains | cachés de IDEs (`Library/Application Support/JetBrains` en macOS, `%LOCALAPPDATA%\JetBrains` en Windows) |
| 🧠 Downloads | instaladores duplicados, ZIPs ya extraídos, dumps viejos, proyectos abandonados con node_modules |

## Características

- 🧹 30+ detectores entre sistema, dispositivos, apps y dev tools
- 🖥️ Multiplataforma: macOS y Windows desde un mismo binario Go, con detectores nativos por plataforma (nada de WSL ni emulación)
- 🛡️ Doble barrera de seguridad: `SafeRoots` (solo borra dentro de `$HOME`/perfil de usuario y el directorio temporal del sistema) + `OffLimits` (jamás toca Documentos, Fotos, Escritorio, iCloud/Credential Manager…)
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

### Compilar para otra plataforma (cross-compile)

Go hace esto trivial sin herramientas extra:

```sh
GOOS=windows GOARCH=amd64 go build -o mistah.exe .
GOOS=darwin  GOARCH=arm64 go build -o mistah-mac .
```

## Roadmap

Ver [BACKLOG.md](BACKLOG.md). En curso:

- `mistah report --json` para integración con scripts
- Detección de duplicados (fotos, PDFs) — evaluando
- Apple notarization para que Gatekeeper no se queje al primer arranque
- Firma de código para Windows (evitar el aviso de SmartScreen)
- Detector de caché de Firefox en Windows (requiere leer `profiles.ini`, ver BACKLOG.md)
- Soporte Linux (largo plazo)

## Contribuir

PRs y reports bienvenidos. Antes de PRs grandes:

1. Abre un issue describiendo el cambio.
2. `make test` debe pasar.
3. Sigue el estilo Go idiomático del repo (errores explícitos, paquetes pequeños, sin globals).
4. Si tu cambio toca un detector o remover, revisa si necesita una
   variante `_darwin.go` / `_windows.go` — ver la nota de arquitectura
   al inicio de `internal/cleaner/cleaner.go` y `internal/system/system.go`.

## Licencia

MIT. Ver [LICENSE](LICENSE).

## Créditos

Construido por [@chrisherlan](https://github.com/chrisherlan) en México,
bajo el paraguas de [sistematlan](https://sistematlan.com).
