import { Call, Events } from "/wails/runtime.js";

const svc = (metodo, ...args) => Call.ByName(`main.CultoApp.${metodo}`, ...args);

const $ = (id) => document.getElementById(id);
const card = $("card");

let cfg = null;
let abaAtual = null;      // id do culto mostrado
let escolhaManual = 0;    // timestamp da última troca manual de aba

// ── Qual culto mostrar ──────────────────────────────────
// O culto do dia; senão o próximo da semana; senão o primeiro.
function cultoAutomatico() {
  const hoje = new Date().getDay();
  let melhor = null, menor = 99;
  for (const c of cfg.cultos) {
    if (c.dia < 0) continue;
    const dist = (c.dia - hoje + 7) % 7;
    if (dist < menor) { menor = dist; melhor = c; }
  }
  return (melhor ?? cfg.cultos[0])?.id ?? null;
}

function escolherAba() {
  const existe = cfg.cultos.some((c) => c.id === abaAtual);
  const manualRecente = Date.now() - escolhaManual < 30 * 60 * 1000;
  if (!existe || !manualRecente) abaAtual = cultoAutomatico();
}

// ── Render ──────────────────────────────────────────────
function el(tag, cls, texto) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (texto != null) e.textContent = texto;
  return e;
}

// ── Modo claro/escuro ───────────────────────────────────
const temaEscuroSistema = matchMedia("(prefers-color-scheme: dark)");
temaEscuroSistema.addEventListener("change", () => cfg && aplicarCores());

function aplicarCores() {
  const modo = cfg.modo === "auto" ? (temaEscuroSistema.matches ? "escuro" : "claro") : cfg.modo;
  const p = modo === "escuro" ? cfg.escuro : cfg.claro;
  const raiz = document.documentElement;
  raiz.dataset.modo = modo;
  raiz.style.setProperty("--fundo", p.fundo);
  raiz.style.setProperty("--texto", p.texto);
  raiz.style.setProperty("--acento", p.destaque);
}

function render() {
  aplicarCores();
  document.documentElement.style.setProperty("--op", cfg.opacidade);
  document.documentElement.style.setProperty("--escala", cfg.escala);
  $("igreja").textContent = cfg.igreja;
  document.body.classList.toggle("duas-colunas", cfg.colunas === 2);

  escolherAba();
  const culto = cfg.cultos.find((c) => c.id === abaAtual);

  // abas
  const abas = $("abas");
  abas.replaceChildren();
  abas.hidden = cfg.cultos.length < 2;
  for (const c of cfg.cultos) {
    const b = el("button", null, c.aba || c.titulo || "Culto");
    b.setAttribute("aria-selected", String(c.id === abaAtual));
    b.addEventListener("click", () => {
      abaAtual = c.id;
      escolhaManual = Date.now();
      render();
    });
    abas.append(b);
  }

  // título
  $("titulo").textContent = culto ? (culto.titulo || culto.aba) : "CultoScript7";
  $("hoje").hidden = !culto || culto.dia !== new Date().getDay();

  // seções
  const conteudo = $("conteudo");
  conteudo.replaceChildren();
  if (!culto) {
    conteudo.append(el("p", "vazio", "Nenhum culto cadastrado. Use “Editar programação” no ícone da bandeja."));
  } else if (!culto.secoes.length) {
    conteudo.append(el("p", "vazio", "Sem etapas ainda."));
  } else {
    for (const s of culto.secoes) {
      const sec = el("section", "secao");
      if (s.titulo) sec.append(el("h2", null, s.titulo));
      const ol = el("ol");
      s.itens.forEach((item, i) => {
        const li = el("li");
        li.append(el("span", "n", String(i + 1).padStart(2, "0")), el("span", "t", item));
        ol.append(li);
      });
      sec.append(ol);
      conteudo.append(sec);
    }
  }
  medir();
}

// ── Ajuste de altura da janela ao conteúdo ──────────────
let ultimaAltura = 0;
function medir() {
  requestAnimationFrame(() => {
    const conteudo = $("conteudo");
    const pad = parseFloat(getComputedStyle(document.body).paddingTop) || 0;
    const natural = card.offsetHeight - conteudo.clientHeight + conteudo.scrollHeight;
    const h = Math.ceil(natural + pad * 2);
    if (h !== ultimaAltura) {
      ultimaAltura = h;
      svc("AjustarAltura", h);
    }
  });
}
new ResizeObserver(medir).observe(card);

// ── Eventos do Go ───────────────────────────────────────
const dados = (ev) => (Array.isArray(ev?.data) ? ev.data[0] : ev?.data);

Events.On("config", (ev) => { cfg = dados(ev); render(); });
Events.On("edicao", (ev) => document.body.classList.toggle("editando", !!dados(ev)));

// Reavalia a aba automaticamente (virada do dia, etc.).
setInterval(() => cfg && render(), 5 * 60 * 1000);

async function iniciar() {
  await document.fonts.ready;
  cfg = await svc("Config");
  document.body.classList.toggle("editando", await svc("ModoEdicao"));
  render();
}
iniciar();
