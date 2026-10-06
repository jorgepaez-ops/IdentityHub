// Build of the technical report: Markdown -> HTML (print stylesheet) -> PDF with Chromium.
//
//   node build.mjs            (or: make informe)
//
// Source:  informe-tecnico.md   (front matter + Markdown + the directives below)
// Output:  informe-tecnico.pdf  (next to the source)
//
// Directives (one per line, resolved before Markdown is parsed):
//   {{mermaid:REL_PATH#N|Caption}}       N-th ```mermaid block of REL_PATH (relative to this folder)
//   {{code:LANG:REL_PATH:FROM-TO}}       lines FROM..TO (1-based, inclusive) of a real repository file
//   {{redact:REL_PATH|Caption}}          pretty JSON of a scanner report with Secret/Match masked
//
// Mermaid diagrams are rendered inside Chromium (vector, no intermediate images). The table of
// contents gets its page numbers in two passes: the PDF is printed once with placeholders, the page
// of each heading is read back with pdf.js, and it is printed again with the real numbers.

import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { createRequire } from 'node:module'
import { marked } from 'marked'
import { chromium } from 'playwright-core'

const here = path.dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)
const SOURCE = path.join(here, 'informe-tecnico.md')
const OUTPUT = path.join(here, 'informe-tecnico.pdf')
const MERMAID_JS = require.resolve('mermaid/dist/mermaid.min.js')

const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
const slug = (s) =>
  s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')

// ── 1. Front matter ──────────────────────────────────────────────────────
function splitFrontMatter(text) {
  const m = text.match(/^---\n([\s\S]*?)\n---\n/)
  if (!m) throw new Error('informe-tecnico.md needs a front matter block')
  const meta = {}
  for (const line of m[1].split('\n')) {
    const i = line.indexOf(':')
    if (i > 0) meta[line.slice(0, i).trim()] = line.slice(i + 1).trim()
  }
  return { meta, body: text.slice(m[0].length) }
}

// ── 2. Directives ────────────────────────────────────────────────────────
function mermaidBlock(rel, n) {
  const file = path.resolve(here, rel)
  const blocks = [...fs.readFileSync(file, 'utf8').matchAll(/```mermaid\n([\s\S]*?)```/g)].map((m) => m[1])
  if (!blocks[n - 1]) throw new Error(`${rel} has no mermaid block #${n}`)
  return blocks[n - 1].trimEnd()
}

function codeLines(rel, range) {
  const [from, to] = range.split('-').map(Number)
  const lines = fs.readFileSync(path.resolve(here, '../..', rel), 'utf8').split('\n')
  // An edited workflow must break the build, not silently shift the quoted excerpt.
  if (!(from >= 1 && to >= from && to <= lines.length)) throw new Error(`${rel}:${range} is outside the file (${lines.length} lines)`)
  const excerpt = lines.slice(from - 1, to)
  if (excerpt.every((line) => line.trim() === '')) throw new Error(`${rel}:${range} is empty`)
  return excerpt.join('\n')
}

// The seeded baseline reports contain real-looking (fake) secrets: the report only ever shows the
// redacted copy, and this masks Secret/Match/Line again as a second line of defence.
function redactedReport(rel, max) {
  const data = JSON.parse(fs.readFileSync(path.resolve(here, '../..', rel), 'utf8'))
  const mask = (o) => {
    if (Array.isArray(o)) return o.map(mask)
    if (o && typeof o === 'object')
      return Object.fromEntries(Object.entries(o).map(([k, v]) => [k, ['Secret', 'Match', 'Line'].includes(k) ? '<redactado>' : mask(v)]))
    return o
  }
  const first = mask(data.slice(0, max))
  return JSON.stringify(first, null, 2)
}

