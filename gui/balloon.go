package gui

import (
	"fmt"
	"log"
	"strconv"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

type Balloon struct {
	widget        *widgets.QWidget
	layout        *widgets.QHBoxLayout
	contentLayout *widgets.QVBoxLayout
	progressBar   *widgets.QProgressBar
}

func (b *Balloon) Show() {
	b.widget.Show()
}

func (b *Balloon) OnGuestPropertyChanged(name, value string, timestamp int64, flags string) {
	log.Printf("OnGuestPropertyChanged %s => %s\n", name, value)
	switch name {
	case "/UFO/Boot/Progress":
		if b.progressBar != nil {
			percentage, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return
			}
			log.Printf("Updating progress bar: %d", int(percentage*100))
			b.progressBar.SetValue(int(percentage * 100))
		}
	case "/UFO/State":
		if value == "LOGGED_IN" {
			log.Println("Closing balloon")
			b.widget.Hide()
			b.widget.Close()
		}
	}
}

func NewBalloon(app *widgets.QApplication, title string, msg string, progress bool) (*Balloon, error) {
	flags := core.Qt__WindowStaysOnTopHint | core.Qt__Popup // core.Qt__X11BypassWindowManagerHint
	widget := widgets.NewQWidget(nil, flags)

	layout := widgets.NewQHBoxLayout()
	contentLayout := widgets.NewQVBoxLayout()
	layout.AddLayout(contentLayout, 0)
	widget.SetLayout(layout)

	titleLayout := widgets.NewQHBoxLayout()
	text := fmt.Sprintf("<b><font color=%s>%s</font></b>", "red", title)
	titleLabel := widgets.NewQLabel2(text, nil, 0)
	titleLabel.SetMinimumWidth(250)
	titleLabel.SetSizePolicy2(widgets.QSizePolicy__Minimum, widgets.QSizePolicy__Minimum)
	titleLayout.QLayout.AddWidget(titleLabel)

	closeIcon, err := IconFromBindata("close.png")
	if err != nil {
		return nil, err
	}

	closeButton := widgets.NewQPushButton3(closeIcon, "", widget)
	closeButton.SetFlat(true)
	closeButton.ConnectClicked(func(checked bool) {
		widget.Close()
		// app.QuitDefault()
	})

	titleLayout.AddWidget(closeButton, 0, core.Qt__AlignRight)
	contentLayout.AddLayout(titleLayout, 0)

	if msg != "" {
		text = fmt.Sprintf("<font color=%s>%s</font>", "black", msg)
		textLabel := widgets.NewQLabel2(msg, nil, 0)
		textLabel.SetSizePolicy2(widgets.QSizePolicy__Minimum, widgets.QSizePolicy__Minimum)
		contentLayout.QLayout.AddWidget(textLabel)
	}

	widget.ConnectPaintEvent(func(vqp *gui.QPaintEvent) {
		path := gui.NewQPainterPath()
		rect := core.NewQRect4(0, 0, widget.Width(), widget.Height())
		rectf := core.NewQRectF5(rect)
		path.AddRoundedRect(rectf, 15.0, 15.0, core.Qt__AbsoluteSize)

		painter := gui.NewQPainter2(widget)
		painter.SetRenderHint(gui.QPainter__HighQualityAntialiasing, true)
		white := gui.QColor_FromRgb2(0, 0, 0, 255)
		pen := gui.NewQPen3(white)
		pen.SetWidth(3)
		painter.SetPen(pen)

		painter.SetClipPath(path, core.Qt__IntersectClip)

		gradient := gui.NewQLinearGradient3(0, 0, 0, 100)
		gradient.SetColorAt(0.0, gui.QColor_FromRgb2(255, 255, 239, 255))
		gradient.SetColorAt(1.0, gui.QColor_FromRgb2(255, 255, 255, 255))
		brush := gui.NewQBrush10(gradient)
		painter.SetBrush(brush)
		painter.DrawPath(path)
		widget.SetMask2(painter.ClipRegion())
	})

	widget.SetAutoFillBackground(true)
	// widget.SetWindowOpacity(0.0)

	var progressBar *widgets.QProgressBar
	if progress {
		progressBar = widgets.NewQProgressBar(nil)
		progressBar.SetRange(0, 100)
		progressBar.SetValue(0)
		contentLayout.QLayout.AddWidget(progressBar)
		// progressBar.Hide()
	}

	widget.Show()

	desktop := widgets.QApplication_Desktop()
	screenRect := desktop.ScreenGeometry(desktop.PrimaryScreen())

	pos := core.NewQPoint2(screenRect.Width()-widget.Width()-5, screenRect.Height()-widget.Height()-5)
	widget.Move(pos)

	/*
		geometry := widget.Geometry()

		self.on_top = self.geometry().y() < screenRect.height() / 2
		if self.on_top:
				balloon_y = self.geometry().bottom() + 10
		else:
				balloon_y = self.geometry().top()
	*/
	/*
	  if vlayout:
	      self.vlayout = vlayout['type'](*vlayout['args'])
	      self.vlayout.set_parent(self)
	      self.contents_layout.addLayout(self.vlayout)
	*/

	return &Balloon{
		widget:        widget,
		layout:        layout,
		contentLayout: contentLayout,
		progressBar:   progressBar,
	}, nil
}
