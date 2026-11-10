// Shared types mirroring the Go server's JSON API.

export type PathType = 'Dir' | 'SymlinkDir' | 'File' | 'SymlinkFile'

export interface PathItem {
  path_type: PathType
  name: string
  mtime: number
  size: number
}

export interface IndexData {
  href: string
  kind: 'Index' | 'Edit' | 'View'
  uri_prefix: string
  allow_upload: boolean
  allow_delete: boolean
  allow_search: boolean
  allow_archive: boolean
  dir_exists: boolean
  auth: boolean
  user: string | null
  paths: PathItem[]
}

export interface TreeNode {
  path: string
  name: string
  expanded: boolean
  loaded: boolean
  collapsed?: boolean
  /** 正在请求子目录（用于折叠箭头位置显示 spinner）。 */
  loading?: boolean
  /** 子目录加载失败的原始错误信息；存在时节点显示警示图标并可重试。 */
  error?: string
  /** 该目录自身的写权限（来自加载它的 listDir 响应），决定右键菜单条目。 */
  canUpload?: boolean
  canDelete?: boolean
  children: TreeNode[]
}

export interface UploadTask {
  id: string
  name: string
  progress: number
  status: 'uploading' | 'done' | 'error'
  error?: string
}

export type SortKey = 'name' | 'mtime' | 'size'
export type SortOrder = 'asc' | 'desc'

export interface ListParams {
  sort: SortKey
  order: SortOrder
  q: string
  /** 仅搜索模式有效：只返回匹配的目录（目录树过滤用）。 */
  dirsOnly?: boolean
}