function expandDirectives(body) {
  let out = body.replace(/^\{\{mermaid:([^#|}]+)#(\d+)(?:\|([^}]*))?\}\}$/gm, (_, rel, n, cap) => {
    const land = cap && cap.startsWith('@L ')
    const text = land ? cap.slice(3) : cap
    return '```mermaid\n' + (land ? '%%landscape\n' : '') + (text ? `%%caption: ${text}\n` : '') + mermaidBlock(rel, Number(n)) + '\n```\n'
  })
  out = out.replace(/^\{\{code:([^:}]+):([^:}]+):(\d+-\d+)\}\}$/gm, (_, lang, rel, range) => {
    return '```' + lang + '\n' + codeLines(rel, range) + '\n```\n\n' + `<p class="src">Fuente: <code>${rel}</code>, líneas ${range}.</p>\n`
  })
  out = out.replace(/^\{\{redact:([^|}]+)\|?([^}]*)\}\}$/gm, (_, rel) => {
    return '```json\n' + redactedReport(rel, 1) + '\n```\n\n' + `<p class="src">Fuente: <code>${rel}</code> (primer hallazgo de doce; <code>Secret</code> y <code>Match</code> enmascarados).</p>\n`
  })
  return out
}

// ── 3. Markdown -> HTML with numbered headings ───────────────────────────
const headings = [] // {level, num, title, id}

