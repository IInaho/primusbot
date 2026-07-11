package view

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
)

func readImageDims(path string) []int {
	ext := strings.ToLower(path[strings.LastIndexByte(path, '.'):])
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil
	}
	return []int{cfg.Width, cfg.Height}
}
