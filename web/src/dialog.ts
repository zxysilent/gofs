// Custom modal dialogs + toast, replacing native prompt/confirm/alert which
// are not supported in some (headless) environments.
import { esc } from './icons'
import { t } from './i18n'

function ensureModalRoot(): HTMLElement {
  let root = document.getElementById('modal-root')
  if (!root) {
    root = document.createElement('div')
    root.id = 'modal-root'
    document.body.appendChild(root)
  }
  return root
}

export function showInputDialog(title: string, placeholder: string, defaultValue: string): Promise<string | null> {
  return new Promise((resolve) => {
    const root = ensureModalRoot()
    root.innerHTML = `
      <div class="modal-overlay" id="modal-overlay">
        <div class="modal-box">
          <div class="modal-title">${esc(title)}</div>
          <input class="modal-input" id="modal-input" placeholder="${esc(placeholder)}" value="${esc(defaultValue)}" />
          <div class="modal-actions">
            <button class="btn" id="modal-cancel">${esc(t('common.cancel'))}</button>
            <button class="btn btn-primary" id="modal-ok">${esc(t('common.ok'))}</button>
          </div>
        </div>
      </div>`
    const input = document.getElementById('modal-input') as HTMLInputElement
    const finish = (val: string | null) => { root.innerHTML = ''; resolve(val) }
    document.getElementById('modal-cancel')!.onclick = () => finish(null)
    document.getElementById('modal-ok')!.onclick = () => finish(input.value.trim())
    document.getElementById('modal-overlay')!.onclick = (e) => { if ((e.target as HTMLElement).id === 'modal-overlay') finish(null) }
    input.onkeydown = (e) => {
      if (e.key === 'Enter') { e.preventDefault(); finish(input.value.trim()) }
      if (e.key === 'Escape') finish(null)
    }
    input.focus()
    input.select()
  })
}

export function showConfirmDialog(title: string, message: string): Promise<boolean> {
  return new Promise((resolve) => {
    const root = ensureModalRoot()
    root.innerHTML = `
      <div class="modal-overlay" id="modal-overlay">
        <div class="modal-box">
          <div class="modal-title">${esc(title)}</div>
          <div class="modal-message">${esc(message)}</div>
          <div class="modal-actions">
            <button class="btn" id="modal-cancel">${esc(t('common.cancel'))}</button>
            <button class="btn btn-danger" id="modal-ok">${esc(t('common.delete'))}</button>
          </div>
        </div>
      </div>`
    const finish = (val: boolean) => { root.innerHTML = ''; resolve(val) }
    document.getElementById('modal-cancel')!.onclick = () => finish(false)
    document.getElementById('modal-ok')!.onclick = () => finish(true)
    document.getElementById('modal-overlay')!.onclick = (e) => { if ((e.target as HTMLElement).id === 'modal-overlay') finish(false) }
    const ok = document.getElementById('modal-ok')!
    ok.focus()
    ok.onkeydown = (e) => { if (e.key === 'Escape') finish(false) }
  })
}

export function showToast(message: string, type?: 'info' | 'success' | 'error'): void {
  let c = document.getElementById('toast-container')
  if (!c) {
    c = document.createElement('div')
    c.id = 'toast-container'
    document.body.appendChild(c)
  }
  const el = document.createElement('div')
  el.className = 'toast ' + (type || 'info')
  el.textContent = message
  c.appendChild(el)
  setTimeout(() => { if (el.parentNode) el.remove() }, 2500)
}

// Robust clipboard copy with execCommand fallback.
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch { /* fall through */ }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    ta.style.top = '0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    ta.setSelectionRange(0, text.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}
