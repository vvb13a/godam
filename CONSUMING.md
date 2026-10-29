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
    "filters": {
      "type": { "values": ["image"], "mode": "active" },
      "mime": { "values": ["image/png", "image/webp"], "mode": "active" },
      "search": { "values": ["brand"], "mode": "active" },
      "sort": { "values": ["date-desc"], "mode": "active" }
    },
    "attributes": {
      "tenant": {
        "values": ["acme"],
        "mode": "hidden",
        "upload_values": ["acme"]
      },
      "entity": {
        "values": ["news", "press"],
        "mode": "active"
      }
    },
    "upload_settings": [
      {
        "format": "image",
        "mime_types": ["image/jpeg", "image/png", "image/webp"],
        "max_bytes": 10485760
      },
      { "format": "video", "max_bytes": 104857600 }
    ],
    "capabilities": ["assets:read", "assets:upload"]
  }
}
```

- `sub` identifies the token actor (for example, the upload-session owner).
- `username` becomes `uploaded_by_name` for uploads made with this token.

### Filters

`scope.filters` controls which listing filters are available, plus their default
and enforcement behaviour. Supported keys are `type`, `mime`, `search`, `sort`,
`color`, `size`, `duration`, `pages`, `dimensions`, `added`, and `modified`.

Each entry has an optional `values` (string or array) and a `mode`:

- `allowed` shows the filter with no default. The client may use or clear it.
- `active` (default) shows the filter and applies `values` as the embed UI's
  initial state. The API does not enforce active values, so the client may
  change or clear them. Values are not an allow-list.
- `locked` applies `values` server-side. The editable control is hidden.
- `hidden` applies `values` server-side without sending the values to the
  client.
- `denied` hides the filter and rejects requests that supply it.

A filter without a grant is denied. Use the wildcard to allow every filter and
then override only the ones that need a different mode:

```json
"filters": {
  "*": { "mode": "allowed" },
  "type": { "values": ["image"], "mode": "locked" },
  "search": { "mode": "denied" },
  "sort": { "mode": "active", "values": "name-asc" }
}
```

`"filters": "*"` is shorthand for allowing every filter with no defaults.
`allowed` and `denied` do not accept `values`; use `active`, `locked`, or
`hidden` when supplying values.

### Host-owned attributes

Assets may carry host-defined attributes such as `tenant`, `entity`, or `kind`.
Add them under `scope.attributes`:

```json
"attributes": {
  "tenant": {
    "values": ["acme"],
    "mode": "hidden",
    "upload_values": ["acme"]
  },
  "entity": {
    "values": ["news", "press"],
    "mode": "active"
  },
  "kind": {
    "values": ["logo", "icon"],
    "mode": "locked",
    "upload_values": "*"
  }
}
```

- `values` follow the same `active`, `locked`, and `hidden` modes as filters.
  Active values are only initial UI filters and are not enforced by the API.
  None of the modes restrict which options the UI may show.
- `upload_values` are stamped onto uploads made with the token, regardless of
  mode. When omitted, nothing is stamped. `"*"` expands to the attribute's
  declared `values`.
- `locked` and `hidden` attributes are enforced for listing, counts, and
  asset-by-ID access; `active` attributes are only filters.

Attribute filters are sent as a JSON object in the `attributes` query parameter,
for example `attributes={"entity":["news"],"kind":["logo"]}`. With an explicit
scope, keys not present in the token are ignored; locked and hidden values cannot
be overridden.

A full-library token can allow every attribute in use with the wildcard:

```json
"attributes": "*"
```

or the equivalent object form:

```json
"attributes": { "*": { "mode": "active" } }
```

The wildcard only supports `active` mode and cannot define values or
`upload_values`. Use it only when the token is allowed to browse the whole
library; picker tokens with tenant isolation should keep explicit keys.

`GET /api/attributes` lists the attribute keys and values available for
filtering. The result is already scoped: tokens without attribute access get an
empty list, explicit scopes return only their active keys, wildcard tokens see
all keys, and locked/hidden attributes are excluded and enforced. Each facet
includes a display `label`, a `sort` setting (`count:asc`, `count:desc`,
`abc:asc`, `abc:desc`, `custom`, or `open`), and a `render` setting (`select`,
`checkbox`, `tags`, `tree`, or `open`). `open` lets each user choose their own order or renderer;
the other values fix it for everyone.

Display labels only affect presentation; filters and uploads continue to use
the raw key/value strings. Metadata management routes require
`settings:attributes`:

```text
POST   /api/attributes/metadata/values
DELETE /api/attributes/metadata/value
DELETE /api/attributes/metadata/key
PUT    /api/attributes/metadata/label
PUT    /api/attributes/metadata/sort
PUT    /api/attributes/metadata/render
PUT    /api/attributes/metadata/search
PUT    /api/attributes/metadata/multi
PUT    /api/attributes/metadata/unassigned
PUT    /api/attributes/metadata/unassigned-label
PUT    /api/attributes/metadata/parent
PUT    /api/attributes/metadata/value-search
PUT    /api/attributes/metadata/values-lock
PUT    /api/attributes/metadata/description
PUT    /api/attributes/metadata/color
PUT    /api/attributes/metadata/value-order
PUT    /api/attributes/metadata/key-order
POST   /api/attributes
```

Facets and values also carry optional `description` and `color` fields, plus a
`position` used by custom ordering. Colors are hex strings without the leading
`#`; values without a color inherit the key color. Reorder values with
`PUT /api/attributes/metadata/value-order` and `{ "key": "model", "values": [...] }`,
and keys with `PUT /api/attributes/metadata/key-order` and `{ "keys": [...] }`.
Set descriptions or colors with the matching metadata routes.

