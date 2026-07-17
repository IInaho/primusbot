package telegram

import (
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func terminalQR(content string) (string, error) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", err
	}
	bitmap := qr.Bitmap()
	if len(bitmap) == 0 {
		return "", nil
	}
	var b strings.Builder
	width := len(bitmap[0])
	writeRow := func(y int) {
		for x := range width {
			top := moduleAt(bitmap, x, y)
			bottom := moduleAt(bitmap, x, y+1)
			switch {
			case top && bottom:
				b.WriteString("\x1b[40m ")
			case top:
				b.WriteString("\x1b[30;47m▀")
			case bottom:
				b.WriteString("\x1b[30;47m▄")
			default:
				b.WriteString("\x1b[47m ")
			}
		}
		b.WriteString("\x1b[0m")
	}
	for y := 0; y < len(bitmap); y += 2 {
		if y > 0 {
			b.WriteByte('\n')
		}
		writeRow(y)
	}
	return b.String(), nil
}

func moduleAt(bitmap [][]bool, x, y int) bool {
	if y < 0 || y >= len(bitmap) || x < 0 || x >= len(bitmap[y]) {
		return false
	}
	return bitmap[y][x]
}
