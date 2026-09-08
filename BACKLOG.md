# BACKLOG — mistah

> Pendientes priorizados. Ítems marcados con `[x]` están completados.
> Última actualización: sesión 2026-06-08.

---

## ✅ Completado en sesiones previas

- [x] Scaffold inicial: `scan`, `apps`, `caches`, `projects`
- [x] Tipo `item.Item` unificado con `Category` y `Risk`
- [x] Detectores ampliados de caches (uv, pnpm, npm `_npx`, JetBrains old versions, Cargo registry, iOS DeviceSupport, CoreSimulator, Xcode Archives)
- [x] `internal/orphans` con detectores: Docker leftover, WhatsApp media
- [x] Comando `orphans`
- [x] `internal/cleaner` con `Mode` (DryRun/Interactive/Yes), `Remover`, `Plan`, `Summary`
- [x] `PathRemover` con guardia `SafeRoots`; `DockerPruneRemover` (sin `--volumes`)
- [x] `TerminalPrompter` con `[s/N/v=ver/q=salir]` y confirmación por nombre para `RiskDangerous`
- [x] Comando `clean` con `--dry-run`, `--yes`, `--include-orphans`, `--include-downloads`
- [x] Bug fix: scanner reutilizado en `TerminalPrompter` (no se descarta el buffer entre prompts)
- [x] Tests unitarios (10 casos en cleaner, parser de tamaños Docker, splitJBVersion, fixtures de orphans)
- [x] **`internal/downloads`** con 7 clasificadores: installer-with-app, archive-extracted, project-folder, db-dump, old-video, old-archive, large-other
- [x] **Comando `downloads`** con tabla agrupada por subcategoría
- [x] Integración downloads → clean vía `--include-downloads`
- [x] Tests de downloads con fixtures temporales (8 casos)
- [x] **`internal/i18n`** sin libs externas, autodetect $LANG/$LC_ALL, soporte es+en, fallback a inglés
- [x] **Catálogos es/en** con paridad garantizada por tests, variantes `.simple` y `.advanced` por entrada
- [x] **`Item.HumanName()`, `HumanDetail(simple)`, `HumanRisk()`** para presentación localizada
- [x] **Flags globales `--lang` y `--advanced`** en root.go
- [x] **Migración de todos los detectores** a NameKey/DetailKey/DetailArgs
- [x] **TerminalPrompter localizado**: prompts, summary, confirmación dangerous
- [x] **`internal/spinner`** con detección de TTY y fallback para non-TTY
- [x] **`internal/wizard`** con tres niveles preset (Light/Standard/Deep) y filtros por nivel
- [x] **Wizard UI completa**: intro, scan animado, menú numerado, confirmación, resumen
- [x] **Cobra integration**: `mistah` sin args ejecuta wizard. Subcomandos siguen igual.
- [x] **Tests del wizard**: 6 casos cubriendo niveles, totales, monotonicidad, level.String()

---

## 🔥 Próxima sesión — completar MVP

### 1. Comando `downloads` (P0) — ✅ COMPLETADO

- [x] `internal/downloads/downloads.go` con detector
- [x] Smart rules: installer-with-app, archive-extracted, project-folder, db-dump, old-video, old-archive
- [x] Catch-all `large-other` para archivos >100 MB sin clasificar
- [x] Comando `cmd/downloads.go` con tabla agrupada
- [x] Integrar items en `clean` (categoría `CategoryDownload`, `RiskAskBefore`)
- [x] Tests con fixtures temporales

### 2. Flags globales + `report` (P0)

- [ ] Flag global `--path` (ya existe parcial en `projects`, normalizar)
- [ ] Flag global `--json` — toda salida puede emitirse como JSON
- [ ] Flag global `--no-color` — desactiva ANSI (cuando se agregue color)
- [ ] Flag global `--yes` — heredable (hoy solo está en `clean`)
- [ ] Comando `report` con `--out <file>` y formato estructurado
- [ ] Refactor: que cada `cmd/*.go` use un renderer común (text vs JSON)

**Estimado:** 2-3 horas. Refactor mediano porque toca todos los comandos.

### 3. Wizard mejoras (post-MVP, opcional)

- [ ] Flags directos para no-tty: `mistah --light`, `--standard`, `--deep` que skipean menu
- [ ] `--no-confirm` para modo CI con wizard
- [ ] Mostrar breakdown por categoría dentro del nivel ("npm: 1.2 GB, brew: 200 MB...")
- [ ] Spinner multi-fase: "Scanning caches..." → "Scanning orphans..." → "Scanning downloads..."

### 4. Color y UI polish (P1)

- [ ] Lib: `github.com/fatih/color` o solo ANSI manual (preferible — menos deps)
- [ ] Verde para "safe", amarillo para "ask", rojo para "danger"
- [ ] Detección de TTY: solo color cuando stdout sea terminal
- [ ] Símbolos: ✓ ✗ → ⚠ (con fallback ASCII en `--no-color`)
- [ ] Tabla con anchos calculados dinámicamente (hoy son fijos y truncan feo)

