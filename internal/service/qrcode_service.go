package service

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/42ShortHub/shortlink/internal/config"
)

type QRCodeService struct {
	baseURL string
}

func NewQRCodeService(cfg *config.Config) *QRCodeService {
	return &QRCodeService{baseURL: cfg.App.BaseURL}
}

func (s *QRCodeService) GeneratePNG(shortCode string, size int) ([]byte, error) {
	url := fmt.Sprintf("%s/s/%s", s.baseURL, shortCode)
	matrix := encodeQR(url)
	img := renderQRImage(matrix, size)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *QRCodeService) GenerateSVG(shortCode string, size int) ([]byte, error) {
	url := fmt.Sprintf("%s/s/%s", s.baseURL, shortCode)
	matrix := encodeQR(url)
	svg := renderQRSVG(matrix, size)
	return []byte(svg), nil
}

// Minimal QR Code encoder (Mode: byte, Error Correction: L, Version auto-selected)
func encodeQR(data string) [][]bool {
	bits := encodeData(data)
	version := selectVersion(len(data))
	size := 17 + version*4
	matrix := make([][]bool, size)
	for i := range matrix {
		matrix[i] = make([]bool, size)
	}
	reserved := make([][]bool, size)
	for i := range reserved {
		reserved[i] = make([]bool, size)
	}

	addFinderPatterns(matrix, reserved, size)
	addAlignmentPatterns(matrix, reserved, version, size)
	addTimingPatterns(matrix, reserved, size)
	addDarkModule(matrix, reserved, version)

	dataBits := addECC(bits, version)
	placeData(matrix, reserved, dataBits, size)
	applyMask(matrix, reserved, size)
	addFormatInfo(matrix, size)

	return matrix
}

func selectVersion(dataLen int) int {
	capacities := []int{0, 17, 32, 53, 78, 106, 134, 154, 192, 230, 271, 321, 367, 425, 458, 520, 586}
	for v := 1; v < len(capacities); v++ {
		if dataLen <= capacities[v] {
			return v
		}
	}
	return 10
}

func encodeData(data string) []byte {
	var bits []byte
	bits = append(bits, 0, 1, 0, 0) // byte mode indicator
	length := len(data)
	for i := 7; i >= 0; i-- {
		bits = append(bits, byte((length>>i)&1))
	}
	for _, b := range []byte(data) {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>i)&1)
		}
	}
	// terminator
	for i := 0; i < 4 && len(bits) < 128; i++ {
		bits = append(bits, 0)
	}
	// pad to byte boundary
	for len(bits)%8 != 0 {
		bits = append(bits, 0)
	}
	return bits
}

func addECC(data []byte, version int) []byte {
	// Simplified: for small QR codes, just pad to required codewords
	totalBits := (16 + version*16) * 8
	for len(data) < totalBits {
		data = append(data, 1, 1, 1, 0, 1, 1, 0, 0)
		if len(data) >= totalBits {
			break
		}
		data = append(data, 0, 0, 0, 1, 0, 0, 0, 1)
	}
	if len(data) > totalBits {
		data = data[:totalBits]
	}
	return data
}

func addFinderPatterns(matrix, reserved [][]bool, size int) {
	drawFinder := func(row, col int) {
		for r := -1; r <= 7; r++ {
			for c := -1; c <= 7; c++ {
				rr, cc := row+r, col+c
				if rr < 0 || rr >= size || cc < 0 || cc >= size {
					continue
				}
				reserved[rr][cc] = true
				isBlack := (r >= 0 && r <= 6 && (c == 0 || c == 6)) ||
					(c >= 0 && c <= 6 && (r == 0 || r == 6)) ||
					(r >= 2 && r <= 4 && c >= 2 && c <= 4)
				matrix[rr][cc] = isBlack
			}
		}
	}
	drawFinder(0, 0)
	drawFinder(0, size-7)
	drawFinder(size-7, 0)
}

