# Asset Delivery

This document describes the current delivery contract: the public routes, how
visual assets are resolved, what is cached, and what is deliberately not
solved yet.

## Model

Every asset has a `role`:

- `original` — the uploaded file.
- `visual` — a derived image asset for audio/video/pdf.

A visual asset points at its parent through `image_of`. Images are their own
visual source and do not get a derived visual asset.

Upload / replace / regenerate creates visual assets for:

| Source | Visual asset |
| --- | --- |
| image | none — the image is the visual source |
| audio | waveform PNG (ffmpeg) |
| video | poster JPEG (ffmpeg) |
| pdf | first-page JPEG (pdftoppm) |

Derived visual assets are capped at 1920 on the long edge. Larger delivery
sizes are produced by imgproxy on demand; no 400px derivative is stored.

Visual assets are excluded from listings, search, stats, zip downloads, backup,
and collection/tag counts. They are deleted with their parent and regenerated in
place, so the visual asset ID stays stable. A missing visual asset is generated
lazily on the first `/img` or authenticated preview request.

imgproxy is the conversion source of truth. Extraction of audio/video/pdf is
still done by ffmpeg/pdftoppm because community imgproxy cannot decode those
formats (PDF and MP4 are Pro features; audio is unsupported).

## Routes

### `GET /img/{assetId}/{spec}` — public visual delivery

Requires a spec. Bare `GET /img/{assetId}` is not registered.

Supported specs:

- `{preset}.{ext}` — named preset, e.g. `/img/{id}/md.webp`.
- `~{signed}.{ext}` — ad-hoc signed imgproxy pipeline.

Resolution:

1. image asset → the asset itself
2. audio/video/pdf → the asset with `role='visual' AND image_of={assetId}`
3. no image source → 404

The spec is applied to the resolved source. Preset scoping and signed
verification use the parent asset ID, while the pixels come from the visual
asset for non-images.

Formats: `jpg`, `png`, `webp`, `gif`, `avif`.

Examples:

```html
<img
  src="/img/ASSET_ID/md.webp"
  srcset="
    /img/ASSET_ID/xs.webp 320w,
    /img/ASSET_ID/sm.webp 640w,
    /img/ASSET_ID/md.webp 1280w,
    /img/ASSET_ID/lg.webp 1920w
  "
  sizes="(max-width: 640px) 100vw, 50vw"
  alt=""
/>

<video controls poster="/img/ASSET_ID/sm.webp">
  <source src="/src/ASSET_ID" type="video/mp4" />
</video>

<img src="/img/ASSET_ID/xs.webp" alt="PDF thumbnail" />
```

Signed usage:

```js
const { segment, url } = await fetch('/api/imgproxy/sign', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    asset_id: 'ASSET_ID',
    format: 'webp',
    processing: 'rs:fit:800:600/q:82',
  }),
}).then((r) => r.json())

// Either the returned URL (assets subdomain) or the /img form:
// /img/ASSET_ID/${segment}
```

Current limitation: `/api/imgproxy/sign` validates the requested asset as an
image, so signed custom pipelines are currently image-only. Presets work for
audio/video/pdf through `image_of`.

### `GET /src/{assetId}` — public original delivery

- Only `role='original'` assets with `audio/*`, `video/*`, or
  `application/pdf`.
- Images and derived visual assets return 404.
- Serves the stored original with `Content-Type`, inline
  `Content-Disposition`, and Range support when the storage reader is
  seekable.
- `?download=1` switches to `attachment`.
- `Cache-Control: public, max-age=3600`.

```html
<a href="/src/ASSET_ID" target="_blank">Open PDF</a>
<a href="/src/ASSET_ID?download=1">Download audio</a>
```

### `GET /api/assets/{id}/content` — authenticated original

Session-protected original bytes for the app. Works for every asset type,
including images. `Cache-Control: private, max-age=3600`; Range support for
seekable storage.

### `GET /api/assets/{id}/preview` — authenticated library preview

The library's own visual preview, not part of the public delivery contract.

- Resolves through `image_of` exactly like `/img`.
- Renders the built-in 400px WebP pipeline through imgproxy.
- Cached in the conversion store under `_library-preview`.
- `Cache-Control: private, max-age=3600`.

`_library-preview` is not a valid preset name, so it cannot be requested through
`/img` or `/convert`; it is only reachable through this authenticated alias.

### `POST /api/assets/{id}/preview` — regenerate visual asset

Re-renders the derived visual asset for audio/video/pdf. For images it refreshes
metadata/updated_at; there is no derived visual asset.

### `GET /convert/{assetId}/{spec}` — conversion cache route

Existing public conversion route used by Caddy on the `assets.` subdomain.
Supports presets and signed specs. The old `{width}x{height}.{ext}` size syntax
has been removed in favor of presets; legacy size cache entries are no longer
generated or served. It does **not** resolve `image_of`, so non-image asset IDs
fail here; use `/img` for those.

