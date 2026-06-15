# mistah — Expansión a usuarios generales (Fase 1)

> Estado: **scope aprobado**. Decisiones de diseño firmadas, listo para
> implementar.
> Última edición: 2026-06-15.

## Por qué este documento

mistah hoy detecta exclusivamente caches y artefactos de desarrollo (npm,
brew, pip, uv, Docker, JetBrains, Xcode). Esa elección hace que la
herramienta sea inútil para cualquier usuario sin entorno de dev: el wizard
les muestra "0 GB" y la promesa de la landing ("Limpia tu Mac, al estilo
open-source") queda desmentida en el primer `mistah`.

Este doc define qué **detectores nuevos** sumamos para que la herramienta
sea relevante en una Mac promedio, sin caer en los antipatrones que
volvieron tóxicas a apps tipo MacKeeper (borrar de más, falsos positivos,
prometer GBs que no existen).

**Audiencia objetivo nueva:** dueño de Mac que no escribe código, usa
iMessage, recibe adjuntos en Mail, sincroniza un iPhone, escucha Spotify,
nunca vacía la papelera.

**Lo que NO cambia en esta fase:**
- El binario sigue siendo CLI. Sin GUI todavía.
- Los detectores actuales (caches/orphans/downloads) se mantienen tal cual.
- El comando `mistah clean --include-orphans --include-downloads` sigue
  funcionando idéntico.

## Principios de diseño para detectores de audiencia general

Estos principios nacen de la diferencia clave: un cache de npm regenera
solo, un backup de iPhone no. La barra de seguridad sube.

1. **Tamaño mínimo para reportar.** No mostrar items <50 MB en la fase
   general; el usuario no ve la diferencia y satura la lista. Excepción:
   Trash, que se reporta siempre porque su semántica es "esto ya es
   basura por definición".

2. **`RiskSafe` solo si el dato es 100% reproducible o ya es basura
   formal.** Trash, snapshots de Time Machine, .ipsw descargados, caches
   de apps. Cualquier cosa que pueda ser "el único respaldo" o "una foto
   que el usuario quería" es `RiskAskBefore` mínimo.

3. **Lista negra explícita (`OffLimits`).** Antes de borrar cualquier
   path, validar contra una lista de paths que mistah NUNCA toca, ni
   por error de detector, ni por flag. Defensa en profundidad sobre
   `cleaner.SafeRoots`.

   Propuesta de `OffLimits`:
   ```
   ~/Documents
   ~/Desktop
   ~/Pictures              (excepto subpaths de cache derivados)
   ~/Movies
   ~/Music                 (excepto subpaths de cache de Music.app)
   ~/Pictures/Photos Library.photoslibrary  (la library entera, sin excepciones)
   ~/Library/Mobile Documents               (iCloud Drive)
   ~/Library/Keychains
   ~/Library/Mail/V*/MailData               (índices, NO Mail Downloads)
   ```

4. **Mostrar contexto humano antes de borrar.** Para iOS backups: nombre
   del device, fecha del último backup, tamaño. Para snapshots: fecha de
   creación. Para Trash: cuántos items, ítem más viejo. El usuario
   general no decide en base a paths absolutos.

5. **Detección oportunista, no exhaustiva.** Si parsear un `.plist`
   falla, el detector skipea ese item con un log silencioso, no rompe
   el scan. Los formatos de Apple cambian entre versiones de macOS;
   cualquier código que dependa de ellos tiene que degradar elegante.

## Categoría nueva en `internal/item`

Hoy hay `CategoryCache`, `CategoryOrphan`, `CategoryDownload`,
`CategoryProject`, `CategoryApp`. Sumamos:

```go
CategorySystem Category = "system"   // Trash, snapshots, Mail Downloads
CategoryDevice Category = "device"   // iOS backups, .ipsw
CategoryAppCache Category = "appcache" // Spotify, Slack, browsers
```

**Por qué no reusar `CategoryCache`:** los caches actuales son todos de
herramientas dev. Mezclarlos con Spotify rompe el filtrado del wizard
(Light hoy = "RiskSafe + path != ''" sobre `inv.Caches` enteros). Una
categoría aparte permite que el wizard ofrezca "modo dev / modo general"
sin tocar la lógica de Light/Standard.

## Fase 1 — 10 detectores

Cada uno definido como un struct conceptual para implementación posterior.

### 1. Trash (Papelera)

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/system/trash.go` |
| Path principal | `~/.Trash` |
| Paths secundarios | `/Volumes/*/.Trashes/<uid>/` para discos externos montados |
| Cómo medir | `disk.DirSize` por path |
| Cómo borrar | `os.RemoveAll` sobre cada hijo del directorio (NO el directorio mismo: `~/.Trash` debe seguir existiendo) |
| Risk | `RiskAskBefore` |
| Categoría | `CategorySystem` |
| Detalle UI | "X items, más viejo de hace Y días" |
| Edge cases | Volúmenes desmontados (skipear), permisos en Trashes de otros usuarios (skipear), archivos `.DS_Store` propios del FS (skipear) |
| Test | TempDir simulando `.Trash` con archivos de varias edades; verificar que el detector reporta correctamente y que el remover deja el dir vacío pero existente |

**Por qué `RiskAskBefore` y no `RiskSafe`:** la papelera es un buffer
intencional. El usuario la puso ahí pero puede recuperar. Borrar sin
preguntar viola la promesa de macOS de "la papelera sigue existiendo
hasta que tú digas". Una sola confirmación al ver el contenido es
suficiente — no por ítem.

### 2. iOS Software Updates (.ipsw)

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/device/ios_updates.go` |
| Path | `~/Library/iTunes/iPhone Software Updates/` |
| Cómo medir | listar `*.ipsw`, sumar `os.Stat().Size()` |
| Cómo borrar | `os.Remove` por archivo |
| Risk | `RiskSafe` |
| Categoría | `CategoryDevice` |
| Detalle UI | "Actualización para <device> versión <X>, <tamaño>" parseando el filename (formato típico: `iPhone15,2_17.4_21E219_Restore.ipsw`) |
| Edge cases | Filename no parseable → mostrar nombre crudo. Path inexistente → no es error. |
| Test | TempDir con .ipsw mock, verificar parsing del filename y borrado limpio |

**Justificación `RiskSafe`:** son redescargables desde Apple. Si el
usuario va a actualizar el iPhone de nuevo, macOS los baja otra vez. Cero
pérdida de datos.

### 3. Time Machine local snapshots

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/system/snapshots.go` |
| Cómo listar | `tmutil listlocalsnapshots /` (un comando por volumen montado relevante) |
| Cómo medir | tmutil **no** reporta tamaño directo. Opciones: (a) ejecutar `du -sh` sobre `/.MobileBackups` (deprecado), (b) usar `diskutil apfs list` y leer `Snapshot Used Space`, (c) reportar conteo de snapshots y dejar el tamaño en "—" mostrando solo cuántos hay. **Decisión propuesta: (c).** Tamaño exacto en APFS es complejo y los snapshots se liberan automáticamente cuando el FS necesita espacio; lo que vendemos es "macOS está reteniendo X snapshots viejos, podemos pedirle que los suelte". |
| Cómo borrar | `tmutil deletelocalsnapshots <date>` por cada snapshot listado |
| Risk | `RiskSafe` |
| Categoría | `CategorySystem` |
| Detalle UI | "X snapshots locales — macOS los recreará si los necesita" |
| Edge cases | `tmutil` no instalado / sin permisos (no debería pasar en macOS estándar pero defenderse): skipear con log. Sistema sin snapshots: no aparece el item. |
| Test | mockear el comando `tmutil` con un binario fake en `$PATH` durante el test (patrón ya usado en `cleaner` para docker) |

**Riesgo de seguridad reportado:** ninguno conocido. Los local snapshots
son una optimización de Time Machine, no un backup primario. Apple los
borra automáticamente cuando el disco está al 80%. Forzar borrado es una
operación soportada por la API pública.

### 4. Mail Downloads

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/system/mail.go` |
| Path | `~/Library/Containers/com.apple.mail/Data/Library/Mail Downloads` |
| Cómo medir | `disk.DirSize` |
| Cómo borrar | `os.RemoveAll` sobre subdirectorios (no el dir raíz; Mail.app lo recrea pero mejor no romperle el árbol) |
| Risk | `RiskSafe` |
| Categoría | `CategorySystem` |
| Detalle UI | "Adjuntos descargados de Mail — se redescargan al abrir el correo" |
| Edge cases | Path no existe (usuarios que nunca abrieron Mail.app) → no aparece. Sandbox de macOS puede impedir lectura: stat error → skip silencioso. |
| Test | TempDir con dir mock |

**Justificación `RiskSafe`:** el contenido del correo en sí está en el
servidor IMAP/POP. Lo que se borra acá es solo el cache de adjuntos
descargados. Mail.app los pide de nuevo si el usuario los abre.

### 5. iOS device backups

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/device/ios_backups.go` |
| Path | `~/Library/Application Support/MobileSync/Backup/<UDID>/` (un dir por device) |
| Cómo medir | `disk.DirSize` por dir UDID |
| Cómo borrar | `os.RemoveAll` por dir UDID |
| Cómo identificar el device | leer `<UDID>/Info.plist` (XML plist), extraer `Device Name`, `Product Type`, `Last Backup Date` |
| Risk | `RiskAskBefore` |
| Categoría | `CategoryDevice` |
| Detalle UI | "<Device Name> (<Product Type>) — último backup hace X días, Y GB" |
| Edge cases | `Info.plist` corrupto o ausente → mostrar UDID crudo, marcar `Risk: RiskAskBefore` siempre. Backup encriptado → seguir siendo borrable, no es relevante para nosotros. UDID dir vacío → ignorar. |
| Test | TempDir con UDID dirs mock + Info.plist sintético; verificar parseo y fallback a UDID si plist falla |

**Justificación `RiskAskBefore` (no Safe):** un backup de iPhone puede
ser irrecuperable si el device se perdió/rompió y nunca se subió a
iCloud. La probabilidad es baja pero la consecuencia es alta. Per-item
prompt es obligatorio. **NUNCA** se incluye en una fase auto del wizard.

### 6. App caches (Spotify, Slack, Discord, Zoom, Teams, Notion, Figma, Arc, etc.)

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/appcache/appcache.go` |
| Paths | dos formas: (a) `~/Library/Caches/<bundle-id>` y (b) `~/Library/Application Support/<app>/Cache` (algunas apps caches viven adentro del Application Support, fuera del sandbox) |
| Cómo medir | `disk.DirSize` por path, suma por app |
| Cómo borrar | `os.RemoveAll` sobre el dir cache, dejando el dir padre intacto si hay otra config dentro |
| Risk | `RiskSafe` |
| Categoría | `CategorySystem` |
| Detalle UI | "Cache de <App> — la app lo regenera al usarse" |
| Lista inicial de apps | Spotify (`com.spotify.client`), Slack (`com.tinyspeck.slackmacgap`), Discord (`com.hnc.Discord`), Zoom (`us.zoom.xos`), Teams (`com.microsoft.teams2`), Notion (`notion.id`), Figma (`com.figma.Desktop`), Arc (`company.thebrowser.Browser`), Telegram (`ru.keepcoder.Telegram`), Linear (`com.linear`) |
| Estructura | tabla `[]appcacheEntry` con `bundleID, displayName, paths []string`. Iteración trivial. Sumar apps después es solo agregar entries, no código. |
| Edge cases | App no instalada → ningún path existe → no se reporta. Cache muy chico (<10 MB) → no se reporta. Cache enorme (>500 MB) → marcar el detail con "(crece sin techo)" para que el usuario entienda por qué. |
| Test | TempDir con dirs mock por bundle ID, verificar que solo reporta los que existen y borrado limpio |

**Justificación `RiskSafe`:** son caches por definición. Spotify
re-cachea las canciones offline al volver a reproducirlas. Slack
re-descarga emojis y avatares. Cero riesgo de pérdida.

**Por qué `CategorySystem` y no `CategoryCache`:** los caches en
`CategoryCache` son todos de herramientas dev y el wizard los filtra
diferente (Light usa solo `RiskSafe`+`Path != ""` sobre `inv.Caches`).
Mezclar Spotify ahí rompería ese filtro o requeriría refactor. Más
limpio que viva en `CategorySystem` y se sume al bucket Light por
selección explícita en el wizard.

### 7. Browser caches (Chrome, Safari, Firefox, Arc, Brave, Edge)

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/appcache/browsers.go` |
| Paths | varían por browser; lista en la tabla más abajo |
| Cómo medir | `disk.DirSize` por path |
| Cómo borrar | `os.RemoveAll` sobre el dir cache. **NUNCA** tocar `Cookies`, `Bookmarks`, `Login Data`, `History` u otros que no sean `Cache` puro. |
| Risk | `RiskSafe` |
| Categoría | `CategorySystem` |
| Detalle UI | "Cache de <Browser> — el browser lo regenera al navegar" |
| Edge cases | Browser corriendo durante el borrado puede dejar archivos lockeados → `os.RemoveAll` falla parcial → reportar como skipped, no como error. Idealmente el wizard avisa "cierra el browser para mejor resultado" en el banner del nivel Light. |

Tabla de paths por browser:

| Browser | Path |
|---|---|
| Chrome | `~/Library/Caches/Google/Chrome` |
| Chrome (Application Support cache) | `~/Library/Application Support/Google/Chrome/Default/Cache`, `~/Library/Application Support/Google/Chrome/Default/Code Cache`, `~/Library/Application Support/Google/Chrome/Default/GPUCache` |
| Safari | `~/Library/Caches/com.apple.Safari` |
| Firefox | `~/Library/Caches/Firefox/Profiles/*/cache2` |
| Arc | `~/Library/Caches/Company.ThebrowserCompany.Browser` |
| Brave | `~/Library/Caches/BraveSoftware/Brave-Browser` |
| Edge | `~/Library/Caches/Microsoft Edge` |

**Test:** TempDir con perfiles mock de cada browser, verificar que el
detector solo apunta a paths cache y NUNCA toca cookies/bookmarks/etc.
Test de regresión obligatorio: si alguien agrega un path al detector,
el test debe fallar a menos que el path nuevo sea explícitamente cache.

### 8. Logs y crash reports

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/system/logs.go` |
| Paths | `~/Library/Logs/`, `~/Library/Logs/DiagnosticReports/` (crashes), `~/Library/Application Support/CrashReporter/` |
| Paths que NO se tocan | `/private/var/log/` (requiere sudo, no nuestro problema) |
| Cómo medir | `disk.DirSize` total, separado por crashes y logs regulares |
| Cómo borrar | `os.RemoveAll` sobre cada subdirectorio dejando el dir padre. Para `DiagnosticReports`, borrar solo `*.crash`, `*.diag`, `*.ips` con mtime > 30 días (no borrar todos: a veces el usuario está debuggeando algo reciente). |
| Risk | `RiskSafe` |
| Categoría | `CategorySystem` |
| Detalle UI | "Logs antiguos del sistema y reportes de crashes" |
| Edge cases | Permisos: algunos logs pueden ser de root y `os.Remove` falla con permission denied. Skipear silencioso, sumar al "skipped" del summary. |
| Test | TempDir con archivos de varias edades, verificar que solo borra los >30 días en DiagnosticReports y todo lo demás en Logs. |

### 9. QuickLook y system thumbnails

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/system/thumbnails.go` |
| Paths | `~/Library/Caches/com.apple.QuickLook.thumbnailcache/`, `/var/folders/<hash>/<hash>/C/com.apple.QuickLook.thumbnailcache/` (este último por usuario, vía `getconf DARWIN_USER_CACHE_DIR`) |
| Cómo medir | `disk.DirSize` |
| Cómo borrar | `os.RemoveAll` sobre el dir, macOS lo recrea on demand |
| Risk | `RiskSafe` |
| Categoría | `CategorySystem` |
| Detalle UI | "Thumbnails de QuickLook — macOS los regenera al previsualizar" |
| Edge cases | Path `/var/folders/...` puede no estar en `SafeRoots` si el detector se ejecuta con un home distinto al usuario actual. **Verificar:** `SafeRoots` ya incluye `/var/folders` y `/private/var/folders` (vimos en `cleaner.go:204`). OK. |
| Test | TempDir, borrado limpio, verificar no se toca nada fuera del path |

### 10. Xcode iOS Simulators viejos

| Campo | Valor |
|---|---|
| Paquete propuesto | `internal/dev/xcode_simulators.go` (carpeta `internal/dev` nueva, agrupa lo dev avanzado más allá de caches) |
| Cómo listar | `xcrun simctl list devices --json` retorna JSON con runtimes y devices. Filtrar `isAvailable: false` (runtime obsoleto) o devices marcados `unavailable, runtime profile not found`. |
| Cómo medir | `~/Library/Developer/CoreSimulator/Devices/<UUID>/` por device, `disk.DirSize` |
| Cómo borrar | `xcrun simctl delete <UUID>` (más limpio que `rm -rf` porque actualiza el índice de Xcode), con fallback a `os.RemoveAll` si el comando falla |
| Risk | `RiskAskBefore` |
| Categoría | `CategoryCache` (se queda con dev por su naturaleza) |
| Detalle UI | "<Device Name> · iOS <X.Y> (runtime no disponible)" |
| Edge cases | `xcrun` no instalado (Xcode no presente) → detector no aplica, retorna lista vacía. Comando falla con exit code → log silencioso, skipear ese device. |
| Test | mockear `xcrun` con un binario fake en `$PATH` (mismo patrón que para `tmutil` y `docker`) |

**Por qué `CategoryCache` y no `CategoryDevice`:** Aunque suene a
"device", estos simuladores son artefactos de desarrollo de Xcode, no
backups de devices reales. Son del bucket dev del wizard. La distinción
importa para el copy: el usuario no-dev nunca debe verlos.

**Por qué `RiskAskBefore`:** algún simulador puede tener datos de
prueba que el dev quiera (capturas, builds testeados). No vale la pena
asumir.

## Integración con el wizard

Hoy el wizard tiene 3 niveles (Light/Standard/Deep) que escalan por
profundidad técnica. Esa estructura no funciona para audiencia mixta:
un no-dev no entiende qué significa "Deep = Standard + orphan data +
Downloads candidates".

Propuesta: **mantener los 3 niveles pero redefinir el contenido** para
que cubran tanto dev como general.

```
Light    — App caches (Spotify, Slack, Zoom...) + Browser caches +
           Trash + Mail Downloads + .ipsw + QuickLook thumbnails +
           Caches dev seguros (npm, brew, pip)
           Todo RiskSafe. Una sola confirmación.

Standard — Light + Docker prune + JetBrains old + Xcode artifacts +
           Time Machine snapshots + Logs y crash reports
           Todo cache/sistema reproducible. Una sola confirmación.

Deep     — Standard + orphans + Downloads candidates + iOS backups +
           Xcode simulators viejos
           Cualquier RiskAskBefore va a fase review (per-item prompt).
           Esto extiende el patrón ya implementado en wizard.PlanForSplit.
```

Cuando el usuario corre el wizard, mistah detecta automáticamente qué
buckets están vacíos y los oculta del menú. Un dev sin iPhone nunca ve
"iOS backups". Un no-dev sin Docker ni Xcode nunca ve "Docker prune"
ni "Xcode simulators".

## Lo que NO entra en Fase 1

Por costo de implementación, riesgo o porque hay productos comerciales
que ya lo hacen mejor:

- **Detección de duplicados (fotos, videos, PDFs).** Necesita hashing
  rolling y UI de revisión por par. Productos como Gemini 2 ya lo hacen
  bien y cobran por eso. No es nuestra batalla todavía.

- **Apps no usadas.** Heurística por `kMDItemLastUsedDate` falla con
  apps lanzadas vía Alfred/Raycast. Riesgo de borrar Final Cut Pro
  porque el usuario lo abre por Spotlight, no por Dock.

- **Photos Library cache derivado.** Tocar la library de Photos puede
  corromperla. Demasiado riesgo para el upside.

- **Mensajes (iMessage) attachments.** `~/Library/Messages/Attachments`
  puede tener GBs pero borrarlos rompe la vista de los chats. Necesita
  diseño cuidadoso (¿solo viejos? ¿solo de chats archivados?). Fase 2.

- **Logs de sistema en `/private/var/log`.** Requieren sudo, mistah no
  pide privilegios elevados por diseño. Fuera de scope permanente.

## Decisiones de diseño aprobadas

Estas eran las preguntas abiertas del primer borrador. Quedan firmadas
así para que la implementación no las re-abra.

### 1. `OffLimits` se enforce en el cleaner (segunda barrera defensiva)

La validación vive en `internal/cleaner/cleaner.go`, junto a `SafeRoots`,
no en cada detector. Razón: aunque mañana metamos un detector mal
escrito que apunte a `~/Documents`, el cleaner se niega a ejecutar.
Defensa en profundidad. El detector decide qué reportar; el cleaner
decide qué se anima a borrar.

Implementación propuesta:

```go
// internal/cleaner/cleaner.go

// OffLimits lists path prefixes that mistah will NEVER delete from,
// regardless of what any detector reported. Resolved against the user's
// home directory at init time.
var OffLimits = func() []string {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil
    }
    return []string{
        filepath.Join(home, "Documents"),
        filepath.Join(home, "Desktop"),
        filepath.Join(home, "Movies"),
        // ~/Pictures: blocked at the root, but specific cache subpaths
        // (e.g. Photos Library derivatives) may be added by detectors
        // and validated individually if we ever expand there.
        filepath.Join(home, "Pictures"),
        filepath.Join(home, "Music"),
        filepath.Join(home, "Library/Mobile Documents"),     // iCloud Drive
        filepath.Join(home, "Library/Keychains"),
        filepath.Join(home, "Library/Application Support/AddressBook"),
        filepath.Join(home, "Library/Calendars"),
    }
}()

