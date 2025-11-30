package main

import "fyne.io/fyne/v2"

// minSizeLayout is a custom layout that enforces a minimum size on its content.
// This is useful for creating fixed-size spacers or ensuring widgets don't shrink
// below a certain size.
type minSizeLayout struct {
	min fyne.Size
}

// Layout resizes the content to fill the container.
func (m *minSizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
	}
}

// MinSize returns the enforced minimum size.
func (m *minSizeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return m.min
}
