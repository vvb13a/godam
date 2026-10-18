export interface User {
  id: string
  name: string
  email: string
  created_at: string
  updated_at: string
}

export interface Instance {
  id: string
  name: string
  color: string
  cover_asset_id?: string
  watermark_asset_id?: string
  allowed_mime_types?: string[]
  max_upload_bytes?: number
  created_at: string
  updated_at: string
}

export type Library = Instance

export interface Tag {
  id: string
  name: string
  description: string
  group_id?: string
  starred: boolean
  asset_count: number
  created_at: string
  updated_at: string
}

export interface TagGroup {
  id: string
  name: string
  description: string
  color: string
  tag_count: number
  created_at: string
  updated_at: string
}

export interface Collection {
  id: string
  parent_id?: string
  name: string
  description: string
  color: string
  sort: number
  created_at: string
  updated_at: string
  asset_count: number
}

export interface SmartCollectionFilter {
  collection?: string
  tag?: string
  type?: string
  sort?: string
}

export interface SmartCollection {
  id: string
  name: string
  description: string
  color: string
  sort: number
  filter: SmartCollectionFilter
  asset_count: number
  created_at: string
  updated_at: string
}

export interface AssetColor {
  hex: string
  bucket: string
  percentage: number
}

export interface AssetMetadata {
  width?: number
  height?: number
  duration_sec?: number
  page_count?: number
  raw_json?: string
}

export interface Asset {
  id: string
  uploaded_by?: string
  uploaded_by_name?: string
  filename: string
  title?: string
  note?: string
  storage_driver: string
  storage_key: string
  mime_type: string
  byte_size: number
  metadata: AssetMetadata
  colors?: AssetColor[]
  tags?: Tag[]
  collections?: Collection[]
  created_at: string
  updated_at: string
}

export interface TypeCount {
  mime_type: string
  count: number
  bytes: number
}

export interface Stats {
  assets: number
  collections: number
  tags: number
  uncategorized: number
  untagged: number
  storage_bytes: number
  types: TypeCount[]
}

export interface LibraryBackup {
  enabled: boolean
  driver: string
  host: string
  port: number
  user: string
  base_path: string
  key_path: string
  host_key_fingerprint: string
  has_password: boolean
  last_run_at?: string
  last_error: string
}

export interface LibraryBackupStatus {
  enabled: boolean
  total: number
  synced: number
  pending: number
  failed: number
  last_error: string
  last_run_at?: string
}
