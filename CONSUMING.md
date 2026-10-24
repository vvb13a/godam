# Consuming godam from another application

This document is the integration contract for any application that wants to
embed the godam asset picker/library and deliver the selected assets. It covers
the iframe API, scoped JWT authentication, environment variables, delivery
URLs, and the wiring a consuming application needs to implement.

## How embedding works

The consuming application renders an iframe that loads godam's `/embed` route:

```text
https://dam.example.com/embed?embed=1&mode=single&token=<jwt>&hostOrigin=https://app.example.com
```

The iframe is the godam single-page app. It authenticates every API request with
the JWT, applies the token scope server-side, and reports selected asset IDs back
to the parent page with `postMessage`.

There are three embed modes:

| `mode` | Behaviour |
| --- | --- |
| `library` (default) | Full library browsing, no selection action bar. |
| `single` | Picker that selects exactly one asset. |
| `multi` | Picker that selects zero or more assets. |

The picker does not upload files into the host page. It returns asset IDs; the
host stores those IDs and renders them with godam's delivery URLs.

### Embed URL parameters

| Parameter | Description |
| --- | --- |
| `embed=1` | Required marker that enables embed mode. |
| `mode` | `library`, `single`, or `multi`. |
| `token` | HS256 JWT. Required for token auth; it may also be sent as `Authorization: Bearer …` on API calls. |
| `hostOrigin` | Origin allowed to receive `postMessage` events, e.g. `https://app.example.com`. Defaults to `*`; set this in production. |
| `selected` | Comma-separated asset IDs to preselect. |
| `collection` / `tag` | Optional host-side picker restriction. Token defaults/locks take precedence. |
| `lock` | Legacy host-side lock for `collection` and/or `tag`, e.g. `collection,tag`. |

### postMessage protocol

Host → picker:

```js
iframe.contentWindow.postMessage({ type: 'godam:ping' }, godamOrigin)
iframe.contentWindow.postMessage(
  { type: 'godam:setSelection', assetIds: ['ASSET_ID'] },
  godamOrigin,
)
```

Picker → host:

```js
// { type: 'godam:ready', assetCount: 42 }
// { type: 'godam:selection', assetIds: ['ASSET_ID'] }
// { type: 'godam:cancel' }
```

Always verify `event.origin === godamOrigin` and, ideally, that
`event.source === iframe.contentWindow`.

## Scoped JWT authentication

JWT auth is enabled by setting `GODAM_JWT_SECRET` on the godam server. The token
is an HS256 JWT signed with the same secret. godam validates the signature,
`exp` (required), `nbf`, `iss`, and `aud`, then enforces the token's scope.

### Claims

```json
{
  "iss": "https://app.example.com",
  "aud": "godam",
  "sub": "user-42",
  "username": "Ada Lovelace",
  "iat": 1740000000,
  "exp": 1740000900,
  "scope": {
    "default_filters": {
      "collection": ["collection-id"],
      "tag": ["tag-id"],
      "type": ["image"],
      "mime": ["image/png", "image/webp"],
      "search": "brand",
      "sort": "date-desc"
    },
    "locked_filters": ["collection", "type", "search", "sort"],
    "allowed_mime_types": ["image/*"],
    "can_upload": true
  }
}
```

- `sub` identifies the token actor (for example, the upload-session owner).
- `username` becomes `uploaded_by_name` for uploads made with this token.
- `scope.default_filters` are applied when the URL/request does not supply that
  filter.
- `scope.locked_filters` forces the corresponding default even if the client
  tries to change it. Supported names: `collection`, `tag`, `type`, `mime`,
  `search`, `sort`.
- `scope.allowed_mime_types` is intersected with the library's own upload
  allow-list and also narrows what the picker lists. Exact types and `type/*`
  wildcards are supported.
- `scope.can_upload` defaults to `true`. Set it to `false` for a read-only
  picker.

`collection` and `tag` accept a string or an array. The special values
`uncategorized` and `untagged` are also understood. `allowed_mime_types` and
`locked_filters` accept a string or an array.

Token authentication works on every API route, including `/api/auth/me`, asset
listing, uploads, previews, and content. Browser elements that cannot send an
`Authorization` header (`<img>`, `<video>`, PDF viewers) can use `?token=<jwt>`
on the authenticated content/preview URLs instead. The embed frontend adds that
parameter automatically.

