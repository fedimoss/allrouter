package registrationcaptcha

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand/v2"

	"golang.org/x/image/vector"
)

const (
	imageWidth  = 360
	imageHeight = 280
	imageScale  = 2
	iconCount   = 16
)

type point struct{ x, y float64 }

// These are geometric silhouettes rather than characters from a font. Each
// rendition gets independent rotation, scale, shear, color and texture. This
// avoids text-OCR and DOM/SVG answer extraction, not general computer vision.
func glyphs() [][]point {
	star := make([]point, 10)
	for i := range star {
		radius := 1.0
		if i%2 == 1 {
			radius = .44
		}
		angle := float64(i)*math.Pi/5 - math.Pi/2
		star[i] = point{radius * math.Cos(angle), radius * math.Sin(angle)}
	}
	heart := make([]point, 48)
	for i := range heart {
		angle := float64(i) * 2 * math.Pi / float64(len(heart))
		heart[i] = point{
			math.Pow(math.Sin(angle), 3),
			-(13*math.Cos(angle) - 5*math.Cos(2*angle) - 2*math.Cos(3*angle) - math.Cos(4*angle)) / 17,
		}
	}
	moon := make([]point, 0, 50)
	for i := 0; i <= 24; i++ {
		angle := -math.Pi/2 - float64(i)*math.Pi/24
		moon = append(moon, point{.9 * math.Cos(angle), .9 * math.Sin(angle)})
	}
	for i := 0; i <= 24; i++ {
		angle := math.Pi/2 + float64(i)*math.Pi/24
		moon = append(moon, point{.45 + .55*math.Cos(angle), .9 * math.Sin(angle)})
	}
	leaf := make([]point, 0, 42)
	for i := 0; i <= 20; i++ {
		y := -1 + float64(i)/10
		leaf = append(leaf, point{.65 * math.Sin(float64(i)*math.Pi/20), y})
	}
	for i := 20; i >= 0; i-- {
		y := -1 + float64(i)/10
		leaf = append(leaf, point{-.65 * math.Sin(float64(i)*math.Pi/20), y})
	}
	umbrella := []point{{-.95, 0}}
	for i := 0; i <= 24; i++ {
		angle := math.Pi + float64(i)*math.Pi/24
		umbrella = append(umbrella, point{.95 * math.Cos(angle), .8 * math.Sin(angle)})
	}
	umbrella = append(umbrella, []point{{.15, 0}, {.15, .72}, {-.05, .95}, {-.4, .95}, {-.6, .7}, {-.38, .55}, {-.26, .7}, {-.1, .7}, {-.1, 0}}...)
	cloud := []point{{-.85, .5}, {-.98, .25}, {-.96, -.08}, {-.7, -.33}, {-.47, -.3}, {-.3, -.73}, {.02, -.86}, {.4, -.7}, {.5, -.35}, {.78, -.36}, {.98, -.07}, {.98, .25}, {.78, .5}}
	return [][]point{
		star,
		heart,
		{{.18, -1}, {-.8, .2}, {-.14, .2}, {-.33, 1}, {.85, -.28}, {.2, -.28}},                                                       // lightning
		{{-1, -.65}, {-.48, -.24}, {0, -.98}, {.48, -.24}, {1, -.65}, {.73, .72}, {-.73, .72}},                                       // crown
		{{-1, -.28}, {.12, -.28}, {.12, -.82}, {1, 0}, {.12, .82}, {.12, .28}, {-1, .28}},                                            // arrow
		{{-.3, -1}, {.3, -1}, {.3, -.3}, {1, -.3}, {1, .3}, {.3, .3}, {.3, 1}, {-.3, 1}, {-.3, .3}, {-1, .3}, {-1, -.3}, {-.3, -.3}}, // cross
		moon,
		{{-.75, -1}, {.75, -1}, {.62, -.55}, {.12, 0}, {.62, .55}, {.75, 1}, {-.75, 1}, {-.62, .55}, {-.12, 0}, {-.62, -.55}}, // hourglass
		leaf,
		{{-1, -.72}, {-.43, -.25}, {.05, -.62}, {.55, -.48}, {1, 0}, {.55, .48}, {.05, .62}, {-.43, .25}, {-1, .72}}, // fish
		umbrella,
		{{-.25, -.88}, {-.25, -.65}, {-.6, -.42}, {-.67, .35}, {-.95, .67}, {.95, .67}, {.67, .35}, {.6, -.42}, {.25, -.65}, {.25, -.88}},                                           // bell
		{{-.9, -.45}, {-.55, -.7}, {-.18, -.53}, {-.02, -.18}, {1, -.18}, {1, .1}, {.75, .1}, {.75, .45}, {.48, .45}, {.48, .1}, {-.02, .1}, {-.25, .46}, {-.65, .46}, {-.92, .12}}, // key
		{{0, -1}, {.52, -.85}, {.77, -.42}, {.7, .08}, {0, 1}, {-.7, .08}, {-.77, -.42}, {-.52, -.85}},                                                                              // location pin
		cloud,
		{{-.85, -.7}, {.55, -.7}, {.55, -.45}, {.95, -.45}, {1, -.05}, {.8, .34}, {.5, .34}, {.32, .78}, {-.55, .78}},                                                                                                                                              // cup
		{{0, -.32}, {-.45, -.93}, {-.94, -.68}, {-.82, -.07}, {-.38, .15}, {-.85, .5}, {-.54, .9}, {0, .42}, {.54, .9}, {.85, .5}, {.38, .15}, {.82, -.07}, {.94, -.68}, {.45, -.93}},                                                                              // butterfly
		{{-.05, -1}, {.78, -.72}, {.78, -.23}, {.17, -.45}, {.17, .65}, {-.05, .9}, {-.57, .9}, {-.78, .58}, {-.62, .28}, {-.28, .22}, {-.05, .28}},                                                                                                                // musical note
		{{-.18, -1}, {.18, -1}, {.18, -.35}, {.6, -.35}, {.6, -.1}, {.18, -.1}, {.18, .58}, {.62, .37}, {.77, .03}, {1, .22}, {.8, .72}, {0, 1}, {-.8, .72}, {-1, .22}, {-.77, .03}, {-.62, .37}, {-.18, .58}, {-.18, -.1}, {-.6, -.1}, {-.6, -.35}, {-.18, -.35}}, // anchor
		{{-.8, -.88}, {0, -1}, {.8, -.88}, {.77, .2}, {.5, .64}, {0, 1}, {-.5, .64}, {-.77, .2}},                                                                                                                                                                   // shield
	}
}

