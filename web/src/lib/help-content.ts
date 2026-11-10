// Static content of the "Guide" tab in the help dialog, in both languages.
// Items may contain simple inline HTML (e.g. <code>...</code>) and are
// rendered with v-html — the content here is static and trusted.
import type { Locale } from '../locales'

export interface HelpSection {
  title: string
  items: string[]
}

const zh: HelpSection[] = [
  {
    title: '浏览与导航',
    items: [
      '左侧目录树点击文件夹进入；右侧列表点击文件夹进入、点击文件预览。',
      '顶部面包屑可快速回到任意上级目录；地址用 <code>#/路径</code> 记录，可收藏/刷新不丢位置。',
      '侧栏与内容之间的分隔条可左右拖动调整宽度。',
    ],
  },
  {
    title: '搜索与排序',
    items: [
      '搜索框输入后回车，在当前目录递归搜索文件。',
      '按名称 / 修改时间 / 大小排序，再次点击切换升降序。',
    ],
  },
  {
    title: '文件操作',
    items: [
      '上传：点"上传"按钮选择文件，显示实时进度。',
      '新建文件夹 / 新建文件；重命名、删除（均不可恢复，删除有二次确认）。',
      '打包下载：把当前目录或某个子目录打包为 zip 下载。',
      '复制完整路径 / 复制 curl 下载命令 / curl 上传命令，便于命令行操作。',
    ],
  },
  {
    title: 'curl 命令示例',
    items: [
      '上传文件：<code>curl -T local/path/to/file.ext http://host:port/path/to/file.ext</code>',
      '下载文件：<code>curl -O http://host:port/path/to/file.ext</code>',
      '查看列表：<code>curl "http://host:port/?json="</code>',
      '搜索文件：<code>curl "http://host:port/?json=&amp;q=keyword"</code>',
      '重命名/移动：<code>curl -X MOVE -H "Destination: http://host:port/new/path" http://host:port/old/path</code>',
      '删除：<code>curl -X DELETE http://host:port/path/to/file.ext</code>',
      '新建目录：<code>curl -X MKCOL http://host:port/path/to/dir</code>',
      '打包下载目录：加 <code>?zip</code>，如 <code>curl -o dir.zip "http://host:port/path/to/dir?zip"</code>',
      '获取文件 SHA256：加 <code>?hash</code>，如 <code>curl "http://host:port/path/to/file.ext?hash"</code>',
    ],
  },
  {
    title: '预览与编辑',
    items: [
      '支持图片、视频、音频、PDF、Markdown、代码、JSON、文本在线预览。',
      '文本类文件可直接点"编辑"在线修改，<code>Ctrl/Cmd+S</code> 保存；Markdown 编辑时可切换预览。',
    ],
  },
  {
    title: '外观与语言',
    items: [
      '侧栏底部可切换主题：自动 / 亮色 / 暗色。',
      '侧栏底部可切换界面语言：中文 / English。',
      '偏好保存在浏览器本地，刷新后保持不变。',
    ],
  },
]

const en: HelpSection[] = [
  {
    title: 'Browsing & navigation',
    items: [
      'Click a folder in the left tree to enter it; in the right pane, click a folder to enter it or a file to preview it.',
      'The breadcrumb at the top jumps back to any parent folder. The location is stored in the hash (<code>#/path</code>), so it survives reloads and can be bookmarked.',
      'Drag the divider between the sidebar and the content to resize the sidebar.',
    ],
  },
  {
    title: 'Search & sort',
    items: [
      'Type in the search box and press Enter to search recursively under the current folder.',
      'Sort by name / modified time / size; click the same column again to reverse the order.',
    ],
  },
  {
    title: 'File operations',
    items: [
      'Upload: click "Upload" and pick files; progress is shown live.',
      'New folder / new file; rename and delete (both irreversible, deletion asks for confirmation).',
      'Download zip: package the current folder — or any subfolder — as a zip archive.',
      'Copy the full path / the curl download command / the curl upload command for command-line use.',
    ],
  },
  {
    title: 'curl examples',
    items: [
      'Upload a file: <code>curl -T local/path/to/file.ext http://host:port/path/to/file.ext</code>',
      'Download a file: <code>curl -O http://host:port/path/to/file.ext</code>',
      'List a folder: <code>curl "http://host:port/?json="</code>',
      'Search: <code>curl "http://host:port/?json=&amp;q=keyword"</code>',
      'Rename / move: <code>curl -X MOVE -H "Destination: http://host:port/new/path" http://host:port/old/path</code>',
      'Delete: <code>curl -X DELETE http://host:port/path/to/file.ext</code>',
      'Create a folder: <code>curl -X MKCOL http://host:port/path/to/dir</code>',
      'Download a folder as zip: append <code>?zip</code>, e.g. <code>curl -o dir.zip "http://host:port/path/to/dir?zip"</code>',
      'Get the file SHA256: append <code>?hash</code>, e.g. <code>curl "http://host:port/path/to/file.ext?hash"</code>',
    ],
  },
  {
    title: 'Preview & edit',
    items: [
      'Images, video, audio, PDF, Markdown, code, JSON and plain text can be previewed in the browser.',
      'Text-like files can be edited in place: click "Edit", then <code>Ctrl/Cmd+S</code> to save. Markdown editing can be toggled to preview.',
    ],
  },
  {
    title: 'Appearance & language',
    items: [
      'The sidebar footer switches the theme: auto / light / dark.',
      'The sidebar footer also switches the UI language: Chinese / English.',
      'Preferences are stored locally in the browser and persist across reloads.',
    ],
  },
]

export const HELP_CONTENT: Record<Locale, HelpSection[]> = { zh, en }