When a request carries a JWT, godam enforces the token scope. Requests without a
JWT are treated as trusted direct access and must be restricted at the network
or proxy layer.

## Environment variables

### Required for a production JWT embedding deployment

| Variable | Purpose |
| --- | --- |
| `GODAM_DOMAIN` | Public hostname, e.g. `dam.example.com`. godam derives `assets.<domain>` for rendered conversions. |
| `GODAM_JWT_SECRET` | HS256 signing secret. Leave empty to disable JWT auth; requests are then treated as trusted direct access. Use at least 32 random bytes, e.g. `openssl rand -hex 32`. |
| `IMGPROXY_KEY` / `IMGPROXY_SALT` | Image conversion signing pair for Docker Compose. The godam container receives them as `GODAM_IMGPROXY_KEY` / `GODAM_IMGPROXY_SALT`. |

### Optional JWT checks

| Variable | Purpose |
| --- | --- |
| `GODAM_JWT_ISSUER` | When set, the token's `iss` must match. |
| `GODAM_JWT_AUDIENCE` | When set, the token's `aud` must contain this value. |

### Runtime and storage

| Variable | Default | Purpose |
| --- | --- | --- |
| `GODAM_ADDR` | `:8080` | HTTP listen address. |
| `GODAM_LIBRARY_NAME` | `Library` | Display name created on first boot. |
| `GODAM_CONVERSION_DIR` | `<storage>/conversions` | Conversion cache shared with Caddy. |
| `GODAM_CONVERSIONS_DRIVER` | `local` | Conversion cache driver. |
| `GODAM_IMGPROXY_URL` | `http://127.0.0.1:8080` | Internal imgproxy base URL. |
| `GODAM_INTERNAL_URL` | derived from `GODAM_ADDR` | How imgproxy reaches godam's source route. |
| `GODAM_MANAGEMENT_TOKEN` | empty | Enables fleet endpoints `/api/system/info` and `/api/system/metrics`. When empty those endpoints are not exposed. |

For a host-run development process, use the `GODAM_IMGPROXY_KEY` and
`GODAM_IMGPROXY_SALT` names. For Docker Compose, use `IMGPROXY_KEY` and
`IMGPROXY_SALT`; Compose passes them through to the godam container.

## Asset delivery from the consuming application

After the picker returns IDs, the host can render assets directly:

```html
<!-- Public rendered image (preset name + format) -->
<img src="https://dam.example.com/img/ASSET_ID/md.webp" alt="" />

<!-- Responsive variants -->
<img
  src="https://dam.example.com/img/ASSET_ID/md.webp"
  srcset="
    https://dam.example.com/img/ASSET_ID/xs.webp 320w,
    https://dam.example.com/img/ASSET_ID/sm.webp 640w,
    https://dam.example.com/img/ASSET_ID/md.webp 1280w,
    https://dam.example.com/img/ASSET_ID/lg.webp 1920w
  "
  sizes="(max-width: 640px) 100vw, 50vw"
  alt=""
/>

<!-- Public original for audio/video/PDF only -->
<a href="https://dam.example.com/src/ASSET_ID">Open PDF</a>
```

Authenticated previews and originals use:

```text
GET /api/assets/{id}/preview?token=<jwt>
GET /api/assets/{id}/content?token=<jwt>
GET /api/assets/zip?ids=ID1,ID2&token=<jwt>
```

`/img`, `/src`, and `/convert` are currently public and protected only by the
asset UUID. Do not embed a private library until delivery-side authorization is
available.

## Wiring checklist for the consuming application

### 1. Configure the godam connection

Add application configuration for:

```text
Godam base URL:        https://dam.example.com
JWT signing secret:    <same value as GODAM_JWT_SECRET on godam>
JWT issuer:            https://app.example.com        (optional)
JWT audience:          godam                          (optional)
Token lifetime:        5-15 minutes recommended
```

Never expose the signing secret to the browser. Token minting must happen on the
server.

### 2. Mint a scoped token server-side

Build the claims from the authenticated user and the screen the picker is being
opened for. Use any HS256 JWT library in the application's ecosystem; the
algorithm and claim names are the contract, not the implementation.

Language-neutral shape:

```text
now = current_unix_timestamp()

claims = {
  "sub":      string(user.id),
  "username": user.display_name,
  "iat":      now,
  "exp":      now + 900,
  "scope": {
    "default_filters":    {...},
    "locked_filters":     [...],
    "allowed_mime_types": [...],
    "can_upload":         true
  }
}

if issuer configured:    claims["iss"] = issuer
if audience configured:  claims["aud"] = audience

token = jwt_sign_hs256(claims, jwt_secret)
```

A scoped example for a "brand images only" picker:

```json
{
  "default_filters": {
    "collection": ["brand-collection-id"],
    "type": ["image"],
    "sort": "date-desc"
  },
  "locked_filters": ["collection", "type", "sort"],
  "allowed_mime_types": ["image/*"],
  "can_upload": false
}
```

Mint a fresh token whenever the picker is opened. If the picker stays open longer
than the token lifetime, reload the iframe with a new token.

### 3. Render the embed UI

The host needs:

- A button or form field that opens the picker.
- A modal/container containing the iframe.
- A hidden field or component state that stores the selected asset IDs.
- A `message` listener that verifies the godam origin and handles selection.

Minimal host-side JavaScript:

```js
const godamOrigin = new URL(godamUrl).origin
const iframe = document.createElement('iframe')

const embedUrl = new URL('/embed', godamUrl)
embedUrl.search = new URLSearchParams({
  embed: '1',
  mode: 'multi',
  token,
  hostOrigin: window.location.origin,
  selected: currentAssetIds.join(','),
}).toString()
iframe.src = embedUrl.toString()
iframe.title = 'godam asset picker'

function onMessage(event) {
  if (event.origin !== godamOrigin) return
  if (event.source !== iframe.contentWindow) return

  const data = event.data || {}
  if (data.type === 'godam:ready') {
    // The iframe is ready. Optionally ping it to re-announce readiness.
    iframe.contentWindow.postMessage({ type: 'godam:ping' }, godamOrigin)
  }
  if (data.type === 'godam:selection') {
    currentAssetIds = data.assetIds || []
    iframe.contentWindow.postMessage(
      { type: 'godam:setSelection', assetIds: currentAssetIds },
      godamOrigin,
    )
    closePicker()
  }
  if (data.type === 'godam:cancel') {
    closePicker()
  }
}

window.addEventListener('message', onMessage)
```

On close, remove the `message` listener and the iframe to avoid leaks and stale
tokens.

### 4. Persist and render selections

Store the returned asset IDs in the consuming application's own data model. Asset
IDs are stable strings; godam owns the files, metadata, tags, and collections.

Render thumbnails and originals with the public delivery URLs, or use the
authenticated preview/content endpoints when the token is still valid. Do not
copy godam URLs into storage as permanent signed links unless you control token
renewal.

### 5. Verify the integration

```bash
# Replace with a token minted by the consuming application.
curl -H "Authorization: Bearer <token>" https://dam.example.com/api/auth/me
```

The response includes the resolved user plus the server-enforced `scope`. If the
call returns `401`, check the secret, algorithm (`HS256`), `exp`, and the
optional issuer/audience values.

Then open the embed URL directly in a browser and confirm:

- The library loads with the expected default filters.
- Locked filters cannot be changed.
- Only allowed file types appear.
- `godam:selection` posts the expected asset IDs to the host.

## Security and limitations

- A JWT is a bearer credential. Treat it like a password, avoid logging URLs
  that contain it, and use short expiries.
- Always set `hostOrigin` in production; `*` only skips the postMessage target
  restriction, not API authorization.
- Browser elements cannot send headers, so the embed frontend appends `?token=`
  to authenticated preview/content URLs. Keep those URLs out of logs and referer
  headers where possible.
- If the host application uses a Content-Security-Policy, allow the godam origin
  in `frame-src`.
- The token scope covers listing/upload defaults, locked filters, allowed MIME
  types, and `can_upload`. It does not yet implement per-asset visibility:
  `/img` and `/src` remain public UUID routes.
- `can_upload` is the only explicit capability flag in this iteration. Library
  management routes remain available to any valid token; split tokens by
  audience/issuer if you need a harder separation today.
- Filter scope is enforced in both the API and the picker UI. A manipulated URL
  cannot escape locked defaults.