---

## 🟡 Backlog medio plazo (post-MVP)

### Más detectores de caches

#### P0 — alto impacto, fáciles de implementar
- [ ] **Electron caches genéricos**: `~/Library/Application Support/*/Cache`, `*/Code Cache`, `*/GPUCache` — culpable silencioso (VSCode, Cursor, Trae, Antigravity, Comet, Slack, Discord, Teams, Notion, Linear, todos los que sean Electron)
- [ ] **iOS Simulator devices**: `~/Library/Developer/CoreSimulator/Devices/` — los simuladores en sí mismos pueden ocupar 5-50 GB en devs iOS
- [ ] **Time Machine local snapshots**: `tmutil listlocalsnapshots /` — Apple los crea sin avisar, ocupan "purgeable space"
- [ ] **Mail.app adjuntos**: `~/Library/Mail/V*/MailData/`
- [ ] **Messages adjuntos**: `~/Library/Messages/Attachments/`
- [ ] **Photos library** caches: `~/Pictures/Photos Library.photoslibrary/resources/`
- [ ] **Spotify cache**: `~/Library/Caches/com.spotify.client`
- [ ] **SiriTTS voices**: `~/Library/Caches/SiriTTS` (voces descargadas, opcional desinstalar via Settings)
- [ ] **VSCode/Cursor/Trae específicos**: `~/Library/Application Support/{Code,Cursor,Trae,Antigravity}/Cache*` con desglose

#### P1 — dev tools faltantes
- [ ] **Bun**: `~/.bun/install/cache`
- [ ] **Deno**: `~/Library/Caches/deno`
- [ ] **Gradle**: `~/.gradle/caches`
- [ ] **Maven**: `~/.m2/repository` (cuidado: no es solo caché, también deps activas)
- [ ] **Playwright**: `~/Library/Caches/ms-playwright`
- [ ] **Cypress**: `~/Library/Caches/Cypress`
- [ ] **Puppeteer**: `~/.cache/puppeteer`
- [ ] **rustup toolchains viejas**: parsear `rustup toolchain list`
- [ ] **nvm versions viejas**: `~/.nvm/versions/node/v*`
- [ ] **pnpm node versions**: `~/Library/pnpm/nodejs/*`
- [ ] **Volta**: `~/.volta/cache`
- [ ] **VirtualBox VMs huérfanas**: `~/VirtualBox VMs`
- [ ] **OrbStack VMs**: desglosar Group Container por VM
- [ ] **JDownloader 2**: `~/Library/Application Support/JDownloader 2`
- [ ] **TorBrowser-Data**: `~/Library/Application Support/TorBrowser-Data`

### Más detectores de orphans

- [ ] **Group Containers de apps desinstaladas**: cruzar `~/Library/Group Containers/*` con `/Applications`
- [ ] **Application Support de apps desinstaladas**: cruzar con `/Applications`
- [ ] **Containers de apps desinstaladas**: cruzar `~/Library/Containers/*` con `/Applications`
- [ ] **Wondershare leftover**: `~/Library/Application Support/com.wondershare.Installer`
- [ ] **Mega Limited cache** (la app sigue, pero el cache crece sin tope)
- [ ] **Adobe leftover** (típicamente >5 GB tras desinstalar)

### Detector genérico "big files" (catch-all)

- [ ] Comando `mistah big-files` que escanea `$HOME` y reporta los N archivos más grandes
- [ ] Filtros: `--min-size 100M`, `--ext .mov,.mp4,.dmg`, `--older-than 90d`
- [ ] Clasifica automáticamente:
  - **Videos personales** (.mov, .mp4, .mkv) >500 MB
  - **DMGs/PKGs/ISOs** sueltos en cualquier sitio
  - **ZIPs/RARs/7z** + detectar si están extraídos al lado
  - **Backups** (.bak, .sql, .dump, .tar.gz)
  - **Carpetas con node_modules** fuera de `~/sourcecode`
  - **Binarios de build** (`target/`, `dist/`, `build/`, `.next/`) en cualquier proyecto
  - **Logs viejos** `~/Library/Logs/*` con >90 días
  - **Crash reports** `~/Library/Logs/DiagnosticReports/`
- [ ] Opcional: detección de duplicados por hash (SHA-256 sobre archivos >100 MB)

### `projects` mejorado

- [ ] Detectar `node_modules`, `vendor/`, `target/`, `.next/`, `dist/`, `build/` dentro de cada proyecto
- [ ] Reportar tamaño de artefactos vs código real
- [ ] Detectar proyectos sin commit en >1 año como "abandonados"
- [ ] Sugerir `git clean -xdf` para los abandonados con git
- [ ] Modo `--clean-build-artifacts` que borra solo carpetas regenerables

### `apps` mejorado