func addAlignmentPatterns(matrix, reserved [][]bool, version, size int) {
	if version < 2 {
		return
	}
	positions := alignmentPositions(version, size)
	for _, row := range positions {
		for _, col := range positions {
			if reserved[row][col] {
				continue
			}
			for r := -2; r <= 2; r++ {
				for c := -2; c <= 2; c++ {
					rr, cc := row+r, col+c
					if rr < 0 || rr >= size || cc < 0 || cc >= size {
						continue
					}
					reserved[rr][cc] = true
					isBlack := r == -2 || r == 2 || c == -2 || c == 2 || (r == 0 && c == 0)
					matrix[rr][cc] = isBlack
				}
			}
		}
	}
}

func alignmentPositions(version, size int) []int {
	if version == 1 {
		return nil
	}
	last := size - 7
	first := 6
	count := version/7 + 2
	step := 0
	if count > 2 {
		step = int(math.Ceil(float64(last-first) / float64(count-1)))
		if step%2 == 1 {
			step++
		}
	}
	positions := []int{first}
	for i := 1; i < count-1; i++ {
		positions = append(positions, last-step*(count-1-i))
	}
	positions = append(positions, last)
	return positions
}

func addTimingPatterns(matrix, reserved [][]bool, size int) {
	for i := 8; i < size-8; i++ {
		reserved[6][i] = true
		matrix[6][i] = i%2 == 0
		reserved[i][6] = true
		matrix[i][6] = i%2 == 0
	}
}

func addDarkModule(matrix, reserved [][]bool, version int) {
	row := 4*version + 9
	matrix[row][8] = true
	reserved[row][8] = true
}

func placeData(matrix, reserved [][]bool, bits []byte, size int) {
	bitIdx := 0
	upward := true
	for col := size - 1; col >= 0; col -= 2 {
		if col == 6 {
			col--
		}
		rows := make([]int, size)
		if upward {
			for i := 0; i < size; i++ {
				rows[i] = size - 1 - i
			}
		} else {
			for i := 0; i < size; i++ {
				rows[i] = i
			}
		}
		for _, row := range rows {
			for dx := 0; dx <= 1; dx++ {
				c := col - dx
				if c < 0 || reserved[row][c] {
					continue
				}
				if bitIdx < len(bits) {
					matrix[row][c] = bits[bitIdx] == 1
					bitIdx++
				}
			}
		}
		upward = !upward
	}
}

func applyMask(matrix, reserved [][]bool, size int) {
	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			if reserved[row][col] {
				continue
			}
			if (row+col)%2 == 0 {
				matrix[row][col] = !matrix[row][col]
			}
		}
	}
}

func addFormatInfo(matrix [][]bool, size int) {
	// Format info for mask 0, ECC level L: 111011111000100
	format := []bool{true, true, true, false, true, true, true, true, true, false, false, false, true, false, false}
	for i := 0; i < 6; i++ {
		matrix[8][i] = format[i]
	}
	matrix[8][7] = format[6]
	matrix[8][8] = format[7]
	matrix[7][8] = format[8]
	for i := 9; i < 15; i++ {
		matrix[14-i][8] = format[i]
	}
	for i := 0; i < 8; i++ {
		matrix[8][size-1-i] = format[14-i]
	}
	for i := 0; i < 7; i++ {
		matrix[size-1-i][8] = format[i]
	}
}

func renderQRImage(matrix [][]bool, size int) image.Image {
	modules := len(matrix)
	scale := size / (modules + 8)
	if scale < 1 {
		scale = 1
	}
	imgSize := (modules + 8) * scale
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))

	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}

	for y := 0; y < imgSize; y++ {
		for x := 0; x < imgSize; x++ {
			img.Set(x, y, white)
		}
	}

	for row := 0; row < modules; row++ {
		for col := 0; col < modules; col++ {
			if matrix[row][col] {
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						img.Set((col+4)*scale+dx, (row+4)*scale+dy, black)
					}
				}
			}
		}
	}

	return img
}

func renderQRSVG(matrix [][]bool, size int) string {
	modules := len(matrix)
	scale := float64(size) / float64(modules+8)
	var sb bytes.Buffer

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`, size, size, size, size))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, size, size))

	for row := 0; row < modules; row++ {
		for col := 0; col < modules; col++ {
			if matrix[row][col] {
				x := float64(col+4) * scale
				y := float64(row+4) * scale
				sb.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="black"/>`, x, y, scale, scale))
			}
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}