Create a new attribute key with `POST /api/attributes` and
`{ "key": "model", "label": "Model" }`. Creating keys requires
`settings:attributes` or `attributes:create_keys`. Creating values requires
`settings:attributes` or `attributes:create_values`, and the key must be
unlocked. `PUT /api/attributes/metadata/values-lock` with
`{ "key": "model", "locked": true }` blocks new values on every path until it is
unlocked.

Upload stamping has two independent instance settings, enabled by default:
new keys and new values are allowed through JWT upload attributes but not
through the attribute APIs. Update them with
`PUT /api/instance/attribute-stamping` and `{ "keys": true, "values": true }`
(`settings:attributes`). This keeps a stolen upload token from creating
vocabulary through the attributes API.

`PUT /api/attributes/metadata/value-search` with
`{ "key": "model", "enabled": true }` adds a value search box to that
attribute's sidebar renderer. It is presentation only and stored per key.

`GET /api/attributes/tree` (`assets:read`) returns nested attribute trees using
the `parent_key` metadata. Set a parent with
`{ "key": "model", "parent": "site" }`; an empty `parent` makes the attribute a
root.

`GET /api/attributes/catalog` (`assets:read`) includes metadata-only values
that no asset uses yet. The dialog uses it to define vocabulary, and the asset
inspector uses it for per-asset selection. Create one or more values for an
existing key with:

```json
{ "key": "model", "values": ["news", "sports", "weather"] }
```

Per-asset selection requires `attributes:select` and replaces the values of one
key without touching other keys:

```text
PUT /api/assets/{id}/attributes
```

```json
{ "key": "model", "values": ["news", "sports"] }
```

An empty `values` array removes the key from that asset. Locked and hidden token
attributes cannot be changed. Apply the same change to many assets with:

```text
POST /api/assets/attributes
```

```json
{ "asset_ids": ["ASSET_ID"], "key": "model", "values": ["news"], "mode": "add" }
```

`mode` is `add` (merge values) or `replace` (set the exact value set; an empty
array clears the key).

Rename a key or value with:

```json
{ "key": "model", "value": "news", "label": "Newsroom" }
```

Omit `value` to rename the key itself. Set `label` to an empty string to clear
the override. Change ordering or rendering with:

```json
{ "key": "model", "sort": "open" }
{ "key": "model", "render": "tags" }
{ "key": "model", "searchable": true }
{ "key": "model", "multi": false }
{ "key": "model", "include_unassigned": true }
{ "key": "model", "label": "No model" }
{ "key": "model", "parent": "site" }
```

