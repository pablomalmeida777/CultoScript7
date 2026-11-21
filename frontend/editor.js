import { Call } from "/wails/runtime.js";

const svc = (metodo, ...args) => Call.ByName(`main.CultoApp.${metodo}`, ...args);
const $ = (id) => document.getElementById(id);

const DIAS = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];
const MODELOS = {
  domingo: () => ({ aba: "Domingo", titulo: "Culto de domingo", dia: 0, secoes: [{ titulo: "Culto", itens: [] }] }),
  quarta: () => ({ aba: "Quarta", titulo: "Culto de quarta-feira", dia: 3, secoes: [{ titulo: "Culto de oração", itens: [] }] }),
  branco: () => ({ aba: "Novo culto", titulo: "Novo culto", dia: -1, secoes: [{ titulo: "", itens: [] }] }),
};

const PRONTAS = {
  claro: [
    ["Branco", "#F7F8FC", "#18203A", "#1D5FD6"],
    ["Areia", "#F6F0E4", "#2B2418", "#9A6B1F"],
    ["Céu", "#E6F0FD", "#14233F", "#1D5FD6"],
    ["Sálvia", "#EEF3EE", "#1C2A22", "#2F7A4F"],
  ],
  escuro: [
    ["Azul noite", "#0B1630", "#EEF2FB", "#DCBB72"],
    ["Grafite", "#1E1F23", "#F3F4F6", "#86AAFF"],
    ["Vinho", "#2A0F17", "#F7EDEF", "#E7B07A"],
    ["Floresta", "#0F1F1A", "#ECF3EF", "#9BD3A8"],
  ],
};
const CAMPOS_COR = [["fundo", "Fundo"], ["texto", "Texto"], ["destaque", "Destaque (seções e abas)"]];
const temaEscuroSistema = matchMedia("(prefers-color-scheme: dark)");

let cfg = null;
let pagina = { tipo: "culto", id: null }; // ou { tipo: "aparencia" }

// ── Salvamento automático ───────────────────────────────
let timer = null;
let pendente = false;

async function enviar() {
  const status = $("status");
  pendente = false;
  try {
    const limpo = structuredClone(cfg);
    for (const c of limpo.cultos) {
      for (const s of c.secoes) s.itens = s.itens.map((t) => t.trim()).filter(Boolean);
    }
    await svc("Salvar", limpo);
    status.className = "status";
    status.textContent = "Salvo";
  } catch (e) {
    status.className = "status erro";
    status.textContent = "Erro ao salvar";
    console.error(e);
  }
}

function salvar(imediato = false) {
  const status = $("status");
  status.className = "status salvando";
  status.textContent = "Salvando…";
  pendente = true;
  clearTimeout(timer);
  timer = setTimeout(enviar, imediato ? 0 : 350);
}

// Grava já o que estiver pendente (antes de exportar/importar).
async function salvarAgora() {
  clearTimeout(timer);
  if (pendente) await enviar();
}

// ── Lateral ─────────────────────────────────────────────
function renderLateral() {
  $("igreja-lateral").textContent = cfg.igreja;
  const lista = $("lista-cultos");
  lista.replaceChildren();
  for (const c of cfg.cultos) {
    const b = document.createElement("button");
    b.className = "item-lateral" + (pagina.tipo === "culto" && pagina.id === c.id ? " ativo" : "");
    b.append(c.aba || "Sem nome");
    if (c.dia >= 0) {
      const s = document.createElement("small");
      s.textContent = DIAS[c.dia];
      b.append(s);
    }
    b.addEventListener("click", () => irPara({ tipo: "culto", id: c.id }));
    lista.append(b);
  }
  $("nav-aparencia").classList.toggle("ativo", pagina.tipo === "aparencia");
}

function irPara(p) {
  pagina = p;
  $("confirmar-exclusao").hidden = true;
  $("btn-excluir-culto").hidden = false;
  render();
  document.querySelector(".principal").scrollTop = 0;
}