- [ ] Tamaño TOTAL (app + Application Support + Containers + Caches asociados)
- [ ] Sugerir desinstalación con `mdls` last-used > 180d
- [ ] Comando `mistah apps uninstall <name>` que llama a `mdfind` para encontrar todos los archivos relacionados

### Mejoras al cleaner

- [ ] Modo `--category cache|orphan|download|all` (filtro por categoría)
- [ ] Modo `--tool docker|jetbrains|...` (filtro por tool)
- [ ] Modo `--min-size 100M` (solo items grandes)
- [ ] Confirmación batch: "borrar todos los marcados con [s]" al final
- [ ] Undo log: registrar qué se borró y cuándo en `~/.mistah/history.log`

---

## 🟢 Backlog largo plazo (visión)

### OSS launch

- [ ] LICENSE (MIT)
- [ ] README.md con: badges, features, screenshots, quickstart
- [ ] CONTRIBUTING.md con: cómo agregar nuevos detectores
- [ ] CODE_OF_CONDUCT.md
- [ ] GitHub Actions: `go test`, `go vet`, `staticcheck`, `golangci-lint`
- [ ] goreleaser config para multi-arch (amd64, arm64)
- [x] Homebrew tap: `sistematlan/tools` (implementado sept. 2026, ver
      "Distribución — Homebrew" abajo)
- [ ] Issue templates (bug, feature, new-detector)
- [ ] PR template
- [ ] Sitio mínimo en GitHub Pages o Vercel (landing + docs)

### Distribución y crecimiento

- [ ] Release v0.1.0 a Homebrew
- [ ] Post en HackerNews ("Show HN: mistah — a transparent disk cleaner for macOS developers")
- [ ] Reddit: r/macOS, r/golang, r/programming
- [ ] Demo en video (asciinema) para README
- [ ] Documentación de cada detector con ejemplo de output

### Ideas más ambiciosas

- [ ] **Modo daemon**: ejecuta `scan` periódicamente y notifica cuando disco <20%
- [ ] **Plugins**: detectores externos cargados desde `~/.mistah/plugins/*.so` o subprocess
- [ ] **Dashboard TUI** con bubbletea (alternativa al CLI puro)
- [ ] **Política de retención**: archivo de config (`~/.mistah/config.toml`) con reglas tipo `npm cache: keep 30 days`
- [ ] **Comparativa pre/post**: muestra diff de uso por carpeta tras `clean`
- [ ] **Sugerencias inteligentes**: "tienes 12 carpetas con node_modules y solo trabajas en 3 proyectos activos"

### Multiplataforma — análisis técnico

**Estado actual (post sept. 2026):** mistah soporta macOS y Windows desde
el mismo módulo Go, usando build tags (`//go:build darwin` /
`//go:build windows`) para separar cada detector/remover concreto. La
orquestación (`Scan`, `Plan`, `Resolver`, `item.Item`) es 100% compartida;
solo los detectores de bajo nivel y sus paths difieren por archivo
(`<paquete>_darwin.go` / `<paquete>_windows.go`).

#### Soporte Windows — implementado

- [x] `internal/disk`: `DirSize` reescrito con `filepath.WalkDir` nativo
      (sin `du`), portable a cualquier OS. `Usage` usa
      `golang.org/x/sys/windows.GetDiskFreeSpaceEx` en Windows,
      `syscall.Statfs` en darwin/linux.
- [x] Caches dev: `%LOCALAPPDATA%`, `%APPDATA%` — npm, pnpm, yarn, pip,
      uv, Go, Composer, node-gyp, Cargo (ruta compartida sin cambios),
      + NuGet (Windows-only, sin equivalente macOS en la tabla original).
- [x] JetBrains: `%LOCALAPPDATA%\JetBrains` (vs `~/Library/Application
      Support/JetBrains` en macOS).
