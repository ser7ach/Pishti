package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// clickableImage is a custom widget that acts like an image but can be tapped.
type clickableImage struct {
	widget.BaseWidget
	Resource fyne.Resource
	FillMode canvas.ImageFill
	minSize  fyne.Size
	onTapped func()
}

// newClickableImage creates a new instance of the custom widget.
func newClickableImage(onTapped func()) *clickableImage {
	img := &clickableImage{
		minSize:  fyne.NewSize(71, 96), // Default to card size.
		onTapped: onTapped,
	}
	img.ExtendBaseWidget(img) // This is crucial for it to be treated as a widget.
	return img
}

// CreateRenderer is a mandatory part of the Widget interface.
func (c *clickableImage) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(c.Resource)
	img.FillMode = c.FillMode
	return &clickableImageRenderer{
		image:  img,
		widget: c,
	}
}

// Tapped is called when the user taps the widget.
func (c *clickableImage) Tapped(_ *fyne.PointEvent) {
	if c.onTapped != nil {
		c.onTapped()
	}
}

// SetOnTapped allows changing the tap handler.
func (c *clickableImage) SetOnTapped(handler func()) {
	c.onTapped = handler
}

// --- Renderer for the custom widget ---

type clickableImageRenderer struct {
	image  *canvas.Image
	widget *clickableImage
}

func (r *clickableImageRenderer) Layout(size fyne.Size) {
	r.image.Resize(size)
}

func (r *clickableImageRenderer) MinSize() fyne.Size {
	return r.widget.minSize
}

func (r *clickableImageRenderer) Refresh() {
	// Update the resource.
	r.image.Resource = r.widget.Resource
	r.image.FillMode = r.widget.FillMode
	// Force clear the underlying image data when resource is nil.
	if r.widget.Resource == nil {
		r.image.Resource = nil
		r.image.Image = nil        // Clear the underlying image data.
		r.image.Translucency = 1.0 // Fully transparent.
	} else {
		r.image.Translucency = 0.0 // Fully opaque.
	}
	r.image.Refresh()
	canvas.Refresh(r.image) // Extra refresh to ensure it updates.
}

func (r *clickableImageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.image}
}

func (r *clickableImageRenderer) Destroy() {}
