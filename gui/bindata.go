package gui

import (
	"github.com/lebauce/vlaunch/statics"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
)

func IconFromBindata(name string) (*gui.QIcon, error) {
	img := gui.NewQImage()
	data, err := statics.Asset("statics/" + name)
	if err != nil {
		return nil, err
	}
	img.LoadFromData(string(data), len(data), "PNG")
	pixmap := gui.NewQPixmap()
	pixmap.ConvertFromImage(img, core.Qt__AutoColor)
	return gui.NewQIcon2(pixmap), nil
}