// ── Página do culto ─────────────────────────────────────
const cultoAtual = () => cfg.cultos.find((c) => c.id === pagina.id);

function renderCulto() {
  const c = cultoAtual();
  $("cabecalho").textContent = c.titulo || c.aba || "Culto";
  $("f-aba").value = c.aba;
  $("f-titulo").value = c.titulo;

  const dias = $("f-dia");
  dias.replaceChildren();
  [["Nenhum", -1], ...DIAS.map((d, i) => [d, i])].forEach(([rotulo, valor]) => {
    const b = document.createElement("button");
    b.textContent = rotulo;
    b.setAttribute("aria-pressed", String(c.dia === valor));
    b.addEventListener("click", () => { c.dia = valor; salvar(); renderCulto(); renderLateral(); });
    dias.append(b);
  });

  renderSecoes();
}

function renderSecoes(foco) {
  const c = cultoAtual();
  const box = $("secoes");
  box.replaceChildren();

  c.secoes.forEach((s, si) => {
    const sec = $("tpl-secao").content.firstElementChild.cloneNode(true);
    const titulo = sec.querySelector(".secao-titulo");
    titulo.value = s.titulo;
    titulo.addEventListener("input", () => { s.titulo = titulo.value; salvar(); });

    const ferr = sec.querySelector(".secao-topo .ferramentas");
    ferr.querySelector('[data-acao="sobe"]').disabled = si === 0;
    ferr.querySelector('[data-acao="desce"]').disabled = si === c.secoes.length - 1;
    ferr.addEventListener("click", (e) => {
      const acao = e.target.closest("button")?.dataset.acao;
      if (!acao) return;
      if (acao === "remove") {
        if (s.itens.length && !sec.dataset.confirmar) {
          sec.dataset.confirmar = "1";
          e.target.closest("button").title = "Clique de novo para excluir a seção";
          e.target.closest("button").style.color = "var(--vermelho)";
          setTimeout(() => { delete sec.dataset.confirmar; }, 2500);
          return;
        }
        c.secoes.splice(si, 1);
      } else {
        mover(c.secoes, si, acao === "sobe" ? -1 : 1);
      }
      salvar();
      renderSecoes();
    });

    const ol = sec.querySelector(".itens");
    s.itens.forEach((texto, ii) => ol.append(criarItem(s, si, ii, texto)));

    sec.querySelector(".btn-adicionar").addEventListener("click", () => {
      s.itens.push("");
      renderSecoes({ si, ii: s.itens.length - 1 });
    });
    box.append(sec);
  });

  if (foco) {
    const sec = box.children[foco.si];
    const inp = sec?.querySelectorAll(".item-texto")[foco.ii];
    if (inp) { inp.focus(); inp.setSelectionRange(foco.pos ?? inp.value.length, foco.pos ?? inp.value.length); }
  }
}

function criarItem(s, si, ii, texto) {
  const li = $("tpl-item").content.firstElementChild.cloneNode(true);
  li.querySelector(".num").textContent = String(ii + 1).padStart(2, "0");
  const inp = li.querySelector(".item-texto");
  inp.value = texto;
  inp.addEventListener("input", () => { s.itens[ii] = inp.value; salvar(); });
  inp.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      s.itens.splice(ii + 1, 0, "");
      salvar();
      renderSecoes({ si, ii: ii + 1 });
    } else if (e.key === "Backspace" && inp.value === "" ) {
      e.preventDefault();
      s.itens.splice(ii, 1);
      salvar();
      renderSecoes({ si, ii: Math.max(0, ii - 1) });
    } else if (e.altKey && (e.key === "ArrowUp" || e.key === "ArrowDown")) {
      e.preventDefault();
      const d = e.key === "ArrowUp" ? -1 : 1;
      if (mover(s.itens, ii, d)) { salvar(); renderSecoes({ si, ii: ii + d }); }
    } else if (e.key === "ArrowUp" || e.key === "ArrowDown") {
      const lista = [...document.querySelectorAll(".item-texto")];
      const alvo = lista[lista.indexOf(inp) + (e.key === "ArrowUp" ? -1 : 1)];
      if (alvo) { e.preventDefault(); alvo.focus(); }
    }
  });

  const ferr = li.querySelector(".ferramentas");
  ferr.querySelector('[data-acao="sobe"]').disabled = ii === 0;
  ferr.querySelector('[data-acao="desce"]').disabled = ii === s.itens.length - 1;
  ferr.addEventListener("click", (e) => {
    const acao = e.target.closest("button")?.dataset.acao;
    if (!acao) return;
    if (acao === "remove") s.itens.splice(ii, 1);
    else mover(s.itens, ii, acao === "sobe" ? -1 : 1);
    salvar();
    renderSecoes();
  });
  return li;
}

