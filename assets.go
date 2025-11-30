package main

import (
	"embed"
	"path"
	"strconv"

	"fyne.io/fyne/v2"
)

//go:embed assets
var embeddedAssets embed.FS

var (
	resourceCardBack   fyne.Resource
	resourceFrame      fyne.Resource
	resourceBackground fyne.Resource
	resourceCardCache  = make(map[string]fyne.Resource)
)

// loadResources initializes all global resource variables.
// This must be called after the Fyne app has been created to avoid deadlocks.
func loadResources() {
	resourceCardBack = mustLoadResource("assets/cards/back.png")
	resourceFrame = mustLoadResource("assets/ui/frame.png")
	resourceBackground = mustLoadResource("assets/ui/background.jpg")
	// Pre-load all card resources into the cache.
	for i := 1; i <= DeckSize; i++ {
		iconPath := strconv.Itoa(i)
		res := mustLoadResource("assets/cards/" + iconPath + ".png")
		resourceCardCache[iconPath] = res
	}
}

// getCardResource safely retrieves a card's resource from the cache.
func getCardResource(card *Card) fyne.Resource {
	iconPath := card.GetIconPath()
	res, ok := resourceCardCache[iconPath]
	if !ok {
		// This case should not happen with valid card data, but as a safeguard,
		// return the card back resource to avoid a crash with a nil resource.
		return resourceCardBack
	}
	return res
}

// mustLoadResource loads a resource from the embedded filesystem and panics on error.
func mustLoadResource(p string) fyne.Resource {
	data, err := embeddedAssets.ReadFile(p)
	if err != nil {
		// If an asset is not found, it's a critical error.
		// Panicking gives a clear stack trace and message.
		panic("failed to load embedded asset " + p + ": " + err.Error())
	}
	return fyne.NewStaticResource(path.Base(p), data)
}
