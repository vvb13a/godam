import type { Component } from 'vue'
import {
  Bolt,
  Book,
  Bookmark,
  Box,
  Briefcase,
  Camera,
  Cloud,
  Database,
  File,
  FilePdf,
  Flag,
  Folder,
  Gift,
  Globe,
  Headphones,
  Heart,
  Image,
  Images,
  Map,
  Palette,
  Sitemap,
  Star,
  Tag,
  Tags,
  Ticket,
  Video,
  Warehouse,
} from '@primeicons/vue'

export interface CollectionIconOption {
  name: string
  label: string
  component: Component
}

// The curated set of PrimeIcons a collection icon can be. Only these are
// imported so the bundle does not pull in the whole icon library.
export const COLLECTION_ICONS: CollectionIconOption[] = [
  { name: 'folder', label: 'Folder', component: Folder },
  { name: 'images', label: 'Images', component: Images },
  { name: 'image', label: 'Image', component: Image },
  { name: 'camera', label: 'Camera', component: Camera },
  { name: 'video', label: 'Video', component: Video },
  { name: 'headphones', label: 'Audio', component: Headphones },
  { name: 'file', label: 'File', component: File },
  { name: 'file-pdf', label: 'PDF', component: FilePdf },
  { name: 'star', label: 'Star', component: Star },
  { name: 'heart', label: 'Heart', component: Heart },
  { name: 'bolt', label: 'Bolt', component: Bolt },
  { name: 'palette', label: 'Palette', component: Palette },
  { name: 'box', label: 'Box', component: Box },
  { name: 'warehouse', label: 'Warehouse', component: Warehouse },
  { name: 'book', label: 'Book', component: Book },
  { name: 'tag', label: 'Tag', component: Tag },
  { name: 'tags', label: 'Tags', component: Tags },
  { name: 'bookmark', label: 'Bookmark', component: Bookmark },
  { name: 'briefcase', label: 'Briefcase', component: Briefcase },
  { name: 'gift', label: 'Gift', component: Gift },
  { name: 'globe', label: 'Globe', component: Globe },
  { name: 'map', label: 'Map', component: Map },
  { name: 'sitemap', label: 'Sitemap', component: Sitemap },
  { name: 'database', label: 'Database', component: Database },
  { name: 'cloud', label: 'Cloud', component: Cloud },
  { name: 'flag', label: 'Flag', component: Flag },
  { name: 'ticket', label: 'Ticket', component: Ticket },
]

export function collectionIcon(name?: string): Component | null {
  if (!name) return null
  return COLLECTION_ICONS.find((option) => option.name === name)?.component ?? null
}