// ErrOffLimits is returned when a remover is asked to touch a protected path.
var ErrOffLimits = errors.New("path is off-limits; mistah refuses to delete user data here")

// isOffLimits returns true iff abs is, or lives under, any OffLimits prefix.
func isOffLimits(abs string) bool {
    for _, root := range OffLimits {
        if abs == root || strings.HasPrefix(abs, root+string(os.PathSeparator)) {
            return true
        }
    }
    return false
}
```

`PathRemover.Remove` agrega el check antes de `os.RemoveAll`:

```go
if isOffLimits(abs) {
    return fmt.Errorf("%w: %s", ErrOffLimits, abs)
}
```

Tests obligatorios para PR 1:

- `Remove` rechaza `~/Documents/foo.txt` aunque venga de un detector.
- `Remove` rechaza `~/Pictures/Photos Library.photoslibrary`.
- `Remove` permite `~/.Trash/old.dmg` (no entra en OffLimits).
- `Remove` permite `~/Library/Caches/com.spotify.client` (no entra).
- Boundary: `~/Documents-old` (similar pero distinto) NO se confunde
  con `~/Documents`.

### 2. Categorías nuevas: `CategorySystem` y `CategoryDevice`

Vamos con todo: dos categorías nuevas en `internal/item/item.go`. Razón:
buscamos detectar el máximo de archivos eliminables sin mezclar
semántica. Los caches dev son una cosa, el sistema operativo es otra,
los devices conectados son otra.

```go
const (
    CategoryCache    Category = "cache"
    CategoryOrphan   Category = "orphan"
    CategoryDownload Category = "download"
    CategoryProject  Category = "project"
    CategoryApp      Category = "app"
    CategorySystem   Category = "system"   // Trash, snapshots, Mail downloads, app caches
    CategoryDevice   Category = "device"   // iOS backups, .ipsw
)
```

Las constantes existentes no se renombran (compatibilidad de API
interna). Las nuevas se suman al final.

### 3. Wizard: copy genérico con superpoder dev opcional

El menú del wizard NO cambia su tagline base ("Limpia tu Mac"). El
copy queda apuntando a usuario general, y cuando mistah detecta
herramientas de dev (npm, brew, Docker, JetBrains, Xcode), agrega un
sub-banner discreto del tipo:

```
What kind of cleanup would you like?

  Detected dev tools — your developer caches will be included too.

  1) Light       12 GB  — App caches, Trash, Mail downloads, iOS updates
  2) Standard    34 GB  — Light + Docker prune, JetBrains, Xcode, snapshots
  3) Deep        58 GB  — Standard + orphans + Downloads + iOS backups (asks per item)
  4) Cancel