func fillPolygon(dst *image.RGBA, points []point, x, y, radius, rotation, shear float64, fill color.Color) {
	raster := vector.NewRasterizer(dst.Bounds().Dx(), dst.Bounds().Dy())
	for i, p := range points {
		px := p.x + p.y*shear
		py := p.y
		tx := x + radius*(px*math.Cos(rotation)-py*math.Sin(rotation))
		ty := y + radius*(px*math.Sin(rotation)+py*math.Cos(rotation))
		if i == 0 {
			raster.MoveTo(float32(tx), float32(ty))
		} else {
			raster.LineTo(float32(tx), float32(ty))
		}
	}
	raster.ClosePath()
	raster.Draw(dst, dst.Bounds(), image.NewUniform(fill), image.Point{})
}

func drawGlyph(dst *image.RGBA, glyph []point, x, y, radius float64, random *rand.Rand) {
	rotation := (random.Float64() - .5) * .8
	shear := (random.Float64() - .5) * .14
	scale := .92 + random.Float64()*.14
	canvasSize := int(math.Ceil((radius*1.3 + 4) * 2 * imageScale))
	canvas := image.NewRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	center := float64(canvasSize) / 2
	ink := color.RGBA{uint8(45 + random.IntN(45)), uint8(65 + random.IntN(55)), uint8(100 + random.IntN(60)), 255}
	fillPolygon(canvas, glyph, center, center, radius*scale*imageScale, rotation, shear, ink)
	// The inset adds an independently colored edge without marking targets.
	fillPolygon(canvas, glyph, center, center, (radius*scale-1.1)*imageScale, rotation, shear,
		color.RGBA{ink.R + 40, ink.G + 36, ink.B + 30, 255})
	left := int(x*imageScale) - canvasSize/2
	top := int(y*imageScale) - canvasSize/2
	draw.Draw(dst, image.Rect(left, top, left+canvasSize, top+canvasSize), canvas, image.Point{}, draw.Over)
}

