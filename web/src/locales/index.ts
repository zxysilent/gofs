// Lightweight i18n for the web UI (no external dependency).
//
// Usage in a component:
//   import { t } from '../locales'
//   <button>{{ t('toolbar.upload') }}</button>
//
// `t()` reads `locale.value`, so templates re-render automatically when the
// locale changes — no special subscription needed.
import { ref } from 'vue'

export type Locale = 'zh' | 'en'

const STORAGE_KEY = 'gofs.locale'

function detectLocale(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'zh' || saved === 'en') return saved
    const nav = (navigator.language || navigator.languages?.[0] || '').toLowerCase()
    return nav.startsWith('zh') ? 'zh' : 'en'
  } catch {
    return 'zh'
  }
}

export const locale = ref<Locale>(detectLocale())

const zh: Record<string, string> = {
  // common
  'common.loading': '加载中...',
  'common.cancel': '取消',
  'common.ok': '确定',
  'common.delete': '确定删除',
  'common.close': '关闭',
  'common.refresh': '刷新',
  'common.retry': '重试',
  'common.loadFailed': '加载失败：{msg}',

  // app
  'app.root': '根目录',
  'app.agentDocTitle': 'gofs 接口手册',
  'app.operationFailed': '操作失败: {msg}',
  'app.splitterTitle': '拖动调整侧栏宽度',

  // toasts
  'toast.copyPath': '已复制完整路径',
  'toast.copyName': '已复制文件名',
  'toast.copyRelPath': '已复制路径',
  'toast.copyCmd': '已复制命令',
  'toast.copyFailed': '复制失败，请手动复制: {text}',
  'toast.renamed': '已重命名为 "{name}"',
  'toast.deleted': '已删除 "{name}"',
  'toast.folderCreated': '已创建文件夹 "{name}"',
  'toast.fileCreated': '已创建文件 "{name}"',
  'toast.saved': '已保存',
  'toast.saveFailed': '保存失败: {msg}',
  'toast.copiedManual': '已复制接口手册全文',
  'toast.copyFailedManual': '复制失败，请手动选择复制',
  'toast.copiedGuide': '已复制使用说明全文',
  'toast.copyFailedGuide': '复制失败，请手动选择复制',
  'toast.copyHash': '已复制 SHA256',

  // dialogs
  'dialog.rename': '重命名',
  'dialog.renamePlaceholder': '请输入新名称',
  'dialog.deleteConfirm': '删除确认',
  'dialog.deleteItemMsg': '确定删除 "{name}" 吗？此操作不可恢复。',
  'dialog.deleteDirMsg': '确定删除目录 "{name}" 及其全部内容吗？此操作不可恢复。',
  'dialog.newFolder': '新建文件夹',
  'dialog.newFolderPlaceholder': '请输入文件夹名称',
  'dialog.newFile': '新建文件',
  'dialog.newFilePlaceholder': '请输入文件名',
  'dialog.discard': '放弃修改',
  'dialog.discardEditMsg': '内容已修改，确定放弃未保存的修改吗？',
  'dialog.discardCloseMsg': '内容已修改，确定关闭并放弃未保存的修改吗？',

  // file kinds
  'kind.dir': '文件夹',
  'kind.image': '图片',
  'kind.video': '视频',
  'kind.audio': '音频',
  'kind.pdf': 'PDF',
  'kind.markdown': 'Markdown',
  'kind.html': '网页',
  'kind.json': 'JSON',
  'kind.text': '文本',
  'kind.code': '代码',
  'kind.archive': '压缩包',
  'kind.file': '文件',

  // file list
  'list.name': '名称',
  'list.type': '类型',
  'list.size': '大小',
  'list.mtime': '修改时间',
  'list.items': '{n} 项',
  'list.today': '今天 {time}',

  // toolbar
  'toolbar.toggleSidebar': '切换侧栏',
  'toolbar.search': '搜索文件...',
  'toolbar.listView': '列表视图',
  'toolbar.gridView': '网格视图',
  'toolbar.upload': '上传',
  'toolbar.sort': '排序',
  'toolbar.sortName': '名称',
  'toolbar.sortMtime': '修改时间',
  'toolbar.sortSize': '大小',
  'toolbar.newFolder': '新建文件夹',
  'toolbar.newFile': '新建文件',
  'toolbar.curlUpload': 'cURL 上传',
  'toolbar.curlUploadTitle': '复制 curl 上传到此目录的命令',
  'toolbar.archive': '打包下载',

  // actions
  'action.downloadArchive': '打包下载',
  'action.copyPath': '复制完整路径',
  'action.copyName': '复制文件名',
  'action.copyCurl': '复制 curl 下载命令',
  'action.openNew': '新窗口打开',
  'action.rename': '重命名',
  'action.delete': '删除',

  // tree context menu
  'menu.newFolder': '新建子目录',
  'menu.copyRelPath': '复制路径',
  'menu.more': '更多操作',
  'menu.copyCurl': '复制 cURL',
  'menu.openNew': '新窗口',

  // sidebar
  'sidebar.fileList': '文件列表',
  'sidebar.help': '使用说明',
  'sidebar.rootTitle': '回到根目录',
  'sidebar.tabGuide': '使用说明',
  'sidebar.tabAgent': 'Agent说明',
  'sidebar.copyAll': '复制全文',
  'sidebar.copyAllTitle': '复制当前说明的全文',
  'sidebar.rawFile': '原始文件',
  'sidebar.copyUrl': '复制地址',
  'sidebar.rawFileTitle': '在新标签页打开原始 Markdown',
  'sidebar.copyUrlTitle': '复制原始 Markdown 地址',
  'sidebar.projectUrl': '项目地址',
  'sidebar.projectUrlTitle': '在新标签页打开项目主页',
  'sidebar.copiedUrl': '已复制地址: {url}',
  'sidebar.copyUrlFailed': '复制失败: {url}',
  'sidebar.filterPlaceholder': '过滤目录…',
  'sidebar.filterEmpty': '没有匹配的目录',
  'sidebar.collapseAll': '全部折叠',

  // preview
  'preview.edit': '编辑',
  'preview.download': '下载',
  'preview.downloadFile': '下载文件',
  'preview.copyLink': '复制链接',
  'preview.curlDownload': '复制 cURL',
  'preview.openNew': '新窗口',
  'preview.openNewTitle': '在浏览器新窗口打开',
  'preview.fullscreen': '全屏',
  'preview.fullscreenTitle': '铺满窗口显示',
  'preview.exitFullscreen': '退出全屏',
  'preview.exitFullscreenTitle': '恢复默认窗口大小',
  'preview.unsaved': '●未保存',
  'preview.preview': '预览',
  'preview.rendered': '渲染',
  'preview.source': '源码',
  'preview.renderedTitle': '以网页形式渲染预览',
  'preview.sourceTitle': '查看 HTML 源码',
  'preview.continueEdit': '继续编辑',
  'preview.saving': '保存中...',
  'preview.save': '保存',
  'preview.notSupported': '此文件类型暂不支持在线预览',
  'preview.tooLarge': '文件过大，无法在线预览，请下载查看。',
  'preview.copyHash': '复制 SHA256',

  // help page
  'help.back': '返回',
  'help.backTitle': '返回文件列表',
  'help.title': '接口手册',
  'help.copyManual': '复制手册',
  'help.copyManualTitle': '复制使用手册内容',

  // theme / locale
  'settings.theme': '主题',
  'settings.themeAuto': '自动',
  'settings.themeLight': '亮色',
  'settings.themeDark': '暗色',
  'settings.language': '语言',
}