- [x] Docker: sin cambios — el CLI es idéntico en ambas plataformas.
      (Nota: Docker Desktop en Windows usa `.vhdx` bajo WSL2 en
      `%LOCALAPPDATA%\Docker\wsl\`; el detector de huérfanos ya lo cubre.)
- [x] Apps instaladas: `internal/apps` reescrito para leer el Registro
      (`HKLM`/`HKCU` × `Uninstall`, incluyendo `WOW6432Node` para apps de
      32 bits). "Último uso" es una heurística (mtime más reciente entre
      los archivos del directorio de instalación) — Windows no tiene un
      campo equivalente a `kMDItemLastUsedDate` de Spotlight.
- [x] Trash → Recycle Bin: implementado con `SHQueryRecycleBinW` /
      `SHEmptyRecycleBinW` (Shell32), no con iteración de directorio —
      la Papelera de Windows es un namespace virtual, no una carpeta.
- [x] `%TEMP%`: detector nuevo sin equivalente 1:1 en macOS (borra
      archivos >7 días, recursivo).
- [x] Miniaturas: `thumbcache_*.db`/`iconcache_*.db` bajo
      `AppData\Local\Microsoft\Windows\Explorer` (equivalente aproximado
      a QuickLook; en Windows es un archivo de base de datos, no una
      carpeta de imágenes individuales).
- [x] Browsers: Chrome, Edge, Brave con rutas
      `AppData\Local\...\User Data\Default\Cache` — con la misma
      disciplina de "nunca tocar `Default\` completo, solo la subcarpeta
      Cache" que ya regía en macOS.
- [x] Apps consumer: Spotify, Slack, Discord, Telegram, Zoom, Teams,
      Notion, Figma con tabla de rutas Windows equivalente.
- [x] iOS backups / .ipsw: mismo código de parseo de Info.plist
      (`internal/device/ios_backups.go`, sin cambios), solo la raíz
      cambia (`AppData\Roaming\Apple Computer\MobileSync\...`).
- [x] `SafeRoots`/`OffLimits`: `%TEMP%` + perfil de usuario; carpetas
      protegidas (`Documents`, `Desktop`, `Videos`, `Pictures`, `Music`)
      + Credential Manager (`AppData\Roaming\Microsoft\Credentials` /
      `\Crypto`) como equivalente a Keychains.
- [x] GoReleaser: target `windows/amd64` agregado, empaqueta `.zip` en
      vez de `.tar.gz` (convención nativa de Windows).
- [x] Validado en máquina Windows 11 real (no solo cross-compile): scan,
      caches, clean --dry-run y clean --yes probados end-to-end vía SSH.

#### Distribución — Homebrew (implementado sept. 2026)

- [x] Repo `sistematlan/homebrew-tools` creado como tap oficial.
- [x] `.goreleaser.yaml`: sección `homebrew_casks` (NO `brews` — esa
      sección está deprecada por GoReleaser desde v2.10 en favor de
      casks para distribuir binarios precompilados; `brews` ahora es
      solo para fórmulas que compilan desde source). Genera y publica
      el cask automáticamente en cada tag `v*`.
- [x] PAT fine-grained (`HOMEBREW_TAP_GITHUB_TOKEN`, scoped únicamente
      a `Contents: Read and write` sobre `homebrew-tools`) como secret
      en `sistematlan/mistah`, separado del `GITHUB_TOKEN` por defecto
      que solo tiene permisos sobre el propio repo.
- [x] Hook `postflight` con el mismo fix de `xattr -d
      com.apple.quarantine` que `install.sh` ya aplicaba — sin firma
      de código/notarización, Gatekeeper bloquearía el binario en el
      primer lanzamiento si no se retira ese flag.
- [x] Casks son un mecanismo exclusivo de macOS (Linuxbrew no los
      soporta, solo fórmulas) — GoReleaser genera de cualquier modo un
      bloque `on_linux` en el `.rb`, así que linuxbrew queda cubierto
      "gratis" sin haberlo pedido explícitamente. Linux sigue teniendo
      `install.sh`/`go install` como vía principal recomendada.
- [x] Validado end-to-end en máquina real: `brew tap sistematlan/tools`
      → `brew trust sistematlan/tools` (paso extra necesario porque es
      un tap de terceros, no `homebrew/cask` oficial — Homebrew lo
      exige por defecto) → `brew install mistah` → `mistah version`
      confirma versión/commit/fecha correctos.
- [ ] Sigue pendiente: firma de código real eliminaría el paso de
      `brew trust` y el hook de `xattr`, mismo gap que macOS ya tenía
      documentado (Apple notarization) — costo de Developer ID, no
      técnico.

#### Distribución — Scoop (implementado sept. 2026)

- [x] Repo `sistematlan/scoop-bucket` creado como bucket oficial —
      mismo modelo que Homebrew (repo git propio con manifiestos,
      sin submission/review central), por eso se priorizó antes de
      winget en la hoja de ruta.
- [x] `.goreleaser.yaml`: sección `scoops` (nombre en plural, distinto
      de `homebrew_casks`) apuntando al `.zip` de `windows_amd64` que
      ya se generaba para la descarga manual — sin build step nuevo.
- [x] PAT fine-grained (`SCOOP_BUCKET_GITHUB_TOKEN`, scope `Contents:
      Read and write` sobre `scoop-bucket` únicamente) como secret en
      `sistematlan/mistah`, mismo patrón que `HOMEBREW_TAP_GITHUB_TOKEN`.
- [x] Validado con snapshot local: el manifest JSON generado apunta
      correctamente al `.zip` con `mistah.exe` y trae el hash SHA256
      correcto.
- [x] Release real (v0.6.1) publicó el manifest en el bucket con
      versión y hash reales.
- [ ] Pendiente: probar `scoop install mistah` en una máquina Windows
      real (no se pudo completar en esta sesión — la máquina Windows
      usada para las pruebas de Homebrew/soporte Windows no estaba
      accesible en el momento). El manifest sigue el formato exacto
      que GoReleaser produce para cientos de otros proyectos, así que
      el riesgo de que falle es bajo, pero queda como verificación
      pendiente antes de considerar esta vía 100% confirmada end-to-end
      (a diferencia de Homebrew, que sí se probó en vivo).

#### Distribución — AUR (Arch Linux): pendiente, bloqueado en un paso manual

- [ ] Investigado sept. 2026, no implementado. AUR es estructuralmente
      distinto a Homebrew/Scoop: no permite paquetes binarios directos
      (solo `PKGBUILD`s, que idealmente compilan desde source; para
      binarios precompilados existe la convención `-bin`, es decir
      publicaríamos `mistah-bin`), usa git+ssh directo a
      `aur.archlinux.org` (no GitHub), y requiere generar `.SRCINFO`
      con `makepkg --printsrcinfo`, herramienta exclusiva de Arch
      Linux (validable vía contenedor Docker `archlinux`, sin problema
      técnico ahí).
- [ ] El bloqueo real: hace falta una **cuenta humana** en
      `aur.archlinux.org` (registro con email + verificación) — no se
      puede crear vía API ni automatizar. Sin esa cuenta no hay llave
      SSH que autorizar ni PKGBUILD que publicar. Pendiente de que
      alguien con acceso a `hola@sistematlan.com` la cree.
- [ ] Una vez exista la cuenta: generar PKGBUILD (probablemente vía un
      contenedor `archlinux:latest` con `base-devel` instalado),
      configurar la llave SSH del bot en el perfil de esa cuenta, y
      automatizar el `git push` a `ssh://aur@aur.archlinux.org/mistah-bin.git`
      desde el workflow de release (no hay soporte nativo de
      GoReleaser para AUR — sería un paso custom en release.yml, no
      una sección de `.goreleaser.yaml`).

#### Distribución — Nix flake (implementado sept. 2026, validación incompleta)

- [x] `flake.nix` en la raíz del repo, self-hosted igual que Homebrew/
      Scoop (a diferencia de nixpkgs central, que exige PR + revisión
      humana, mismo tipo de gatekeeping que AUR/winget). Permite `nix
      run github:sistematlan/mistah` / `nix profile install
      github:sistematlan/mistah` sin pasar por ningún registro
      central.
- [x] `buildGoModule` con `proxyVendor = true` — se probó primero sin
      esa opción y falló con "inconsistent vendoring" (el `go mod
      vendor` interno de Nix discrepaba con los requerimientos
      explícitos del `go.mod`, aunque `go mod vendor` corrido
      directamente con Go 1.26.3 funciona sin problema — parece un
      detalle de la implementación de `buildGoModule`, no un problema
      real del proyecto). `proxyVendor = true` es la mitigación que
      la propia documentación de nixpkgs recomienda para esta clase
      de discrepancia.
- [ ] **Validación incompleta**: no se pudo confirmar un `nix build`
      exitoso de punta a punta en esta sesión. El primer intento (sin
      `proxyVendor`) sí llegó hasta la fase de compilación real
      (`Building subPackage .`) antes de fallar por el vendoring —
      confirma que la estructura del flake, el fetch de `self` como
      `src`, y las dependencias de sistema (`go`, `gcc`, toolchain)
      funcionan. El segundo intento (con `proxyVendor = true`, la
      versión que quedó en el repo) se atoró repetidamente
      descargando un solo path desde `cache.nixos.org` sin completar
      en varios intentos de hasta 4 minutos — parece un problema de
      red/infraestructura del entorno sandbox usado en esta sesión,
      no un error del flake, pero no se pudo confirmar con certeza
      total. **Antes de anunciar esta vía públicamente en README/web,
      alguien con un entorno Nix real (o con mejor conectividad a
      cache.nixos.org) debe correr `nix build .#default` y confirmar
      que compila.**
- [ ] `vendorHash = null` quedó como placeholder documentado, no como
      el hash real — Nix nunca llegó a reportarlo porque el build no
      completó. Debe recalcularse en el primer build exitoso.
- [ ] No anunciado todavía en README.md/web/index.html a propósito de
      lo anterior — anunciar una vía de instalación sin haberla
      confirmado funcional sería peor que no tenerla.

#### Soporte Windows — deliberadamente NO portado (sin equivalente razonable)

- [ ] **Time Machine snapshots** (`tmutil`) — VSS/Volume Shadow Copy es
      el concepto más cercano en Windows, pero requiere `vssadmin` con
      privilegios de administrador. mistah nunca eleva permisos; queda
      fuera de alcance salvo que se decida lo contrario explícitamente.
- [ ] **Xcode Simulators** (`xcrun simctl`) — Xcode es exclusivo Apple,
      no hay equivalente conceptual en Windows.
- [ ] **iMessage attachments** — iMessage no existe en Windows.
- [ ] **Mail.app downloads** — sin Outlook equivalente implementado aún
      (se podría agregar: `%LOCALAPPDATA%\Microsoft\Outlook\...`, pero
      el formato de caché de Outlook es menos directo que una carpeta
      simple; queda como follow-up, no como "imposible").
- [ ] **Logs y crash reports genéricos** — Windows usa Event Viewer +
      `%LOCALAPPDATA%\CrashDumps`, arquitectura distinta a "carpeta de
      logs por app" de macOS. Sin detector dedicado todavía.
- [ ] **Firefox en Windows** — el perfil vive en una carpeta con nombre
      aleatorio (`xxxxxxxx.default-release`) que requiere leer
      `profiles.ini` para resolver; se omitió de la tabla fija de
      browsers en vez de adivinar. Follow-up sencillo.
- [ ] **Firma de código / notarización** — el `.exe` no está firmado;
      SmartScreen mostrará "editor desconocido". Requiere certificado de
      firma de código (costo anual), análogo al Apple Developer ID que
      tampoco se ha comprado todavía para macOS.
- [x] **Distribución nativa — Scoop**: implementada (ver "Distribución
      — Scoop" abajo). `winget` (repo central de Microsoft, requiere
      PR + revisión) y `Chocolatey` siguen pendientes; no hay
      `install.ps1` equivalente al `install.sh` de macOS/Linux tampoco.

#### Soporte Linux — implementado (sept. 2026)

Portado con el mismo enfoque que Windows: build tags
(`//go:build linux`), reutilizando toda la orquestación compartida
(`Scan`, `Plan`, `Resolver`, `item.Item`). Confirmado el análisis
previo del backlog: fue más simple que Windows porque la mayoría de
convenciones (XDG Base Directory, freedesktop.org) son estándares
abiertos en lugar de APIs propietarias que hay que envolver.

- [x] `internal/disk`: sin cambios — ya compartía implementación con
      macOS vía `disk_unix.go` (`syscall.Statfs`), y `DirSize` ya era
      portable desde el trabajo de Windows (sin `du`).
- [x] Caches dev: `~/.cache/` (npm, pnpm→`.local/share/pnpm`, yarn, Go,
      pip, uv, Composer, node-gyp) y JetBrains en `~/.cache/JetBrains`.
- [x] Apps instaladas: `internal/apps` parsea archivos `.desktop`
      (`/usr/share/applications`, `/usr/local/share/applications`,
      `~/.local/share/applications`) — el estándar freedesktop.org,
      equivalente al Registro de Windows. "Último uso" es una
      heurística débil (mtime del binario referenciado en `Exec=`),
      documentado como tal, no como precisión falsa.
- [x] Papelera: implementada según la spec real de freedesktop.org
      (`~/.local/share/Trash/{files,info}`) — el remover borra el par
      `files/<name>` + `info/<name>.trashinfo` juntos; borrar solo
      `files/` deja registros huérfanos que confunden a los gestores
      de archivos al intentar restaurar.
- [x] `/tmp`: detector que **solo considera archivos del UID actual**
      (`syscall.Stat_t.Uid`) — a diferencia de `%TEMP%` en Windows,
      `/tmp` es compartido entre todos los usuarios del sistema, así
      que iterar sin filtrar por dueño sería un error de seguridad/
      privacidad, no solo un bug.
- [x] Miniaturas: `~/.cache/thumbnails` — spec de thumbnails de
      freedesktop.org, usada por Nautilus, Dolphin, Thunar, etc.
- [x] Browsers: Chrome, Chromium, Brave, Edge bajo `~/.config/<vendor>/
      <product>/Default/Cache` — mismo layout que Windows, solo la raíz
      cambia de `%LOCALAPPDATA%` a `~/.config`.
- [x] Docker: sin cambios — CLI idéntica. El detector de huérfanos
      apunta a `/var/lib/docker` (dato del daemon, típicamente 0700 —
      se degrada a 0 bytes sin root en vez de fallar, igual que otros
      detectores ante permission-denied).
- [x] `SafeRoots`/`OffLimits`: `/tmp` + perfil de usuario; protege
      `.ssh`/`.gnupg` explícitamente (a diferencia de macOS/Windows,
      en Linux estas SÍ son carpetas de archivos planos dentro de
      `$HOME`, no un almacén gestionado por el OS).
- [x] `install.sh`: extendido para detectar `Linux` vía `uname -s` y
      descargar el tarball correcto — funciona igual dentro de WSL,
      donde `uname` reporta `Linux`, no `Windows` (esto es justo lo
      que permite instalar mistah dentro de una sesión WSL con el
      mismo one-liner que macOS, sin tocar el `.zip` de Windows).
- [x] GoReleaser + CI (`ubuntu-latest`) agregados.
- [x] Validado en Debian 13 (trixie) corriendo dentro de WSL2 en
      Windows real, vía SSH: `scan`, `caches`, `clean --dry-run` y el
      wizard interactivo completo (incluida la ruta de cancelar).

Deliberadamente NO portado (mismo criterio que Windows — sin
equivalente confiable, no simplemente "no hubo tiempo"):

- [ ] **Backups de iPhone / .ipsw** — no existe un cliente oficial de
      iTunes/Apple Devices para Linux. La alternativa de la comunidad
      (`libimobiledevice`) deja elegir un directorio de salida arbitrario
      por invocación, así que no hay una ruta estándar donde apuntar.
- [ ] **Time Machine / snapshots del sistema** — btrfs/LVM snapshots
      existen pero son gestionados por el administrador del sistema,
      no algo que un CLI sin privilegios de root deba tocar.
- [ ] **Mail.app / Adjuntos de iMessage** — exclusivos de Apple.
- [ ] **Logs y crash reports genéricos** — la ubicación varía demasiado
      entre distros/escritorios (`systemd-coredump`, `~/.cache/abrt`,
      etc.) para hardcodear una sola ruta con confianza.
- [ ] **Firefox** — mismo problema de nombre de perfil aleatorio que en
      Windows (requiere leer `profiles.ini`); pendiente como mejora
      compartida entre ambas plataformas.
- [x] **Empaquetado nativo — .deb / .rpm**: implementado sept. 2026
      vía `nfpms` en GoReleaser, publicados como asset directo en cada
      GitHub Release (sin repo APT/YUM propio, sin firma GPG todavía —
      ver "Distribución — .deb/.rpm" abajo). AUR, Snap, y Flatpak
      siguen pendientes.
- [ ] **Linux arm64** — sin publicar todavía (Raspberry Pi, servidores
      ARM); el propio `install.sh` falla explícitamente en esa combinación
      en vez de intentar descargar un artefacto que no existe.

#### Distribución — .deb / .rpm (implementado sept. 2026)

- [x] `.goreleaser.yaml`: sección `nfpms` con `formats: [deb, rpm]`.
      GoReleaser filtra automáticamente por plataforma sin necesitar
      un campo `if` explícito (ese campo resultó ser Pro-only en la
      versión OSS 2.16 que usamos — lo intentamos primero y
      `goreleaser check` lo rechazó).
- [x] Sin repo APT/YUM propio: los paquetes se publican como asset
      directo en el mismo GitHub Release donde ya viven los
      `.tar.gz`/`.zip`. No hay `apt install mistah` desde un
      repositorio de terceros — es `dpkg -i`/`rpm -i` manual, mismo
      nivel de fricción que la descarga manual de macOS/Windows.
- [x] Sin firma GPG de los paquetes — mismo modelo de confianza que el
      resto de assets sin firmar (el usuario decide confiar antes de
      `dpkg -i`/`rpm -i`, igual que con el `.tar.gz` sin firmar).
- [x] Validado con **instalación real** (no solo generación del
      paquete) en contenedores Docker con `--platform linux/amd64`:
      `debian:12` (`dpkg -i` + `mistah version` correcto, binario en
      `/usr/bin/mistah`) y `fedora:latest` (`rpm -i` + `mistah
      version` correcto). Se probó primero con un snapshot local y
      después con los assets reales descargados del release v0.6.2
      publicado — ambas rutas funcionaron igual.
- [ ] Repo APT/YUM propio (permitiría `apt install`/`dnf install` reales
      desde un repositorio agregado) — requiere hostear y mantener un
      servidor de repositorio firmado; no es trivial y queda como
      mejora futura real, no una tarea menor.

### Métricas de salud

- [ ] `mistah doctor`: detecta problemas comunes (Docker sin volumes, brew obsoleto, etc.)
- [ ] `mistah baseline`: guarda snapshot del estado actual para comparar después
- [ ] `mistah diff <baseline>`: muestra qué creció/decreció desde un baseline

---

## 🐛 Deuda técnica conocida

- [ ] `cmd/orphans.go` y `cmd/scan.go` tienen escapes `\u00xx` literales — no afectan output (las strings con comillas dobles se procesan correctamente) pero quedan feos en el código fuente. Reemplazar por caracteres directos.
- [x] `disk.DirSize` — ~~usa `du -sk` (subprocess)~~ reescrito con `filepath.WalkDir` nativo durante el port a Windows (sept. 2026); ya no hay subproceso ni dependencia de `du`, y esto benefició a las 3 plataformas por igual.
- [ ] `caches.Scan()` recorre cada cache path secuencialmente. Paralelizar con goroutines + `errgroup` (gain ~3-5x) — el reemplazo de `du` no resolvió esto, sigue siendo E/S secuencial, solo ya no paga el costo de spawnear un proceso por llamada.
- [ ] No hay manejo de errores diferenciado: si un detector falla, podría dejar la lista vacía. Cada detector debería retornar `(items, []error)` y `Scan()` agregar errores como warnings.
- [ ] Tests no cubren `Scan()` real (solo helpers); son frágiles ante cambios de paths del sistema. Considerar inyección de filesystem (afero o similar) para mockear.
- [ ] `cmd/scan.go` y `cmd/caches.go` duplican lógica de ordenado por `Bytes`. Mover a `internal/item`.

---

## 📌 Decisiones tomadas

- **Licencia**: MIT (a confirmar al hacer LICENSE)
- **Lenguaje**: Go 1.26
- **Distribución primaria**: Homebrew tap `sistematlan/tools` (implementado sept. 2026) + `install.sh`/`go install` para Linux
- **CLI framework**: cobra
- **Telemetría**: cero, jamás
- **Modelo comercial**: pure OSS por ahora; sponsorship + consulting como vías futuras
- **`clean` por defecto**: solo caches; orphans requieren `--include-orphans`, downloads requieren `--include-downloads`
- **UX de confirmación**: ítem por ítem con `[s/N/v/q]`, default seguro en NO
- **Docker**: `system prune -f` (sin `--volumes`); volumes solo con flag explícito futuro `--include-volumes`
- **SafeRoots**: `$HOME`/perfil de usuario + directorio temporal del sistema (`/var/folders`, `/tmp`, `/private/*` en macOS; `os.TempDir()` en Windows). Cualquier otro path es rechazado.
- **Multiplataforma**: macOS + Windows + Linux soportados (ver "Multiplataforma — análisis técnico" arriba, ambos implementados en sesión de sept. 2026).
- **Cobertura objetivo v0.1.0**: caches dev + orphans básicos + downloads. P0 de Application Support (Electron, iOS Simulator, Time Machine, Mail, Messages) queda para v0.2.0.

### Democratización (sesión 4 backlog)

- **Audiencia objetivo expandida**: técnicos no-dev y power users macOS, no consumer mainstream.
- **NO hacer GUI nativa Swift**: convertiría mistah en otro producto, mercado saturado por CleanMyMac, perdería ventaja de auditabilidad open-source.
- **NO hacer TUI bubbletea**: gain marginal sobre Camino A para audiencia adicional.
- **Modo por default**: simple (lenguaje humano, niveles preset). Modo técnico opt-in con `--advanced` o `--verbose`.
- **Wizard sin args**: `mistah` sin subcomando ejecuta wizard con tres niveles preset (Ligera / Estándar / Profunda).
- **Distribución**: Apple notarization + curl one-liner + Homebrew. Requiere Apple Developer Account ($99/año).
- **i18n**: detección automática via `LANG`/`LC_ALL`. Soporte es + en. Sin libs externas — mapas Go nativos.
- **Plan de implementación**: 4 sesiones — (1) i18n+lenguaje humano, (2) wizard+niveles, (3) report+flags globales, (4) distribución+notarización.

### User research / feedback (estrategia)

- **No A/B testing tradicional hasta tener 1000+ usuarios reales.** Requiere telemetría que traiciona la promesa de privacidad.
- **Etapa 1 (primeros 5 usuarios)**: sesiones guiadas 1-on-1 de 30 minutos vía Zoom + share screen. Cualitativo profundo. Jakob Nielsen: 5 usuarios descubren 80% de problemas de UX.
- **Etapa 2 (50-100 usuarios)**: encuesta opt-in al final de `clean` con link manual a Tally/Typeform. URL solo lleva versión + bytes liberados, nunca paths o contenido. Sin recolección automática.
- **Etapa 3 (1000+ usuarios)**: solo entonces considerar telemetría opt-in granular con default OFF, comando explícito (`mistah telemetry enable`), documentación de payload exacto y endpoint open-source para auditar.
- **Reclutamiento de testers iniciales**:
  - [ ] Build in public: tweet/post en X y LinkedIn buscando 5 beta testers macOS
  - [ ] r/macapps, r/golang
  - [ ] Slack/Discord de comunidades técnicas locales
  - [ ] Compañeros devs directos
- **NUNCA**:
  - Telemetría sin consentimiento explícito
  - Recolectar paths, nombres de archivo, contenido
  - Servidor de datos sin código fuente abierto
  - Cookie tracking en landing page

---

## 🎯 Definition of Done para v0.1.0

Del SPEC original:

- [x] `mistah scan` muestra resumen de disco y top categorías
- [x] `mistah apps` lista apps con último uso y tamaño
- [x] `mistah caches` detecta y totaliza caches de dev
- [x] `mistah projects --path ~/sourcecode` reporta estado git de cada repo
- [x] `mistah clean --dry-run` lista candidatos sin borrar nada
- [x] `mistah clean` pide confirmación por ítem
- [x] `mistah downloads` clasifica Downloads por tipo y antigüedad
- [ ] `mistah report --json` emite reporte estructurado
- [x] Binario único, sin dependencias externas en runtime
- [x] `make build` compila en < 10 segundos
- [x] Tests pasan con `make test`

**Faltan:** `report --json` y flags globales. Después MVP completo (10/11 ✓).
