package tui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
)

// renderAlbumCover renders an image as terminal cells. Each upper-half block
// combines two sampled pixels, preserving a roughly square image despite a
// terminal cell being taller than it is wide.
func renderAlbumCover(path string, width, height int) (string, error) {
	if width <= 0 || height <= 0 {
		return "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", err
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return "", fmt.Errorf("cover has empty bounds")
	}

	lines := make([]string, height)
	for y := range height {
		var line strings.Builder
		for x := range width {
			top := sampleCoverColor(img, x, y*2, width, height*2)
			bottom := sampleCoverColor(img, x, y*2+1, width, height*2)
			fmt.Fprintf(&line, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				top.R, top.G, top.B, bottom.R, bottom.G, bottom.B)
		}
		line.WriteString("\x1b[0m")
		lines[y] = line.String()
	}
	return strings.Join(lines, "\n"), nil
}

func sampleCoverColor(img image.Image, x, y, width, height int) color.NRGBA {
	bounds := img.Bounds()
	sourceX := bounds.Min.X + x*bounds.Dx()/width
	sourceY := bounds.Min.Y + y*bounds.Dy()/height
	return color.NRGBAModel.Convert(img.At(sourceX, sourceY)).(color.NRGBA)
}