function render(body) {
  const counters = [0, 0]
  let annexIdx = 0
  let inAnnex = false
  let figN = 0
  const renderer = new marked.Renderer()

  renderer.heading = function ({ tokens, depth }) {
    const text = this.parser.parseInline(tokens)
    const plain = tokens.map((t) => t.raw ?? t.text ?? '').join('').replace(/[`*_]/g, '')
    if (depth > 2) return `<h${depth}>${text}</h${depth}>\n`
    let num
    if (depth === 1) {
      inAnnex = /^Anexo\b/i.test(plain)
      if (inAnnex) {
        annexIdx += 1
        num = ''
        counters[1] = 0
      } else {
        counters[0] += 1
        counters[1] = 0
        num = String(counters[0])
      }
    } else {
      counters[1] += 1
      num = inAnnex ? `${String.fromCharCode(64 + annexIdx)}.${counters[1]}` : `${counters[0]}.${counters[1]}`
    }
    const id = slug(`${num} ${plain}`)
    headings.push({ level: depth, num, title: plain, id })
    const numSpan = num ? `<span class="num">${num}</span> ` : ''
    return `<h${depth} id="${id}" class="${depth === 1 && inAnnex ? 'annex' : ''}">${numSpan}${text}</h${depth}>\n`
  }

  renderer.code = ({ text, lang }) => {
    if (lang === 'mermaid') {
      figN += 1
      let src = text
      let land = false
      let cap = ''
      src = src.replace(/^%%landscape\n/m, () => ((land = true), ''))
      src = src.replace(/^%%caption: (.*)\n/m, (_, c) => ((cap = c), ''))
      const fig = `<figure class="diagrama"><div class="mermaid">${esc(src)}</div>${cap ? `<figcaption>Figura ${figN}. ${esc(cap)}</figcaption>` : ''}</figure>\n`
      return land ? `<div class="apaisada">${fig}</div>\n` : fig
    }
    return `<pre class="code"><code class="lang-${lang || 'text'}">${esc(text)}</code></pre>\n`
  }

  renderer.image = ({ href, title, text }) => {
    figN += 1
    return `<figure class="captura" data-fig="${figN}"><img src="${href}" alt="${esc(text)}"><figcaption>Figura ${figN}. ${esc(title || text)}</figcaption></figure>`
  }

  // Relative links point at repository files: in a PDF they cannot be followed, so keep the text.
  renderer.link = function ({ href, tokens }) {
    const text = this.parser.parseInline(tokens)
    return /^(https?:|#)/.test(href) ? `<a href="${href}">${text}</a>` : text
  }

  renderer.table = function (token) {
    const head = token.header.map((c) => `<th>${this.parser.parseInline(c.tokens)}</th>`).join('')
    const rows = token.rows.map((r) => `<tr>${r.map((c) => `<td>${this.parser.parseInline(c.tokens)}</td>`).join('')}</tr>`).join('')
    const idcol = token.header[0].text === 'Id' ? ' class="idcol"' : ''
    return `<div class="tabla"><table${idcol}><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>\n`
  }

  marked.use({ renderer, gfm: true })
  let html = marked.parse(body)

  html = html.replace(/«\[completar:[^\]]*\]»/g, (m) => `<span class="placeholder">${m}</span>`)
  return html
}

// ── 4. Cover and table of contents ───────────────────────────────────────
function coverHtml(meta) {
  const row = (k, v) => `<tr><th>${k}</th><td>${v}</td></tr>`
  return `<section class="portada">
  <p class="curso">${meta.institucion || ''}</p>
  <h1 class="titulo">${meta.proyecto}</h1>
  <p class="subtitulo">${meta.subtitulo}</p>
  <table class="datos">
    ${row('Proyecto', meta.proyecto)}
    ${row('Curso', meta.curso)}
    ${row('Docente', meta.docente)}
    ${row('Equipo', meta.equipo)}
    ${row('Repositorio', `<a href="${meta.repositorio}">${meta.repositorio}</a>`)}
    ${row('Licencia del producto', meta.licencia)}
    ${row('Fecha del documento', meta.fecha)}
    ${row('Fecha de entrega', meta.entrega)}
  </table>
</section>`
}

function tocHtml(pages) {
  const items = headings
    .map((h) => {
      const pg = pages?.get(h.id) ?? '00'
      const label = h.num ? `<span class="n">${h.num}</span>${esc(h.title)}` : esc(h.title)
      return `<li class="l${h.level}"><a href="#${h.id}"><span class="t">${label}</span><span class="d"></span><span class="p">${pg}</span></a></li>`
    })
    .join('\n')
  return `<section class="toc"><h1 class="toc-title">Tabla de contenido</h1><ul>${items}</ul></section>`
}

// ── 5. Stylesheet ────────────────────────────────────────────────────────
const CSS = `
@page { size: A4; margin: 22mm 18mm 22mm 18mm;
  @bottom-center { content: counter(page); font: 9pt "Helvetica Neue", Arial, sans-serif; color: #555; }
  @bottom-left { content: "Identity Hub — Informe técnico"; font: 8pt "Helvetica Neue", Arial, sans-serif; color: #888; }
}
@page :first { margin: 0; @bottom-center { content: none; } @bottom-left { content: none; } }
:root { --ink: #1f2937; --muted: #555; --accent: #1d4e89; --line: #cfd6df; --soft: #f3f6fa; }
* { box-sizing: border-box; }
html { font: 10pt/1.45 "Helvetica Neue", Arial, "Segoe UI", sans-serif; color: var(--ink); -webkit-print-color-adjust: exact; print-color-adjust: exact; }
body { margin: 0; }
h1, h2, h3, h4 { font-family: "Helvetica Neue", Arial, sans-serif; color: var(--accent); line-height: 1.2; break-after: avoid; }
h1 { font-size: 20pt; margin: 0 0 6mm; padding-bottom: 2mm; border-bottom: 2px solid var(--accent); break-before: page; }
h1.annex { color: #333; border-color: #333; }
h2 { font-size: 14pt; margin: 8mm 0 3mm; }
h3 { font-size: 11.5pt; margin: 6mm 0 2mm; color: #2b3a4a; }
h4 { font-size: 10pt; margin: 4mm 0 1mm; }
.num { margin-right: 2mm; }
p { margin: 0 0 3mm; text-align: justify; hyphens: auto; }
ul, ol { margin: 0 0 3mm; padding-left: 6mm; }
li { margin-bottom: 1mm; }
a { color: var(--accent); text-decoration: none; }
code { font: 8.6pt/1.3 "SF Mono", Menlo, Consolas, monospace; background: var(--soft); padding: 0 1mm; border-radius: 1mm; word-break: break-word; }
pre.code { font: 7.6pt/1.35 "SF Mono", Menlo, Consolas, monospace; background: #f6f8fa; border: 1px solid var(--line); border-left: 3px solid var(--accent); padding: 2.5mm 3mm; margin: 0 0 1.5mm; white-space: pre-wrap; word-break: break-word; overflow-wrap: anywhere; }
pre.code code { background: none; padding: 0; font: inherit; }
p.src { font-size: 8pt; color: var(--muted); margin-bottom: 4mm; text-align: left; }
.tabla { margin: 0 0 4mm; }
table { border-collapse: collapse; width: 100%; font-size: 8.4pt; line-height: 1.3; }
th, td { border: 1px solid var(--line); padding: 1.2mm 1.8mm; vertical-align: top; text-align: left; }
th { background: var(--soft); color: #1b2a3a; }
thead { display: table-header-group; }
tr { break-inside: avoid; }
td code, th code { font-size: 7.6pt; }
table.idcol td:first-child { white-space: nowrap; }
figure { margin: 3mm 0 5mm; break-inside: avoid; text-align: center; }
figure.diagrama svg { max-width: 100%; height: auto; max-height: 230mm; }
.apaisada { page: land; break-before: page; break-after: page; }
.apaisada figure.diagrama svg { max-height: 150mm; }
@page land { size: A4 landscape; margin: 16mm 14mm 18mm 14mm;
  @bottom-center { content: counter(page); font: 9pt "Helvetica Neue", Arial, sans-serif; color: #555; }
  @bottom-left { content: "Identity Hub — Informe técnico"; font: 8pt "Helvetica Neue", Arial, sans-serif; color: #888; }
}
figure.captura img { max-width: 100%; max-height: 135mm; border: 1px solid var(--line); }
figcaption, p.caption-fig { font-size: 8.4pt; color: var(--muted); margin-top: 1.5mm; text-align: center; }
blockquote { margin: 0 0 3mm; padding: 1mm 4mm; border-left: 3px solid var(--line); color: #444; background: #fafbfc; }
.placeholder { background: #fff3a8; border: 1px dashed #b38f00; padding: 0 1mm; color: #5c4a00; font-weight: 600; }
.sevalta { color: #9b1c1c; font-weight: 600; }
/* cover */
.portada { height: 297mm; padding: 30mm 22mm 20mm; background: linear-gradient(180deg, #0f2a4a 0, #0f2a4a 105mm, #fff 105mm); break-after: page; }
.portada .curso { color: #c9d8ec; font-size: 11pt; letter-spacing: .08em; text-transform: uppercase; margin: 0 0 8mm; text-align: left; }
.portada h1.titulo { color: #fff; border: 0; font-size: 36pt; margin: 0 0 3mm; padding: 0; break-before: auto; }
.portada .subtitulo { color: #dce7f5; font-size: 14pt; text-align: left; margin: 0 0 40mm; }
.portada table.datos { font-size: 10.5pt; border: 0; margin-top: 12mm; }
.portada table.datos th { width: 48mm; background: none; border: 0; border-bottom: 1px solid var(--line); color: var(--accent); padding: 3mm 2mm; }
.portada table.datos td { border: 0; border-bottom: 1px solid var(--line); padding: 3mm 2mm; }
/* toc */
.toc { break-after: page; }
.toc h1.toc-title { break-before: page; }
.toc ul { list-style: none; padding: 0; margin: 0; }
.toc li { margin: 0; }
.toc li.l1 { margin-top: 2.2mm; font-weight: 700; }
.toc li.l2 { padding-left: 8mm; font-size: 9.2pt; }
.toc a { display: flex; align-items: baseline; color: var(--ink); }
.toc .t { flex: none; max-width: 85%; }
.toc .n { display: inline-block; min-width: 9mm; }
.toc .d { flex: 1; border-bottom: 1px dotted #999; margin: 0 1.5mm; }
.toc .p { flex: none; font-variant-numeric: tabular-nums; }
`

function pageHtml(meta, bodyHtml, pages) {
  return `<!doctype html>
<html lang="es"><head><meta charset="utf-8"><title>${esc(meta.proyecto)} — Informe técnico</title>
<base href="${pathToFileURL(here + '/').href}">
<style>${CSS}</style></head>
<body>${coverHtml(meta)}${tocHtml(pages)}${bodyHtml}</body></html>`
}

// ── 6. Chromium ──────────────────────────────────────────────────────────
async function printPdf(browser, html, outFile) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'informe-'))
  const htmlFile = path.join(tmp, 'informe.html')
  fs.writeFileSync(htmlFile, html)
  const page = await browser.newPage()
  const errors = []
  page.on('pageerror', (e) => errors.push(String(e)))
  await page.goto(pathToFileURL(htmlFile).href, { waitUntil: 'load' })
  await page.addScriptTag({ path: MERMAID_JS })
  const failed = await page.evaluate(async () => {
    // eslint-disable-next-line no-undef
    mermaid.initialize({
      startOnLoad: false,
      theme: 'neutral',
      securityLevel: 'loose',
      fontFamily: '"Helvetica Neue", Arial, sans-serif',
      flowchart: { htmlLabels: true, useMaxWidth: true },
      sequence: { useMaxWidth: true, mirrorActors: false },
    })
    const bad = []
    for (const el of document.querySelectorAll('.mermaid')) {
      try {
        // eslint-disable-next-line no-undef
        await mermaid.run({ nodes: [el] })
      } catch (e) {
        bad.push(String(e).slice(0, 200))
      }
    }
    await document.fonts.ready
    return bad
  })
  if (failed.length) throw new Error('Mermaid failed: ' + failed.join(' | '))
  if (errors.length) throw new Error('Page errors: ' + errors.join(' | '))
  await page.pdf({ path: outFile, preferCSSPageSize: true, printBackground: true, outline: true, tagged: true })
  await page.close()
  fs.rmSync(tmp, { recursive: true, force: true })
}

async function pageOfHeadings(pdfFile) {
  const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs')
  const doc = await pdfjs.getDocument({ data: new Uint8Array(fs.readFileSync(pdfFile)), useSystemFonts: true }).promise
  const norm = (s) => s.replace(/\s+/g, '').toLowerCase()
  const texts = []
  for (let i = 1; i <= doc.numPages; i++) {
    const tc = await (await doc.getPage(i)).getTextContent()
    texts.push(norm(tc.items.map((it) => it.str).join(' ')))
  }
  // The cover and the table of contents come first: start the search after the last page that
  // still holds TOC entries (the one right before the first body heading, a page break follows).
  const first = headings[0]
  let start = 2
  while (start < texts.length && !texts[start].includes(norm(`${first.num}${first.title}`))) start++
  const pages = new Map()
  let cursor = start
  for (const h of headings) {
    const needle = norm(`${h.num}${h.title}`)
    let found = -1
    for (let i = cursor; i < texts.length; i++) {
      if (texts[i].includes(needle)) {
        found = i
        break
      }
    }
    if (found < 0) throw new Error(`Heading not found in the PDF: ${h.num} ${h.title}`)
    pages.set(h.id, found + 1)
    cursor = found
  }
  return { pages, total: doc.numPages }
}

// ── main ─────────────────────────────────────────────────────────────────
const { meta, body } = splitFrontMatter(fs.readFileSync(SOURCE, 'utf8'))
const bodyHtml = render(expandDirectives(body))
const browser = await chromium.launch()
try {
  const draft = path.join(os.tmpdir(), `informe-draft-${process.pid}.pdf`)
  await printPdf(browser, pageHtml(meta, bodyHtml, null), draft)
  const first = await pageOfHeadings(draft)
  fs.rmSync(draft, { force: true })
  // Print to a temp file and move it into place only after the pagination check passes, so a failed
  // build never leaves a PDF with a wrong table of contents where the release picks it up.
  const final = path.join(os.tmpdir(), `informe-final-${process.pid}.pdf`)
  await printPdf(browser, pageHtml(meta, bodyHtml, first.pages), final)
  const second = await pageOfHeadings(final)
  for (const [id, pg] of first.pages) {
    if (second.pages.get(id) !== pg) {
      fs.rmSync(final, { force: true })
      throw new Error(`Pagination moved between passes at ${id}`)
    }
  }
  fs.copyFileSync(final, OUTPUT)
  fs.rmSync(final, { force: true })
  console.log(`informe-tecnico.pdf: ${second.total} pages, ${(fs.statSync(OUTPUT).size / 1024 / 1024).toFixed(2)} MB`)
  for (const h of headings) console.log(`${String(second.pages.get(h.id)).padStart(3)}  ${'  '.repeat(h.level - 1)}${h.num} ${h.title}`)
} finally {
  await browser.close()
}
