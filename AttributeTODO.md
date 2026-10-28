# Attribute & Microservice API TODO

Goal: make attributes the host-owned organization model, remove DAM-owned
taxonomy (collections, server-stored smart collections), and close the
remaining gaps so godam exposes a complete DAM microservice API while keeping
the embedded picker/library UI.

The rule: godam stores values, indexes them, filters them, and enforces
`active` / `locked` / `hidden`. The host owns keys, values, vocabulary, labels,
hierarchy, definitions, and views. Attribute definitions are optional config;
per-user presentation state stays client-side or with the host.

## Implemented

- [x] Removed tags and tag groups (tables, services, API routes, filters, UI).
      Assets use attributes for organization and tag-like rendering.
- [x] Flat, multi-valued `asset_attributes` storage and indexes
      (`store/migrations/00014_asset_attributes.sql`).
- [x] Attribute filters: AND across keys, OR within a key, plus absence
      (`Unassigned`) filtering.
- [x] Actor-scoped facets and catalog, including metadata-only keys/values so
      new vocabulary is manageable before first use.
- [x] Token enforcement for `active` / `locked` / `hidden`.
- [x] Per-asset attribute writes (`PUT /api/assets/{id}/attributes`) and bulk
      add/replace (`POST /api/assets/attributes`), with visual-asset sync and
      `updated_at` updates.
- [x] Metadata API/UI for keys and values:
      labels, descriptions, colors, sort (count/name asc-desc, custom, open),
      render mode (select, checkbox, tags, tree, open), single/multi,
      searchable, per-key value search, unassigned label, parent key,
      values lock, upload stamping toggles.
- [x] Key/value creation and deletion from the attributes dialog; inline
      multi-value creation with `,` / `;`.
- [x] Live drag-and-drop ordering for keys and values; custom value order is
      respected by sidebar renderers and trees.
- [x] Renderers: select, checkbox/radio, tag pills, nested tree with conditional
      counts and collection-style guide lines.
- [x] Key/value descriptions rendered via PrimeVue tooltips; colors inherit from
      the key unless overridden; per-key value search bars.
- [x] Inspector shows attribute labels, descriptions, colors, key order, and
      per-asset value selection (`attributes:select`).
- [x] Attribute full-text search: FTS index, per-key searchable toggle, short
      term fallback, actor scoping.
- [x] Vocabulary/cardinality enforcement:
      per-key values lock, single/multi validation, allowed existing values,
      `attributes:create_keys` / `attributes:create_values`,
      `settings:attributes`, and upload stamping with separate key/value
      toggles (default on).
- [x] Upload stamping validation for JWT `upload_values`; global upload
      settings only affect uploads, not the attributes API.
- [x] Bulk attributes via assets context menu and command palette
      (add / replace / clear).
- [x] Unassigned values can be labeled, described, colored, and reordered.
- [x] Inspector is visible by default in both sidebar modes and collapsible.
- [x] No global value-search setting; value search is per attribute.

## Remaining gaps

### 1. Data formats and query depth

- [ ] Typed formats: number, date/datetime, boolean, color, URL, asset
      reference, duration.
- [ ] Typed range filters and typed sorting.
- [ ] `all_of` within a key, in addition to any-of.
- [ ] `none_of` / negative value filters beyond the unassigned sentinel.
- [ ] Prefix/descendant operators for path-like values, if intra-key hierarchy
      is needed.
- [ ] Document and enforce value normalization/case/unicode policy.
- [ ] Stable value identity, value merge, and rename semantics independent of
      display labels.
- [ ] Per-value searchability (currently per-key only).
- [ ] Search ranking/weights and key-name search.
- [ ] Attribute-based asset sort.
- [ ] Limits: keys per asset, values per key, key/value lengths, bulk body size.

### 2. Host definitions, tenancy, and operations

- [ ] Host-supplied definitions via JWT / `postMessage` (labels, vocabulary,
      order, colors, render descriptors) with precedence over stored config.
- [ ] Tenant/host scoping for attribute metadata; current labels, colors,
      vocabulary, locks, and search settings are instance-global.
- [ ] Definition import/export for provisioning, backup, and migration.
- [ ] Definition versioning/audit and change validation.
- [ ] Direct/trusted upload stamping (currently JWT `upload_values` only).
- [ ] Reserved key namespace for migrated taxonomy and host-owned keys.
- [ ] Explicit vocabulary mode field if `values_locked` + capabilities is not
      sufficient for host use cases.
- [ ] Host-level shared preferences (starring/pins) if required across devices.

### 3. Renderers, tree, and sidebar UX

- [ ] Additional renderers: table, color swatch, numeric/date range, toggle,
      breadcrumb/path.
- [ ] Tree selection semantics: selecting a child should optionally apply its
      ancestor path.