function mover(arr, i, d) {
  const j = i + d;
  if (j < 0 || j >= arr.length) return false;
  [arr[i], arr[j]] = [arr[j], arr[i]];
  return true;
}

// ── Página de aparência ─────────────────────────────────
function renderAparencia() {
  $("cabecalho").textContent = "Configurações";
  $("a-igreja").value = cfg.igreja;
  for (const b of $("a-modo").children) b.setAttribute("aria-pressed", String(b.dataset.modo === cfg.modo));
  renderPaletas();
  for (const b of $("a-colunas").children) b.setAttribute("aria-pressed", String(Number(b.dataset.colunas) === cfg.colunas));
  $("a-largura").value = cfg.largura;
  $("a-escala").value = cfg.escala;
  $("a-opacidade").value = cfg.opacidade;
  atualizarSaidas();
}

function modoAtivo() {
  return cfg.modo === "auto" ? (temaEscuroSistema.matches ? "escuro" : "claro") : cfg.modo;
}

function renderPaletas() {
  const box = $("a-paletas");
  box.replaceChildren();
  for (const [modo, nome] of [["claro", "Modo claro"], ["escuro", "Modo escuro"]]) {
    const p = cfg[modo];
    const col = document.createElement("div");
    col.className = "paleta";

    const h3 = document.createElement("h3");
    h3.textContent = nome;
    if (modoAtivo() === modo) {
      const tag = document.createElement("small");
      tag.textContent = "em uso agora";
      h3.append(tag);
    }

    const previa = document.createElement("div");
    previa.className = `previa ${modo}`;
    previa.innerHTML = `<strong>Programação</strong><em>Culto Divino</em>
      <span><b>01</b>Louvor</span><span><b>02</b>Hino inicial</span><span style="border:0"><b>03</b>Pregação</span>`;
    const pintar = () => {
      previa.style.setProperty("--p-fundo", p.fundo);
      previa.style.setProperty("--p-texto", p.texto);
      previa.style.setProperty("--p-destaque", p.destaque);
    };
    pintar();

    const cores = document.createElement("div");
    cores.className = "cores";
    for (const [campo, rotulo] of CAMPOS_COR) {
      const linha = document.createElement("label");
      linha.className = "cor";
      const nomeEl = document.createElement("span");
      nomeEl.textContent = rotulo;
      nomeEl.style.cssText = "margin:0;font-weight:400;font-size:13px;color:var(--texto)";
      const inp = document.createElement("input");
      inp.type = "color";
      inp.value = p[campo].toLowerCase();
      const hex = document.createElement("code");
      hex.textContent = p[campo];
      inp.addEventListener("input", () => {
        p[campo] = inp.value.toUpperCase();
        hex.textContent = p[campo];
        pintar();
        salvar();
      });
      linha.append(nomeEl, inp, hex);
      cores.append(linha);
    }

    const prontas = document.createElement("div");
    prontas.className = "prontas";
    for (const [rotulo, fundo, texto, destaque] of PRONTAS[modo]) {
      const b = document.createElement("button");
      b.innerHTML = `<i style="--f:${fundo};--d:${destaque}"></i>`;
      b.append(rotulo);
      b.addEventListener("click", () => {
        Object.assign(p, { fundo, texto, destaque });
        salvar(true);
        renderPaletas();
      });
      prontas.append(b);
    }

    col.append(h3, previa, cores, prontas);
    box.append(col);
  }
}