```

Si NO hay tools dev, el banner desaparece silenciosamente y los items
relacionados quedan ausentes del cómputo. Cero menciones a npm/Docker
en el copy si el usuario no los tiene.

Implementación: una función `wizard.detectDevPresence() bool` que stat-ea
3-4 paths conocidos. Si retorna true, el menú imprime la línea extra y
usa la i18n key `wizard.menu.dev-detected`.

Esto resuelve la promesa "le ayudamos al dev a mantener limpia su
máquina" sin sacrificar el posicionamiento general.

### 4. Versión del release final: `v0.2.0`

Brinco de minor justificado. Es expansión de scope significativa,
agrega categorías nuevas, redefine los niveles del wizard, cambia el
copy de la landing. SemVer minor bump aplica limpio.

Tag final cuando los 11 pasos del plan estén mergeados (10 PRs +
release tag). PRs intermedios NO se taguean — quedan en `main`
acumulando hasta que el conjunto esté completo.

## Estimación de impacto

Sobre una Mac promedio de un usuario no-dev (3 años de uso, iPhone
sincronizado, Mail.app activo, Spotify, Slack, navega con Chrome):

| Detector | GB típicos | Confianza |
|---|---|---|
| iOS device backups | 4-15 | alta — depende de cuántos devices |
| Trash | 0.5-5 | alta |
| Mail Downloads | 0.5-3 | media |
| Time Machine snapshots | 5-40 | alta — varía mucho |
| iOS .ipsw | 0-8 | media — depende si actualizó iPhone recientemente |
| App caches (Spotify, Slack, Zoom, etc.) | 2-8 | alta |
| Browser caches | 1-5 | alta |
| Logs y crash reports | 0.5-3 | media |
| QuickLook thumbnails | 0.2-2 | alta |
| Xcode simulators (solo si tiene Xcode) | 0-20 | media |
| **Total Fase 1 (Mac general)** | **14-89 GB** | **media-alta** |
| **Total Fase 1 (Mac dev)** | **20-110 GB** | **media-alta** |

Para comparación, lo que mistah detecta hoy en una Mac de no-dev: <1 GB.

## Plan de implementación

Aprobado el scope, los PRs van en este orden. Cada uno es independiente,
testeable y revertible. Ningún PR intermedio se taguea — todos se
acumulan en `main` hasta que la fase esté completa.

1. **PR 1 — Infra:** `CategorySystem` y `CategoryDevice` en `internal/item`,
   `OffLimits` + `ErrOffLimits` + check en `PathRemover.Remove` en
   `internal/cleaner`. Tests del check defensivo (5 casos enumerados arriba).
   Cero detectores nuevos en este PR — solo cimientos.

2. **PR 2 — Detectores triviales (`RiskSafe`, un path c/u):**
   Trash, .ipsw, Mail Downloads, QuickLook thumbnails. Los 4 más
   simples, mismo patrón de implementación, mismo patrón de test. Un
   solo PR para no fragmentar.

3. **PR 3 — App caches:** paquete `internal/appcache` con la tabla de
   apps. Spotify, Slack, Discord, Zoom, Teams, Notion, Figma, Arc,
   Telegram, Linear. Test que cubra "app instalada / no instalada /
   cache muy chico ignorado".

4. **PR 4 — Browser caches:** mismo paquete `internal/appcache`,
   submódulo `browsers.go`. Chrome, Safari, Firefox, Arc, Brave, Edge.
   Test de regresión: el detector NUNCA reporta paths que no terminen
   en `Cache`, `Code Cache`, `GPUCache` o `cache2`.

5. **PR 5 — Logs y crash reports:** `internal/system/logs.go`. Cuidado
   con la regla de mtime > 30 días en DiagnosticReports.

6. **PR 6 — Time Machine snapshots:** mock de `tmutil` en tests
   (mismo patrón que ya hay para `docker`). Wrapper que parsea
   `tmutil listlocalsnapshots /` y orquesta `deletelocalsnapshots`.

7. **PR 7 — iOS device backups:** `internal/device/ios_backups.go`,
   parseo de `Info.plist` con fallback a UDID crudo. `RiskAskBefore`
   estricto, NUNCA cae en bucket auto.

8. **PR 8 — Xcode iOS Simulators:** `internal/dev/xcode_simulators.go`,
   mock de `xcrun simctl`.

9. **PR 9 — Wizard rebalanceado:** redistribuir Light/Standard/Deep
   con los 10 detectores nuevos. Banner "Detected dev tools" cuando
   `detectDevPresence()` retorna true. Actualizar i18n ES/EN. Refrescar
   los tests del wizard (`TestPlanFor_*`, `TestPlanForSplit_*`).

10. **PR 10 — Landing y README:** quitar npm/brew/Docker del hero del
    landing, copy nuevo enfocado en "tu Mac" sin condicionar al stack
    dev. Sección "también para devs" como segunda mitad. README con
    los detectores nuevos en una tabla.

11. **Tag `v0.2.0`** y release. Goreleaser publica binarios.

Cada PR debe pasar `go test ./...` y `go vet ./...` verde antes de
merge. El CI ya está configurado para correr eso en cada push a `main`.

---

# Fase 2 — Consistencia y mensajería

> Estado: **scope aprobado**. Listo para implementar.
> Última edición: 2026-06-15.

Fase 1 dejó dos frentes abiertos. Fase 2 los cierra. Es deliberadamente
pequeña: dos entregas, no diez. No agrega categorías nuevas ni reabre
las decisiones de diseño firmadas en Fase 1.

## Frente A — Cablear los detectores a `clean` y `scan` (deuda de Fase 1)

### El problema

En Fase 1 cableamos los 30 detectores al **wizard** (`mistah` sin args),
pero NO al comando granular `mistah clean` ni al informativo `mistah scan`.
Hoy:

- `mistah clean` solo conoce caches dev, orphans y downloads (~18 detectores).
- `mistah scan` solo reporta caches dev y orphans.
- El wizard conoce los 30 (system, device, appcache, dev incluidos).

Esto es una inconsistencia de producto: un power user que prefiere el
control granular de `clean` no ve su papelera, sus backups de iPhone, ni
sus snapshots de Time Machine. La capacidad existe pero está escondida
detrás del wizard.

### La solución

Centralizar la recolección de inventario en un solo lugar que tanto el
wizard como `clean`/`scan` consuman. Hoy `wizard.Scan()` ya orquesta los
6 detectores; el problema es que vive en el paquete `wizard` y los cmd
no deberían importar `wizard` (sería raro que `clean` dependa del wizard).

Opción elegida: **mover la orquestación a un paquete neutral**
`internal/inventory` que ni wizard ni cmd "posean". Ambos lo consumen.

```
internal/inventory/inventory.go
  - type Inventory struct { Caches, Orphans, Downloads, System, Device, DevAdvanced }
  - func Scan() (Inventory, error)   // mueve aquí wizard.Scan()
  - func (Inventory) All() []item.Item  // flatten para clean/scan