const en: Record<string, string> = {
  // common
  'common.loading': 'Loading...',
  'common.cancel': 'Cancel',
  'common.ok': 'OK',
  'common.delete': 'Delete',
  'common.close': 'Close',
  'common.refresh': 'Refresh',
  'common.retry': 'Retry',
  'common.loadFailed': 'Failed to load: {msg}',

  // app
  'app.root': 'Root',
  'app.agentDocTitle': 'gofs API Manual',
  'app.operationFailed': 'Operation failed: {msg}',
  'app.splitterTitle': 'Drag to resize the sidebar',

  // toasts
  'toast.copyPath': 'Full path copied',
  'toast.copyName': 'File name copied',
  'toast.copyRelPath': 'Path copied',
  'toast.copyCmd': 'Command copied',
  'toast.copyFailed': 'Copy failed, please copy manually: {text}',
  'toast.renamed': 'Renamed to "{name}"',
  'toast.deleted': 'Deleted "{name}"',
  'toast.folderCreated': 'Folder "{name}" created',
  'toast.fileCreated': 'File "{name}" created',
  'toast.saved': 'Saved',
  'toast.saveFailed': 'Save failed: {msg}',
  'toast.copiedManual': 'API manual copied',
  'toast.copyFailedManual': 'Copy failed, please select and copy manually',
  'toast.copiedGuide': 'Full guide copied',
  'toast.copyFailedGuide': 'Copy failed, please select and copy manually',
  'toast.copyHash': 'SHA256 copied',

  // dialogs
  'dialog.rename': 'Rename',
  'dialog.renamePlaceholder': 'Enter a new name',
  'dialog.deleteConfirm': 'Confirm deletion',
  'dialog.deleteItemMsg': 'Delete "{name}"? This cannot be undone.',
  'dialog.deleteDirMsg': 'Delete folder "{name}" and all of its contents? This cannot be undone.',
  'dialog.newFolder': 'New folder',
  'dialog.newFolderPlaceholder': 'Enter folder name',
  'dialog.newFile': 'New file',
  'dialog.newFilePlaceholder': 'Enter file name',
  'dialog.discard': 'Discard changes',
  'dialog.discardEditMsg': 'The content has changed. Discard unsaved changes?',
  'dialog.discardCloseMsg': 'The content has changed. Close and discard unsaved changes?',

  // file kinds
  'kind.dir': 'Folder',
  'kind.image': 'Image',
  'kind.video': 'Video',
  'kind.audio': 'Audio',
  'kind.pdf': 'PDF',
  'kind.markdown': 'Markdown',
  'kind.html': 'HTML',
  'kind.json': 'JSON',
  'kind.text': 'Text',
  'kind.code': 'Code',
  'kind.archive': 'Archive',
  'kind.file': 'File',

  // file list
  'list.name': 'Name',
  'list.type': 'Type',
  'list.size': 'Size',
  'list.mtime': 'Modified',
  'list.items': '{n} items',
  'list.today': 'Today {time}',

  // toolbar
  'toolbar.toggleSidebar': 'Toggle sidebar',
  'toolbar.search': 'Search files...',
  'toolbar.listView': 'List view',
  'toolbar.gridView': 'Grid view',
  'toolbar.upload': 'Upload',
  'toolbar.sort': 'Sort',
  'toolbar.sortName': 'Name',
  'toolbar.sortMtime': 'Modified',
  'toolbar.sortSize': 'Size',
  'toolbar.newFolder': 'New folder',
  'toolbar.newFile': 'New file',
  'toolbar.curlUpload': 'cURL Upload',
  'toolbar.curlUploadTitle': 'Copy the curl command to upload into this folder',
  'toolbar.archive': 'Download zip',

  // actions
  'action.downloadArchive': 'Download as zip',
  'action.copyPath': 'Copy full path',
  'action.copyName': 'Copy file name',
  'action.copyCurl': 'Copy curl download command',
  'action.openNew': 'Open in new tab',
  'action.rename': 'Rename',
  'action.delete': 'Delete',

  // tree context menu
  'menu.newFolder': 'New subfolder',
  'menu.copyRelPath': 'Copy path',
  'menu.more': 'More actions',
  'menu.copyCurl': 'Copy cURL',
  'menu.openNew': 'New tab',

  // sidebar
  'sidebar.fileList': 'Files',
  'sidebar.help': 'Guide',
  'sidebar.rootTitle': 'Back to root',
  'sidebar.tabGuide': 'Guide',
  'sidebar.tabAgent': 'Agent guide',
  'sidebar.copyAll': 'Copy all',
  'sidebar.copyAllTitle': 'Copy the full text of this guide',
  'sidebar.rawFile': 'Raw file',
  'sidebar.copyUrl': 'Copy URL',
  'sidebar.rawFileTitle': 'Open the raw Markdown in a new tab',
  'sidebar.copyUrlTitle': 'Copy the raw Markdown URL',
  'sidebar.projectUrl': 'Project',
  'sidebar.projectUrlTitle': 'Open the project homepage in a new tab',
  'sidebar.copiedUrl': 'URL copied: {url}',
  'sidebar.copyUrlFailed': 'Copy failed: {url}',
  'sidebar.filterPlaceholder': 'Filter folders…',
  'sidebar.filterEmpty': 'No matching folders',
  'sidebar.collapseAll': 'Collapse all',

  // preview
  'preview.edit': 'Edit',
  'preview.download': 'Download',
  'preview.downloadFile': 'Download file',
  'preview.copyLink': 'Copy link',
  'preview.curlDownload': 'Copy cURL',
  'preview.openNew': 'New tab',
  'preview.openNewTitle': 'Open in a new browser tab',
  'preview.fullscreen': 'Fullscreen',
  'preview.fullscreenTitle': 'Fill the browser window',
  'preview.exitFullscreen': 'Exit fullscreen',
  'preview.exitFullscreenTitle': 'Back to default size',
  'preview.unsaved': '●Unsaved',
  'preview.preview': 'Preview',
  'preview.rendered': 'Rendered',
  'preview.source': 'Source',
  'preview.renderedTitle': 'Preview as a web page',
  'preview.sourceTitle': 'Show the HTML source',
  'preview.continueEdit': 'Keep editing',
  'preview.saving': 'Saving...',
  'preview.save': 'Save',
  'preview.notSupported': 'Preview is not available for this file type',
  'preview.tooLarge': 'File is too large to preview. Please download it.',
  'preview.copyHash': 'Copy SHA256',

  // help page
  'help.back': 'Back',
  'help.backTitle': 'Back to the file list',
  'help.title': 'API manual',
  'help.copyManual': 'Copy manual',
  'help.copyManualTitle': 'Copy the manual content',

  // theme / locale
  'settings.theme': 'Theme',
  'settings.themeAuto': 'Auto',
  'settings.themeLight': 'Light',
  'settings.themeDark': 'Dark',
  'settings.language': 'Language',
}

const messages: Record<Locale, Record<string, string>> = { zh, en }

/** Translate a key, optionally interpolating `{name}` placeholders. */
export function t(key: string, vars?: Record<string, string | number>): string {
  const dict = messages[locale.value] ?? zh
  let s = dict[key] ?? zh[key] ?? key
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      s = s.split(`{${k}}`).join(String(v))
    }
  }
  return s
}

function applyLangAttr(l: Locale) {
  document.documentElement.lang = l === 'zh' ? 'zh-CN' : 'en'
}

export function setLocale(l: Locale): void {
  locale.value = l
  try {
    localStorage.setItem(STORAGE_KEY, l)
  } catch { /* ignore */ }
  applyLangAttr(l)
}

export function toggleLocale(): void {
  setLocale(locale.value === 'zh' ? 'en' : 'zh')
}

/** Apply the initial locale to <html lang> (call once at startup). */
export function initLocale(): void {
  applyLangAttr(locale.value)
}