function atualizarSaidas() {
  $("o-largura").textContent = `${cfg.largura} px`;
  $("o-escala").textContent = `${Math.round(cfg.escala * 100)}%`;
  $("o-opacidade").textContent = `${Math.round(cfg.opacidade * 100)}%`;
}

// ── Render geral ────────────────────────────────────────
function render() {
  if (pagina.tipo === "culto" && !cultoAtual()) {
    pagina = cfg.cultos.length ? { tipo: "culto", id: cfg.cultos[0].id } : { tipo: "aparencia" };
  }
  $("pagina-culto").hidden = pagina.tipo !== "culto";
  $("pagina-aparencia").hidden = pagina.tipo !== "aparencia";
  if (pagina.tipo === "culto") renderCulto(); else renderAparencia();
  renderLateral();
}

// ── Ligações fixas ──────────────────────────────────────
function ligar() {
  $("f-aba").addEventListener("input", (e) => { cultoAtual().aba = e.target.value; salvar(); renderLateral(); });
  $("f-titulo").addEventListener("input", (e) => {
    cultoAtual().titulo = e.target.value;
    $("cabecalho").textContent = e.target.value || cultoAtual().aba || "Culto";
    salvar();
  });

  $("btn-nova-secao").addEventListener("click", () => {
    const c = cultoAtual();
    c.secoes.push({ titulo: "", itens: [] });
    salvar();
    renderSecoes();
    $("secoes").lastElementChild?.querySelector(".secao-titulo").focus();
  });

  $("btn-excluir-culto").addEventListener("click", () => {
    $("btn-excluir-culto").hidden = true;
    $("confirmar-exclusao").hidden = false;
  });
  $("btn-cancelar-exclusao").addEventListener("click", () => {
    $("btn-excluir-culto").hidden = false;
    $("confirmar-exclusao").hidden = true;
  });
  $("btn-confirmar-exclusao").addEventListener("click", () => {
    cfg.cultos = cfg.cultos.filter((c) => c.id !== pagina.id);
    salvar(true);
    irPara({ tipo: "culto", id: cfg.cultos[0]?.id ?? null });
  });

  // novo culto
  const menu = $("menu-novo");
  $("btn-novo").addEventListener("click", (e) => { e.stopPropagation(); menu.hidden = !menu.hidden; });
  document.addEventListener("click", () => { menu.hidden = true; });
  menu.addEventListener("click", (e) => {
    const modelo = e.target.closest("button")?.dataset.modelo;
    if (!modelo) return;
    const novo = { id: Date.now().toString(36), ...MODELOS[modelo]() };
    cfg.cultos.push(novo);
    salvar(true);
    irPara({ tipo: "culto", id: novo.id });
    $("f-aba").select();
  });

  $("nav-aparencia").addEventListener("click", () => irPara({ tipo: "aparencia" }));

  // aparência
  $("a-igreja").addEventListener("input", (e) => { cfg.igreja = e.target.value; salvar(); renderLateral(); });
  $("a-modo").addEventListener("click", (e) => {
    const m = e.target.closest("button")?.dataset.modo;
    if (!m) return;
    cfg.modo = m;
    salvar(true);
    renderAparencia();
  });
  temaEscuroSistema.addEventListener("change", () => pagina.tipo === "aparencia" && renderPaletas());
  $("a-colunas").addEventListener("click", (e) => {
    const n = Number(e.target.closest("button")?.dataset.colunas);
    if (!n) return;
    cfg.colunas = n;
    // Lado a lado precisa de mais largura; volta a um valor confortável no modo simples.
    if (n === 2 && cfg.largura < 640) cfg.largura = 680;
    if (n === 1 && cfg.largura > 560) cfg.largura = 400;
    salvar(true);
    renderAparencia();
  });
  for (const [id, campo] of [["a-largura", "largura"], ["a-escala", "escala"], ["a-opacidade", "opacidade"]]) {
    $(id).addEventListener("input", (e) => {
      cfg[campo] = Number(e.target.value);
      atualizarSaidas();
      salvar();
    });
  }
  $("a-restaurar").addEventListener("click", () => svc("RestaurarPosicao"));
  $("a-restaurar-cores").addEventListener("click", () => {
    // Padrões = primeira paleta pronta de cada modo (as mesmas do config.go).
    for (const modo of ["claro", "escuro"]) {
      const [, fundo, texto, destaque] = PRONTAS[modo][0];
      Object.assign(cfg[modo], { fundo, texto, destaque });
    }
    salvar(true);
    renderPaletas();
  });
  $("a-restaurar-layout").addEventListener("click", () => {
    // Mesmos valores de configPadrao() em config.go.
    Object.assign(cfg, { colunas: 1, largura: 400, escala: 1, opacidade: 0.88 });
    salvar(true);
    renderAparencia();
  });
  // Exportar / importar
  const aviso = (texto, tipo = "") => {
    const a = $("aviso-backup");
    a.textContent = texto;
    a.className = `aviso ${tipo}`;
    a.hidden = !texto;
  };
  const confirmar = (mostrar) => {
    $("confirmar-importacao").hidden = !mostrar;
    $("b-importar").hidden = mostrar;
  };
  $("b-exportar").addEventListener("click", async () => {
    aviso("");
    try {
      await salvarAgora();
      const caminho = await svc("ExportarCultos");
      if (caminho) aviso(`Cultos exportados para ${caminho}`, "ok");
    } catch (e) {
      aviso(`Não foi possível exportar: ${e?.message ?? e}`, "erro");
    }
  });
  $("b-importar").addEventListener("click", () => { aviso(""); confirmar(true); });
  $("b-importar-cancelar").addEventListener("click", () => confirmar(false));
  $("b-importar-confirmar").addEventListener("click", async () => {
    try {
      await salvarAgora();
      const nova = await svc("ImportarCultos");
      if (!nova) return; // cancelou a escolha do arquivo
      cfg = nova;
      confirmar(false);
      const n = cfg.cultos.length;
      aviso(`${n} ${n === 1 ? "culto importado" : "cultos importados"}.`, "ok");
      renderLateral();
    } catch (e) {
      aviso(`Não foi possível importar: ${e?.message ?? e}`, "erro");
    }
  });

  $("link-projeto").addEventListener("click", (e) => { e.preventDefault(); svc("AbrirProjeto"); });
}

function aplicarTemaEditor() {
  const escuro = cfg.editorTema !== "claro";
  document.documentElement.dataset.tema = escuro ? "escuro" : "claro";
  const b = $("btn-tema");
  b.setAttribute("aria-checked", String(escuro));
  b.title = escuro ? "Modo escuro do editor (clique para claro)" : "Modo claro do editor (clique para escuro)";
}

async function iniciar() {
  cfg = await svc("Config");
  aplicarTemaEditor();
  svc("Versao").then((v) => { $("versao").textContent = v === "dev" ? "dev" : `v${v}`; });
  $("btn-tema").addEventListener("click", () => {
    cfg.editorTema = cfg.editorTema === "claro" ? "escuro" : "claro";
    // Transição suave entre os temas (cross-fade); instantânea se o
    // Windows estiver com "reduzir animações".
    const reduzir = matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (document.startViewTransition && !reduzir) document.startViewTransition(aplicarTemaEditor);
    else aplicarTemaEditor();
    salvar(true);
  });
  pagina = { tipo: "culto", id: cfg.cultos[0]?.id ?? null };
  ligar();
  render();
}
iniciar();