```

`wizard.Inventory` se vuelve un alias o se reexporta desde `inventory`
para no romper los tests del wizard. La lógica de `PlanFor`/`PlanForSplit`
se queda en `wizard` (es UX del wizard, no del inventario).

### Cambios en cmd

- `mistah scan`: agrupa el reporte por categoría (Sistema / Dispositivos /
  Apps / Dev) en vez de solo "Caches dev" y "Huérfanos". Sigue siendo
  read-only.
- `mistah clean`: nuevas flags para incluir los buckets nuevos, por
  defecto conservador (solo lo RiskSafe sin prompt; lo RiskAskBefore
  siempre pregunta por ítem, como ya hace).

  ```
  mistah clean                      # caches dev (comportamiento actual, sin cambios)
  mistah clean --include-system     # + papelera, app/browser caches, snapshots, logs…
  mistah clean --include-device     # + backups iOS, .ipsw
  mistah clean --all                # todo, equivalente al Deep del wizard
  ```

  `--include-orphans` y `--include-downloads` ya existen; se mantienen.

### Riesgo

Bajo. No hay detectores nuevos; solo se expone lo ya testeado a través de
otra puerta. El `OffLimits` y el split por Risk del cleaner siguen
protegiendo igual. El único cuidado es no cambiar el comportamiento por
defecto de `mistah clean` (debe seguir limpiando solo caches dev sin
flags, para no sorprender a quien ya lo usa en scripts).

### PRs del Frente A

1. **PR A1 — Paquete `internal/inventory`:** mover `wizard.Scan()` y el
   tipo `Inventory` allí. `wizard` reexporta para compatibilidad. Tests
   de que el inventario agrega todos los buckets.
2. **PR A2 — `scan` agrupado:** reescribir `cmd/scan.go` para reportar
   por categoría usando `inventory.Scan()`.
3. **PR A3 — `clean` con flags nuevas:** `--include-system`,
   `--include-device`, `--all`. Default sin cambios. Actualizar `--help`
   e i18n.

## Frente B — iMessage attachments

### El problema

`~/Library/Messages/Attachments/` guarda cada foto, video, audio y archivo
que pasó por iMessage. En Macs con iMessage intenso puede ser 10-40 GB.
Pero es data sensible: borrar un adjunto rompe su preview en el chat (el
texto queda, el adjunto muestra un ícono roto).

### La solución — conservadora

NO borramos por chat ni intentamos parsear `chat.db` (la base SQLite de
Messages). Eso es frágil y peligroso. En su lugar:

- Detectar adjuntos **por antigüedad de mtime**. Solo adjuntos con más de
  N meses (propuesta: 6 meses, configurable) se consideran candidatos.
- `RiskAskBefore` **estricto**. NUNCA cae en bucket auto del wizard;
  siempre fase review per-item, igual que iOS backups.
- Reportar como **un solo Item agregado** con el total de bytes de
  adjuntos viejos y el conteo, NO un item por archivo (serían miles).
- El remover borra solo los archivos > N meses dentro de Attachments,
  reusando el patrón de `OldFilesRemover` que ya existe (pero sin filtro
  de extensión — aquí cualquier archivo viejo cuenta).

### Detalle técnico

| Campo | Valor |
|---|---|
| Paquete | `internal/system/messages.go` (junto a Mail, mismo dominio) |
| Path | `~/Library/Messages/Attachments/` |
| Estructura | árbol de subdirs hash (`ab/cd/<guid>/archivo`); hay que walk recursivo |
| Cómo medir | sumar bytes de archivos con mtime > 6 meses |
| Cómo borrar | `OldFilesRemover` con MaxAgeDays=180 y Extensions vacío (= cualquier ext) |
| Risk | `RiskAskBefore` SIEMPRE |
| Categoría | `CategorySystem` |
| Edge cases | permisos (Messages está en un contenedor con TCC; lectura puede fallar sin Full Disk Access → skip silencioso con log); árbol vacío → no item |

### Cambio necesario en `OldFilesRemover`

Hoy `OldFilesRemover` exige `Extensions` no vacío y NO recursa. Para
iMessage necesitamos:
- `Extensions` vacío = "cualquier archivo" (matchea todo).
- Recursión dentro del árbol de hash de Attachments.

Decisión: agregar un campo `Recursive bool` al `OldFilesRemover` y tratar
`len(Extensions) == 0` como "match all". Cambio retrocompatible: los usos
actuales (crash reports) pasan Extensions no vacío y Recursive=false, así
que su comportamiento no cambia. Tests existentes deben seguir verdes.

### Riesgo

Medio. Mitigado por: RiskAskBefore estricto (siempre prompt), filtro de
6 meses (no toca conversaciones recientes), agregación en un solo item
(el usuario decide una vez, informado del total y conteo), y el hecho de
que el texto del chat NUNCA se toca (vive en chat.db, que no tocamos).

El peor caso realista: el usuario borra adjuntos viejos y luego scrollea
a una conversación de hace un año y ve íconos rotos donde había fotos.
El Detail del item debe advertir esto explícitamente.

### PRs del Frente B

4. **PR B1 — `OldFilesRemover` extendido:** campo `Recursive`, semántica
   `Extensions` vacío = match all. Tests de retrocompatibilidad (crash
   reports siguen igual) + tests de los modos nuevos.
5. **PR B2 — Detector iMessage:** `internal/system/messages.go`,
   `scanMessagesAttachments(home)`, filtro 6 meses, agregación. i18n.
   Cablear a `system.ScanHome`. Tests con árbol de attachments sintético.

## Versión del release

`v0.3.0`. El Frente A es expansión de comportamiento (flags nuevas en
clean, scan reagrupado); el Frente B agrega un detector. Minor bump.

## Lo que sigue fuera de scope (Fase 3+)

- Duplicados por hash (fotos, PDFs, videos).
- Apps no usadas (heurística de Spotlight frágil).
- Photos Library cache derivado.
- Apple notarization (no es feature de limpieza; es trabajo de release).