`searchable` includes the key's values in asset text search. `multi` controls
whether a renderer adds (`true`) or replaces (`false`) its selection.
`include_unassigned` adds an `Unassigned` facet value for assets with no value
for that key. Its display label is set with
`PUT /api/attributes/metadata/unassigned-label` and `{ "key": "model", "label": "No model" }`.
Delete a single value
with `{ "key": "model", "value": "news" }` or an entire key with
`{ "key": "model" }`; both remove the value from every asset.

### Upload settings

`scope.upload_settings` narrows uploads only; it does not hide assets from the
picker. It accepts one object or an array:

```json
"upload_settings": [
  { "format": "image", "mime_types": ["image/png", "image/webp"], "max_bytes": 5242880 },
  { "format": "pdf" },
  { "format": "*", "max_bytes": 104857600 }
]
```

- `format` is `image`, `audio`, `video`, `pdf`, or `*`/omitted for all formats.
- `mime_types` restricts the accepted types. When omitted, every type of that
  format is accepted.
- `max_bytes` overrides the size limit for that format. When omitted, the
  library's limit applies.
- Settings are always intersected with the library's allow-list and size limits.
  A token can make uploads stricter, never broader.

### Capabilities

Management and configuration routes require explicit capabilities in
`scope.capabilities`:

```json
{
  "scope": {
    "capabilities": [
      "assets:read",
      "assets:upload",
      "assets:update",
      "assets:delete",
      "settings:presets",
      "settings:upload"
    ]
  }
}
```

| Capability | Purpose |
| --- | --- |
| `assets:read` | List, view, preview, download assets |
| `assets:upload` | Upload files and use upload sessions |
| `assets:update` | Edit assets, attributes, previews |
| `assets:delete` | Delete assets |
| `settings:storage` | Read/change storage and backup settings |
| `settings:presets` | Read/change presets and preset previews |
| `settings:upload` | Change library upload settings |
| `conversions:warm` | Warm conversions |
| `conversions:purge` | Purge the conversion cache |
| `library:update` | Rename the library, set cover/watermark |
| `settings:attributes` | Manage attribute labels, ordering, rendering, nesting, search, selection, and values |
| `attributes:select` | Change attribute values per asset, including bulk add/replace |
| `attributes:create_keys` | Create new attribute keys through the attributes API |
| `attributes:create_values` | Create new attribute values through the attributes API |

`assets:*` grants all `assets:*` capabilities, and `*` grants everything.
Tokens without a `capabilities` claim default to `assets:read` only. Add
`assets:upload` to allow uploads. Direct network-trusted access receives all
capabilities.

Filter `values`, `mime_types`, and `upload_values` accept a string or an array.

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
| `GODAM_DOMAIN` | Public application hostname, e.g. `dam.example.com`. |
| `GODAM_DELIVERY_DOMAIN` | Optional public origin for `/img` and `/src`, e.g. `assets.example.com` or `cdn.example.com`. Defaults to `GODAM_DOMAIN`. |
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

`/img` and `/src` are currently public and protected only by the
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
    "filters":         {...},
    "attributes":      {...},
    "upload_settings": [...],
    "capabilities":    ["assets:read", "assets:upload"]
  }
}

if issuer configured:    claims["iss"] = issuer
if audience configured:  claims["aud"] = audience

token = jwt_sign_hs256(claims, jwt_secret)
```

A scoped example for a "brand images only" picker:

```json
{
  "sub": "user-42",
  "username": "Ada Lovelace",
  "exp": 1740000900,
  "scope": {
    "filters": {
      "type": { "values": ["image"], "mode": "locked" },
      "sort": { "values": ["date-desc"], "mode": "locked" }
    },
    "upload_settings": [
      { "format": "image", "mime_types": ["image/*"] }
    ],
    "capabilities": ["assets:read"]
  }
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
IDs are stable strings; godam owns the files, metadata, and attributes.

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
- Locked and hidden attributes are enforced for API listing, counts, and
  asset-by-ID operations. `/img` and `/src` are still public
  UUID routes, so private delivery requires signed or authenticated URLs before
  relying on hidden attributes for tenant isolation.
- Management and configuration routes require explicit capabilities. Grant only
  what the consuming application needs and use `*` sparingly.
- Filter scope is enforced in both the API and the picker UI. A manipulated URL
  cannot escape locked defaults.
