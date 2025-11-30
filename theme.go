package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

// transparentTheme is a custom theme that makes specific widgets transparent.
type transparentTheme struct {
	fyne.Theme
}

// newTransparentTheme wraps the provided theme.
func newTransparentTheme(t fyne.Theme) fyne.Theme {
	return &transparentTheme{Theme: t}
}

// Color overrides the default color for specific widget states.
func (t *transparentTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Make button and input backgrounds transparent.
	if name == theme.ColorNameButton || name == theme.ColorNameDisabledButton || name == theme.ColorNameInputBackground {
		return color.Transparent
	}
	// Make the dropdown menu background transparent.
	if name == theme.ColorNameMenuBackground {
		return color.Transparent
	}
	// Disabled text should be slightly grey to show it's disabled.
	if name == theme.ColorNameDisabled {
		return color.NRGBA{R: 150, G: 150, B: 150, A: 255} // Grey text for disabled state.
	}
	// Dialog and overlay backgrounds should be opaque and visible.
	if name == theme.ColorNameOverlayBackground {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 180} // Semi-transparent dark overlay.
	}
	// Dialog background should be solid and visible.
	if name == theme.ColorNameBackground {
		return color.NRGBA{R: 40, G: 40, B: 40, A: 255} // Solid dark grey.
	}
	// Hover color for menu items and selected items.
	if name == theme.ColorNameHover || name == theme.ColorNamePressed {
		return color.NRGBA{R: 60, G: 100, B: 180, A: 200} // Blue hover/pressed effect.
	}
	// Ensure all normal text is white and visible.
	if name == theme.ColorNameForeground {
		return color.White
	}
	// For all other colors, use the default from the base theme.
	return t.Theme.Color(name, variant)
}

// Variant returns the theme variant. By declaring the theme as dark, Fyne will
// automatically use light-colored text and icons, which is what is needed for the
// widgets placed on a dark background image.
func (t *transparentTheme) Variant() fyne.ThemeVariant {
	return theme.VariantDark
}
