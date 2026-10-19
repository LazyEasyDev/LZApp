package captcha

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"
	"sync"
)

const circleCount = 20

type canvas struct {
	*image.Paletted
	dotSize int
	scale   float64
}

type pngBufferPool struct {
	sync.Pool
}

func (pool *pngBufferPool) Get() *png.EncoderBuffer {
	buffer, _ := pool.Pool.Get().(*png.EncoderBuffer)
	return buffer
}

func (pool *pngBufferPool) Put(buffer *png.EncoderBuffer) {
	pool.Pool.Put(buffer)
}

var encodingBuffers pngBufferPool

type digitLayout struct {
	width         int
	height        int
	left          float64
	top           int
	step          float64
	skewLimit     float64
	waveAmplitude float64
}

func layoutDigits(width, height int) digitLayout {
	span := 0.90 * float64(width)
	fontScale := min(0.80*float64(height)/fontHeight, span/float64(Length*fontWidth+(Length-1)*3))
	digitHeight := max(1, int(fontScale*fontHeight))
	digitWidth := max(1, int(math.Ceil(float64(digitHeight)*fontWidth/fontHeight)))
	return digitLayout{
		width:         digitWidth,
		height:        digitHeight,
		left:          (float64(width) - span) / 2,
		top:           (height - digitHeight) / 2,
		step:          (span - float64(digitWidth)) / (Length - 1),
		skewLimit:     0.06 * float64(digitWidth),
		waveAmplitude: 0.03 * float64(digitHeight),
	}
}

func renderPNG(digits [Length]byte, width, height int) ([]byte, error) {
	palette := make(color.Palette, circleCount+1)
	palette[0] = color.Transparent
	primary := color.RGBA{R: uint8(rand.IntN(129)), G: uint8(rand.IntN(129)), B: uint8(rand.IntN(129)), A: 255}
	palette[1] = primary
	for index := 2; index < len(palette); index++ {
		shift := rand.IntN(255-int(max(primary.R, primary.G, primary.B))) - int(min(primary.R, primary.G, primary.B))
		palette[index] = color.RGBA{
			R: uint8(int(primary.R) + shift),
			G: uint8(int(primary.G) + shift),
			B: uint8(int(primary.B) + shift),
			A: 255,
		}
	}
	layout := layoutDigits(width, height)
	picture := &canvas{
		Paletted: image.NewPaletted(image.Rect(0, 0, width, height), palette),
		dotSize:  max(1, layout.height/fontHeight),
		scale:    float64(layout.height) / (2 * fontHeight),
	}
	for index, digit := range digits {
		picture.drawDigit(digitFont[digit], layout.left+float64(index)*layout.step, layout)
	}
	picture.strikeThrough()
	picture.distort(randomFloat(0.5, 1)*layout.waveAmplitude, randomFloat(0.6, 1.2)*float64(layout.height))
	for range circleCount {
		radius := randomInt(1, picture.dotSize+1)
		picture.drawCircle(randomInt(radius, width-radius), randomInt(radius, height-radius), radius, uint8(randomInt(1, circleCount)))
	}
	var output bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed, BufferPool: &encodingBuffers}
	if err := encoder.Encode(&output, picture.Paletted); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func randomInt(minimum, maximum int) int {
	if maximum <= minimum {
		return minimum
	}
	return minimum + rand.IntN(maximum-minimum)
}

func randomFloat(minimum, maximum float64) float64 {
	return minimum + rand.Float64()*(maximum-minimum)
}

func (picture *canvas) drawHorizontal(fromX, toX, posY int, colorIndex uint8) {
	for posX := fromX; posX <= toX; posX++ {
		picture.SetColorIndex(posX, posY, colorIndex)
	}
}

func (picture *canvas) drawCircle(posX, posY, radius int, colorIndex uint8) {
	errorTerm := 1 - radius
	deltaX := 1
	deltaY := -2 * radius
	offsetX := 0
	offsetY := radius
	picture.SetColorIndex(posX, posY+radius, colorIndex)
	picture.SetColorIndex(posX, posY-radius, colorIndex)
	picture.drawHorizontal(posX-radius, posX+radius, posY, colorIndex)
	for offsetX < offsetY {
		if errorTerm >= 0 {
			offsetY--
			deltaY += 2
			errorTerm += deltaY
		}
		offsetX++
		deltaX += 2
		errorTerm += deltaX
		picture.drawHorizontal(posX-offsetX, posX+offsetX, posY+offsetY, colorIndex)
		picture.drawHorizontal(posX-offsetX, posX+offsetX, posY-offsetY, colorIndex)
		picture.drawHorizontal(posX-offsetY, posX+offsetY, posY+offsetX, colorIndex)
		picture.drawHorizontal(posX-offsetY, posX+offsetY, posY-offsetX, colorIndex)
	}
}

func (picture *canvas) drawDigit(rows [fontHeight]string, posX float64, layout digitLayout) {
	skew := randomFloat(-layout.skewLimit, layout.skewLimit)
	for rowIndex := range layout.height {
		row := rows[rowIndex*fontHeight/layout.height]
		rowOffset := skew * (2*(float64(rowIndex)+0.5)/float64(layout.height) - 1)
		left := int(math.Round(posX + rowOffset))
		for column := range layout.width {
			if row[column*fontWidth/layout.width] == '1' {
				picture.SetColorIndex(left+column, layout.top+rowIndex, 1)
			}
		}
	}
}

func (picture *canvas) strikeThrough() {
	width, height := picture.Bounds().Dx(), picture.Bounds().Dy()
	posY := randomInt(height/3, height-height/3)
	amplitude := randomFloat(5, 20) * picture.scale
	frequency := 2 * math.Pi / (randomFloat(80, 180) * picture.scale)
	for posX := range width {
		offsetX := int(amplitude * math.Cos(float64(posY)*frequency))
		offsetY := int(amplitude * math.Sin(float64(posX)*frequency))
		for row := range picture.dotSize {
			picture.drawCircle(posX+offsetX, posY+offsetY+row, randomInt(0, picture.dotSize)/2, 1)
		}
	}
}

func (picture *canvas) distort(amplitude, period float64) {
	source := picture.Paletted
	destination := image.NewPaletted(source.Bounds(), source.Palette)
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	frequency := 2 * math.Pi / period
	horizontalOffsets := make([]int, height)
	for posY := range height {
		horizontalOffsets[posY] = int(amplitude * math.Sin(float64(posY)*frequency))
	}
	for posX := range width {
		verticalOffset := int(amplitude * math.Cos(float64(posX)*frequency))
		for posY := range height {
			destination.SetColorIndex(posX, posY, source.ColorIndexAt(posX+horizontalOffsets[posY], posY+verticalOffset))
		}
	}
	picture.Paletted = destination
}