func addTexture(dst *image.RGBA, random *rand.Rand, dots int) {
	for i := 0; i < dots; i++ {
		x := random.IntN(dst.Bounds().Dx())
		y := random.IntN(dst.Bounds().Dy())
		shade := uint8(185 + random.IntN(50))
		for dx := 0; dx < imageScale; dx++ {
			for dy := 0; dy < imageScale; dy++ {
				dst.SetRGBA(x+dx, y+dy, color.RGBA{shade, shade, shade + 10, 255})
			}
		}
	}
	for i := 0; i < 7; i++ {
		x1 := random.Float64() * float64(dst.Bounds().Dx())
		y1 := random.Float64() * float64(dst.Bounds().Dy())
		x2 := random.Float64() * float64(dst.Bounds().Dx())
		y2 := random.Float64() * float64(dst.Bounds().Dy())
		for step := 0; step < 300; step++ {
			t := float64(step) / 300
			dst.SetRGBA(int(x1+(x2-x1)*t), int(y1+(y2-y1)*t), color.RGBA{210, 219, 232, 255})
		}
	}
}

func encodeImage(img image.Image) (string, error) {
	var buffer bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buffer, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

func generateImages() (*Challenge, []target, error) {
	var seed [32]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return nil, nil, err
	}
	random := rand.New(rand.NewChaCha8(seed))
	shapes := glyphs()
	selected := random.Perm(len(shapes))[:iconCount]
	ordered := random.Perm(iconCount)[:RequiredClicks]
	board := image.NewRGBA(image.Rect(0, 0, imageWidth*imageScale, imageHeight*imageScale))
	prompt := image.NewRGBA(image.Rect(0, 0, imageWidth*imageScale, 64*imageScale))
	background := image.NewUniform(color.RGBA{246, 248, 252, 255})
	draw.Draw(board, board.Bounds(), background, image.Point{}, draw.Src)
	draw.Draw(prompt, prompt.Bounds(), background, image.Point{}, draw.Src)
	addTexture(board, random, 700)
	addTexture(prompt, random, 120)
	positions := make([]target, iconCount)
	for i, shapeID := range selected {
		x := 45 + float64(i%4)*90 + (random.Float64()-.5)*18
		y := 35 + float64(i/4)*70 + (random.Float64()-.5)*12
		positions[i] = target{X: x, Y: y, Radius: 26}
		drawGlyph(board, shapes[shapeID], x, y, 22, random)
	}
	targets := make([]target, RequiredClicks)
	for i, index := range ordered {
		targets[i] = positions[index]
		x := (float64(i) + .5) * imageWidth / RequiredClicks
		drawGlyph(prompt, shapes[selected[index]], x, 32, 20, random)
	}
	encodedBoard, err := encodeImage(board)
	if err != nil {
		return nil, nil, err
	}
	encodedPrompt, err := encodeImage(prompt)
	if err != nil {
		return nil, nil, err
	}
	return &Challenge{
		Image:          encodedBoard,
		PromptImage:    encodedPrompt,
		Width:          imageWidth,
		Height:         imageHeight,
		RequiredClicks: RequiredClicks,
		ExpiresIn:      int(ChallengeTTL.Seconds()),
	}, targets, nil
}