Caddy serves cached conversions from the shared conversion volume and rewrites
cache misses to `/convert`.

### `GET /internal/source/{id}` — imgproxy source

Internal-only source route fetched by imgproxy. Caddy blocks `/internal/*` on
the public site.

## Caching

Conversions are stored at:

```text
{sourceAssetID}/{cacheName}.{format}
```

The source is the resolved image asset: the original image, or the derived
visual asset for audio/video/pdf.

- `/img/{id}/{preset}.{ext}` uses the preset name as `cacheName`.
- `/img/{id}/~{signed}.{ext}` uses the signed token as `cacheName`.
- `/api/assets/{id}/preview` uses `_library-preview`.
- `/convert/{id}/{spec}` uses the same cache entries as `/img`, but only for
  image asset IDs.

The `assets.` subdomain serves cached files directly from Caddy with
`Cache-Control: public, max-age=86400`. `/img` is served by godam, which reads
and writes the same conversion store.

There is **no cache busting yet**:

- Replacing an asset or editing a preset can leave stale conversions until the
  24h TTL expires or the cache is purged.
- `POST /api/conversions/purge` clears generated conversions.
- The `?v=updated_at` query on the library preview URL only busts the browser
  cache; the server conversion cache ignores it.

## Where files live

### Derived visual assets

Visual assets are normal assets stored through the local driver:

- Default host data dir:
  `~/Library/Application Support/godam/storage/uploads/{visualAssetID}.jpg|png`
- Docker:
  `/data/godam/storage/uploads/...` inside the `godam-data` volume
- `make dev`: storage uses the host data dir above; conversions use
  `./.dev/conversions`

Their rows are in the `assets` table with `role='visual'`:

```bash
sqlite3 "$HOME/Library/Application Support/godam/godam.db" \
  "select id, image_of, filename, mime_type, storage_key from assets where role='visual';"
```

For Docker, copy the DB out first if `sqlite3` is not installed in the image:

```bash
docker compose cp godam:/data/godam/godam.db ./godam.db
```

The filename is `{original-name}-visual.jpg|png`; the storage key is the
visual asset's own ID (`uploads/{visualAssetID}.jpg|png`).

### Conversions and the library 400px preview

The library preview and all imgproxy conversions are **cached**, not stored as
assets:

- Default host:
  `~/Library/Application Support/godam/storage/conversions/{sourceAssetID}/_library-preview.webp`
- `make dev`: `.dev/conversions/{sourceAssetID}/_library-preview.webp`
- Docker: `/conversions/{sourceAssetID}/_library-preview.webp`
- Caddy sees the same files at `/srv/conversions/...`

`{sourceAssetID}` is the original image ID, or the derived visual asset ID for
audio/video/pdf. Public `/img` preset and signed conversions use the same cache
directory with `{preset}.{format}` or the signed token as the file name.

### Legacy previews

`storage/previews/{assetID}.jpg|png` are from the old stored-preview pipeline.
They are no longer read or written and can be deleted.

## Security and visibility — current state

- `/img` and `/src` are unauthenticated.
- `/convert` is also unauthenticated (pre-existing).
- Asset UUIDs are the only thing protecting public routes; they are not an
  access-control boundary.
- There is no per-asset or per-collection visibility/publish flag.
- There are no signed or expiring original URLs.
- `/src` publicly exposes audio/video/pdf originals. This is the highest-risk
  part of the current contract.
- Visual assets and the library preview are re-encoded derivatives and are not
  the originals, but they still reveal content (including PDF pages and video
  frames).
- `/api/*` routes remain session-protected, except the explicitly public
  delivery and auth routes.
- Caddy blocks `/internal/*` from the public site.

These are intentional gaps for now. The delivery routes are the foundation;
visibility and signing should be layered on before exposing private libraries.

## Known limitations / follow-ups

- Add cache busting keyed by asset `updated_at` and preset `modified_at`.
- Add asset/collection visibility (`private`/`public`) and enforce it in `/img`
  and `/src`; default private.
- Add signed original URLs (expiry, audience) or a CDN token scheme.
- Extend `/api/imgproxy/sign` to non-image parent assets via `image_of`.
- Consider CDN/object storage for `/src`; serving large media through godam
  does not scale well.
- Range support depends on the storage driver implementing `io.ReadSeeker`;
  verify SFTP/object-storage behavior.
- imgproxy community cannot extract audio/video/pdf; PDF/MP4 need Pro, audio
  always needs ffmpeg.
- `/img` currently requires an explicit preset or signed spec. There is no
  public default image route.
- `Caddy` only accelerates `/convert` cache paths; `/img` is served by godam
  even though it shares the conversion cache.