- [ ] Group/label multiple child attributes under one parent; current nested
      values from different child keys are mixed.
- [ ] Tree counts conditioned on other active sidebar filters (current tree
      counts only account for enforced scopes and parent path).
- [ ] Lazy loading/performance for large attribute trees.
- [ ] Drag assets from the grid onto attribute keys/values in the sidebar
      (collections support this today).
- [ ] Quicklinks/starring/pinning for attribute keys/values.
- [ ] Shared saved views (server-stored smart collections have no equivalent yet).
- [ ] Command palette filter/navigation by attribute values (currently bulk
      assignment only).
- [ ] Client-side preference namespacing by instance/token identity.

### 4. Tag-like attribute UX

- [ ] Free-text chips input directly on assets in the picker/inspector;
      attribute rendering is filter-side and per-asset selection uses a select
      field.
- [ ] Autocomplete from facets while typing.
- [ ] Value vocabulary/normalization convention and limits.

### 5. Remove DAM-owned taxonomy and integrate replacements

- [x] Remove tags, tag groups, joins, services, routes, UI, JWT filter grants,
      and `Asset.Tags`.
- [ ] Remove collections, joins, services, routes, UI, JWT filter grants, and
      `Asset.Collections`.
- [ ] Move smart collections to client-side saved attribute views.
- [ ] Update quicklinks to pin attribute keys/values/views.
- [ ] Replace preset collection scoping (`domain/preset.go:11`) with attribute
      scoping or library-wide presets.
- [ ] Replace taxonomy stats with attribute aggregates or remove them from
      `/api/stats`.
- [x] Remove `tag` and `untagged` from `FilterNames` and embed params.
- [ ] Include attributes in asset export/import manifests (currently manifests
      include collections).
- [ ] Decide old-snapshot restore behavior and provide a deprecation window.
- [ ] Update `CONSUMING.md`, sample consuming app, and integration tests.

### 6. Tests and hardening

- [x] Service tests for metadata, vocabulary locks, single/multi, upload
      stamping, nested trees, ordering, descriptions, colors, search.
- [ ] API tests for every metadata/config route and capability failure.
- [ ] Frontend tests for dialog editing, drag/drop ordering, tree filtering,
      tooltips, colors, per-key search, and inspector ordering.
- [ ] Migration tests for taxonomy export/drop/restore.
- [ ] Multi-tenant isolation tests for global metadata.
- [ ] Limits and fuzz/abuse tests.

## Decided

- Admin metadata capability: `settings:attributes`.
- Per-asset selection: `attributes:select`.
- Key/value creation capabilities: `attributes:create_keys`,
  `attributes:create_values`.
- Upload stamping is governed by separate instance key/value toggles, default
  on, and only applies to uploads.
- Cardinality is enforced on uploads and writes, not just rendering.
- Per-key `values_locked` blocks new values on every write path.
- Value search is per key.
- Inspector is visible by default and collapsible.
- Custom key/value order is persisted and displayed in sidebar/inspector.

## Open decisions

- Which typed formats ship and whether typed storage is needed.
- Whether intra-key hierarchy uses paths, stable IDs, or another view config.
- Host-definition precedence and metadata tenancy model.
- Whether unassigned can be assigned by APIs or remains filter-only.
- Stable value identity/merge strategy.
- Shared saved views and quicklink ownership.
- Preset scoping after collections are removed.
- Default renderer and whether per-user choice can override a host-locked render.
- Direct/trusted upload stamping contract.

## Suggested sequence

1. Typed formats and any required query operators.
2. Host-supplied definitions, precedence, and tenancy scoping.
3. Tree/UX refinements plus drag-assignment and quicklinks/saved views.
4. Preset, stats, and export integration.
5. Migrate collections into attributes, then remove taxonomy.
6. Limits, audit/versioning, frontend tests, and deprecation cleanup.

## References

- `store/attribute.go`, `store/attribute_metadata.go`
- `store/migrations/00014_asset_attributes.sql` through
  `00023_attribute_value_search_per_key.sql`
- `domain/attribute.go`, `domain/scope.go`, `domain/preset.go`
- `service/asset_service.go`, `service/instance_service.go`,
  `service/stats_service.go`
- `service/preview_asset.go`
- `api/assets.go`, `api/instance.go`, `api/server.go`
- `frontend/src/components/AttributesDialog.vue`
- `frontend/src/components/AppSidebarAttributes.vue`
- `frontend/src/components/AttributeTree.vue`
- `frontend/src/components/Inspector.vue`
- `frontend/src/lib/attributeRender.ts`, `attributeSort.ts`, `attributeTree.ts`,
  `attributeCatalog.ts`, `attributesDialog.ts`
- `CONSUMING.md`
