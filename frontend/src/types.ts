export interface Library {
  id: string
  name: string
  color: string
  sort: number
  created_at: string
  updated_at: string
}

export interface Tag {
  id: string
  library_id: string
  name: string
  group_id?: string
  starred: boolean
  asset_count: number
  created_at: string
}

export interface TagGroup {
  id: string
  library_id: string
  name: string
  description: string
  color: string
  tag_count: number
  created_at: string
  updated_at: string
}

export interface Collection {
  id: string
  library_id: string
  name: string
  description?: string
  icon?: string
  created_at: string
  updated_at: string
  asset_count: number
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
  library_id: string
  filename: string
  storage_driver: string
  storage_key: string
  mime_type: string
  byte_size: number
  metadata: AssetMetadata
  tags?: Tag[]
  collections?: Collection[]
  created_at: string
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
