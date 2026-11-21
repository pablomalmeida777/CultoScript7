// icongen gera os ícones do CultoScript desenhando um caractere ("7") a partir
// de um arquivo de fonte TTF/OTF.
//
//	go run ./tools/icongen -font assets-src/fonte-icone.ttf -text 7
//
// Saídas:
//
//	build/appicon.png        1024px, fundo azul arredondado + caractere branco (base do .ico)
//	icons/tray-escuro.png    caractere branco, para barra de tarefas escura
//	icons/tray-claro.png     caractere preto, para barra de tarefas clara
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

var azul = color.RGBA{0x1D, 0x5F, 0xD6, 0xFF}

func main() {
	fontPath := flag.String("font", "assets-src/fonte-icone.ttf", "arquivo TTF/OTF")
	text := flag.String("text", "7", "caractere a desenhar")
	appFill := flag.Float64("app", 0.62, "altura do caractere em relação ao ícone do app")
	trayFill := flag.Float64("tray", 0.92, "altura do caractere em relação ao ícone da bandeja")
	opticoX := flag.Float64("opticox", 1.0, "compensação óptica horizontal do ícone do app (0 = caixa, 1 = centro de massa)")
	opticoY := flag.Float64("opticoy", 0.25, "compensação óptica vertical do ícone do app (0 = caixa, 1 = centro de massa)")
	saida := flag.String("saida", ".", "pasta base de saída")
	flag.Parse()
	must(os.Chdir(*saida))

	data, err := os.ReadFile(*fontPath)
	must(err)
	f, err := sfnt.Parse(data)
	must(err)

	r := []rune(*text)[0]
	var buf sfnt.Buffer
	gi, err := f.GlyphIndex(&buf, r)
	must(err)
	if gi == 0 {
		log.Fatalf("a fonte não tem o caractere %q", r)
	}
	const ppem = 1000
	segs, err := f.LoadGlyph(&buf, gi, fixed.I(ppem), nil)
	must(err)

	// App: quadrado azul arredondado + caractere branco.
	const N = 1024
	app := image.NewRGBA(image.Rect(0, 0, N, N))
	bg := vector.NewRasterizer(N, N)
	roundedRect(bg, 0, 0, N, N, N*0.225)
	bg.Draw(app, app.Bounds(), image.NewUniform(azul), image.Point{})
	dx, dy := drawGlyph(app, segs, float64(N)**appFill, color.White, *opticoX, *opticoY)
	must(save(app, "build/appicon.png"))
	// Deslocamento em fração do lado do ícone, para replicar nos selos em CSS.
	fmt.Printf("deslocamento óptico: x %+.4f, y %+.4f (fração do lado)\n", dx/N, dy/N)

	// Bandeja: só o caractere (sem fundo), centralizado pela caixa.
	for name, c := range map[string]color.Color{
		"icons/tray-escuro.png": color.White,
		"icons/tray-claro.png":  color.RGBA{0x1B, 0x1B, 0x1B, 0xFF},
	} {
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		drawGlyph(img, segs, 64**trayFill, c, 0, 0)
		must(save(img, name))
	}
	fmt.Printf("ícones gerados: %q com %s\n", *text, filepath.Base(*fontPath))
}

// Centralização óptica: o glifo é primeiro centralizado pela caixa e depois
// deslocado em direção ao seu centro de massa (tinta), na fração indicada.
// O "7" tem muito peso na barra de cima e à esquerda; centralizar só pela
// caixa o faz parecer deslocado.
// Devolve o deslocamento aplicado, em pixels.
func drawGlyph(dst *image.RGBA, segs sfnt.Segments, height float64, c color.Color, opticoX, opticoY float64) (float64, float64) {
	W, H := dst.Bounds().Dx(), dst.Bounds().Dy()
	// 1ª passada: máscara centralizada pela caixa, para medir a tinta.
	mask := image.NewAlpha(image.Rect(0, 0, W, H))
	rasterGlyph(mask, segs, height, 0, 0, color.Opaque)
	var sx, sy, total float64
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			a := float64(mask.AlphaAt(x, y).A)
			sx += a * (float64(x) + 0.5)
			sy += a * (float64(y) + 0.5)
			total += a
		}
	}
	dx, dy := 0.0, 0.0
	if total > 0 {
		dx = (float64(W)/2 - sx/total) * opticoX
		dy = (float64(H)/2 - sy/total) * opticoY
	}
	rasterGlyph(dst, segs, height, dx, dy, c)
	return dx, dy
}

// rasterGlyph centraliza o contorno pela caixa delimitadora, escala para a
// altura pedida e aplica o deslocamento (dx, dy).
func rasterGlyph(dst draw.Image, segs sfnt.Segments, height, dx, dy float64, c color.Color) {
	b := segs.Bounds()
	minX, minY := fx(b.Min.X), fx(b.Min.Y)
	gw, gh := fx(b.Max.X)-minX, fx(b.Max.Y)-minY
	s := height / gh
	W, H := float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy())
	if gw*s > W*0.92 { // caracteres largos não podem vazar
		s = W * 0.92 / gw
	}
	ox := (W-gw*s)/2 - minX*s + dx
	oy := (H-gh*s)/2 - minY*s + dy
	p := func(v fixed.Point26_6) (float32, float32) {
		return float32(fx(v.X)*s + ox), float32(fx(v.Y)*s + oy)
	}

	z := vector.NewRasterizer(int(W), int(H))
	for _, sg := range segs {
		switch sg.Op {
		case sfnt.SegmentOpMoveTo:
			x, y := p(sg.Args[0])
			z.MoveTo(x, y)
		case sfnt.SegmentOpLineTo:
			x, y := p(sg.Args[0])
			z.LineTo(x, y)
		case sfnt.SegmentOpQuadTo:
			x1, y1 := p(sg.Args[0])
			x, y := p(sg.Args[1])
			z.QuadTo(x1, y1, x, y)
		case sfnt.SegmentOpCubeTo:
			x1, y1 := p(sg.Args[0])
			x2, y2 := p(sg.Args[1])
			x, y := p(sg.Args[2])
			z.CubeTo(x1, y1, x2, y2, x, y)
		}
	}
	z.ClosePath()
	z.DrawOp = draw.Over
	z.Draw(dst, dst.Bounds(), image.NewUniform(c), image.Point{})
}

func roundedRect(z *vector.Rasterizer, x, y, w, h, r float32) {
	const k = 0.5523 // aproximação de arco por Bézier cúbica
	z.MoveTo(x+r, y)
	z.LineTo(x+w-r, y)
	z.CubeTo(x+w-r+r*k, y, x+w, y+r-r*k, x+w, y+r)
	z.LineTo(x+w, y+h-r)
	z.CubeTo(x+w, y+h-r+r*k, x+w-r+r*k, y+h, x+w-r, y+h)
	z.LineTo(x+r, y+h)
	z.CubeTo(x+r-r*k, y+h, x, y+h-r+r*k, x, y+h-r)
	z.LineTo(x, y+r)
	z.CubeTo(x, y+r-r*k, x+r-r*k, y, x+r, y)
	z.ClosePath()
}

func fx(v fixed.Int26_6) float64 { return float64(v) / 64 }

func save(img image.Image, out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
